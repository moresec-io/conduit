package reporter

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/client"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/proto"
	"github.com/moresec-io/conduit/pkg/utils"
	"github.com/singchia/geminio"
	gclient "github.com/singchia/geminio/client"
)

type Reporter struct {
	machineid string
	end       geminio.End

	client *client.Client

	mtx   sync.RWMutex
	cache []proto.Conduit // key: machineid, value: ipnets
}

func NewReporter(conf *config.Config, client *client.Client) (*Reporter, error) {
	reporter := &Reporter{
		cache: []proto.Conduit{},
	}

	id, err := machineid.ID()
	if err != nil {
		return nil, err
	}
	reporter.machineid = id

	dialer := func() (net.Conn, error) {
		return utils.DialRandom(&config.Conf.Conduit.Dial)
	}
	opt := gclient.NewEndOptions()
	opt.SetMeta([]byte(id))
	end, err := gclient.NewEndWithDialer(dialer, opt)
	if err != nil {
		return nil, err
	}
	reporter.end = end

	err = end.Register(context.TODO(), proto.RPCOnlineConduit, reporter.onlineConduit)
	if err != nil {
		return nil, err
	}
	err = end.Register(context.TODO(), proto.RPCOfflineConduit, reporter.offlineConduit)
	if err != nil {
		return nil, err
	}

	go reporter.sync()
	return reporter, nil
}

func (reporter *Reporter) onlineConduit(_ context.Context, req geminio.Request, rsp geminio.Response) {
	data := req.Data()
	request := &proto.OnlineConduitRequest{}
	err := json.Unmarshal(data, request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	reporter.mtx.Lock()
	defer reporter.mtx.Unlock()

	found := false
	for _, oldone := range reporter.cache {
		if oldone.MachineID == request.MachineID {
			// typically we won't be here
			found = true
			removes, adds := compareNets(oldone.IPNets, request.IPNets)
			for _, remove := range removes {
				err = reporter.client.DelIPSetIP(remove.IP)
				if err != nil {
					log.Errorf("reporter online conduit, del ipset err: %s", err)
					continue
				}
			}
			for _, add := range adds {
				err = reporter.client.AddIPSetIP(add.IP)
				if err != nil {
					log.Errorf("reporter online conduit, add ipset err: %s", err)
					continue
				}
			}
			break
		}
	}
	if !found {
		reporter.cache = append(reporter.cache, proto.Conduit{
			MachineID: request.MachineID,
			IPNets:    request.IPNets,
		})
		for _, newone := range request.IPNets {
			err = reporter.client.AddIPSetIP(newone.IP)
			if err != nil {
				log.Errorf("reporter online conduit, add ipset err: %s", err)
				continue
			}
		}
	}
	return
}

func (reporter *Reporter) offlineConduit(_ context.Context, req geminio.Request, rsp geminio.Response) {
	data := req.Data()
	request := &proto.OfflineConduitRequest{}
	err := json.Unmarshal(data, request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	reporter.mtx.Lock()
	defer reporter.mtx.Unlock()
}

func (reporter *Reporter) sync() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		<-ticker.C
		err := reporter.reportConduit()
		if err != nil {
			log.Errorf("reporter sync, report agent err: %s", err)
			continue
		}
		err = reporter.pullCluster()
		if err != nil {
			log.Errorf("reporter sync, pull cluster err: %s", err)
			continue
		}
	}
}

func (reporter *Reporter) reportConduit() error {
	// conduit network
	networks, err := utils.ListNetworks()
	if err != nil {
		return err
	}
	request := &proto.ReportConduitRequest{
		MachineID: reporter.machineid,
		IPNets:    networks,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := reporter.end.NewRequest(data)
	rsp, err := reporter.end.Call(context.TODO(), proto.RPCReportConduit, req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return err
	}
	return nil
}

func (reporter *Reporter) pullCluster() error {
	request := &proto.PullClusterRequest{
		MachineID: reporter.machineid,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := reporter.end.NewRequest(data)
	rsp, err := reporter.end.Call(context.TODO(), proto.RPCPullCluster, req)
	if err != nil {
		return err
	}
	data = rsp.Data()
	response := &proto.PullClusterResponse{}
	err = json.Unmarshal(data, response)
	if err != nil {
		return err
	}
	reporter.mtx.Lock()
	defer reporter.mtx.Unlock()

	removes, adds := compareConduits(reporter.cache, response.Conduits)
	reporter.cache = response.Conduits
	for _, remove := range removes {
		err = reporter.client.DelIPSetIP(remove.IP)
		if err != nil {
			log.Errorf("reporter pull cluster, del ipset err: %s", err)
			continue
		}
	}
	for _, add := range adds {
		err = reporter.client.AddIPSetIP(add.IP)
		if err != nil {
			log.Errorf("reporter pull cluster, add ipset err: %s", err)
			continue
		}
	}
	return nil
}

func compareConduits(old, new []proto.Conduit) ([]net.IPNet, []net.IPNet) {
	keeps := []string{}
	removes := []net.IPNet{}
	adds := []net.IPNet{}

	for _, oldone := range old {
		found := false
		for _, newone := range new {
			if oldone.MachineID == newone.MachineID {
				rs, as := compareNets(oldone.IPNets, newone.IPNets)
				removes = append(removes, rs...)
				adds = append(adds, as...)
				found = true
				break
			}
		}
		if !found {
			for _, elem := range oldone.IPNets {
				removes = append(removes, elem)
			}
		}
	}

	for _, newone := range new {
		found := false
		for _, keep := range keeps {
			if newone.MachineID == keep {
				found = true
				break
			}
		}
		if !found {
			for _, elem := range newone.IPNets {
				adds = append(adds, elem)
			}
		}
	}
	return removes, adds
}

func compareNets(old, new []net.IPNet) ([]net.IPNet, []net.IPNet) {
	keeps := []net.IPNet{}
	removes := []net.IPNet{}
	adds := []net.IPNet{}

	for _, oldnet := range old {
		found := false
		for _, newnet := range new {
			if oldnet.String() == newnet.String() {
				keeps = append(keeps, oldnet)
				found = true
				break
			}
		}
		if !found {
			removes = append(removes, oldnet)
		}
	}

	for _, newnet := range new {
		found := false
		for _, keep := range keeps {
			if newnet.String() == keep.String() {
				found = true
				break
			}
		}
		if !found {
			adds = append(adds, newnet)
		}
	}
	return removes, adds
}
