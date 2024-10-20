package syncer

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/repo"
	"github.com/moresec-io/conduit/pkg/network"
	"github.com/moresec-io/conduit/pkg/proto"
	"github.com/singchia/geminio"
	gclient "github.com/singchia/geminio/client"
)

const (
	SyncModeUp = 1 << iota
	SyncModeDown
)

type Syncer interface {
	ReportServer(request *proto.ReportServerRequest) (*proto.ReportServerResponse, error)
}

func NewSyncer(conf *config.Config, repo repo.Repo, syncMode int) (Syncer, error) {
	return newsyncer(conf, repo, syncMode)
}

type syncer struct {
	machineid string
	end       geminio.End

	repo repo.Repo

	conf     *config.Config
	mtx      sync.RWMutex
	cache    []proto.Conduit // key: machineid, value: ipnets
	syncMode int
}

func newsyncer(conf *config.Config, repo repo.Repo, syncMode int) (*syncer, error) {
	syncer := &syncer{
		cache: []proto.Conduit{},
		repo:  repo,
	}

	// connect to manager
	dialer := func() (net.Conn, error) {
		return network.DialRandom(&config.Conf.Manager.Dial)
	}
	opt := gclient.NewEndOptions()
	opt.SetMeta([]byte(conf.MachineID))
	end, err := gclient.NewEndWithDialer(dialer, opt)
	if err != nil {
		return nil, err
	}
	syncer.end = end

	// only downlink cares about other conduits online/offline
	if syncMode&SyncModeDown != 0 {
		err = end.Register(context.TODO(), proto.RPCOnlineConduit, syncer.onlineConduit)
		if err != nil {
			return nil, err
		}
		err = end.Register(context.TODO(), proto.RPCOfflineConduit, syncer.offlineConduit)
		if err != nil {
			return nil, err
		}
	}

	go syncer.sync(syncMode)
	return syncer, nil
}

func (syncer *syncer) ReportServer(request *proto.ReportServerRequest) (*proto.ReportServerResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req := syncer.end.NewRequest(data)
	rsp, err := syncer.end.Call(context.TODO(), proto.RPCReportServer, req)
	if err != nil {
		return nil, err
	}
	if rsp.Error() != nil {
		return nil, err
	}
	data = rsp.Data()
	response := &proto.ReportServerResponse{}
	err = json.Unmarshal(data, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (syncer *syncer) ReportClient(request *proto.ReportClientRequest) (*proto.ReportClientResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req := syncer.end.NewRequest(data)
	rsp, err := syncer.end.Call(context.TODO(), proto.RPCReportClient, req)
	if err != nil {
		return nil, err
	}
	if rsp.Error() != nil {
		return nil, err
	}
	data = rsp.Data()
	response := &proto.ReportClientResponse{}
	err = json.Unmarshal(data, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// client only
func (syncer *syncer) onlineConduit(_ context.Context, req geminio.Request, rsp geminio.Response) {
	data := req.Data()
	request := &proto.OnlineConduitRequest{}
	err := json.Unmarshal(data, request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	syncer.mtx.Lock()
	defer syncer.mtx.Unlock()

	found := false
	for _, oldone := range syncer.cache {
		if oldone.MachineID == request.MachineID {
			// typically we won't be here
			found = true
			removes, adds := compareNets(oldone.IPNets, request.IPNets)
			for _, remove := range removes {
				err = syncer.repo.DelIPSetIP(remove.IP)
				if err != nil {
					log.Errorf("syncer online conduit, del ipset err: %s", err)
					continue
				}
			}
			for _, add := range adds {
				err = syncer.repo.AddIPSetIP(add.IP)
				if err != nil {
					log.Errorf("syncer online conduit, add ipset err: %s", err)
					continue
				}
			}
			break
		}
	}
	if !found {
		syncer.cache = append(syncer.cache, proto.Conduit{
			MachineID: request.MachineID,
			IPNets:    request.IPNets,
		})
		for _, newone := range request.IPNets {
			err = syncer.repo.AddIPSetIP(newone.IP)
			if err != nil {
				log.Errorf("syncer online conduit, add ipset err: %s", err)
				continue
			}
		}
	}
	return
}

// client only
func (syncer *syncer) offlineConduit(_ context.Context, req geminio.Request, rsp geminio.Response) {
	data := req.Data()
	request := &proto.OfflineConduitRequest{}
	err := json.Unmarshal(data, request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	syncer.mtx.Lock()
	defer syncer.mtx.Unlock()

	for i, oldone := range syncer.cache {
		if oldone.MachineID == request.MachineID {
			for _, remove := range oldone.IPNets {
				err = syncer.repo.DelIPSetIP(remove.IP)
				if err != nil {
					log.Errorf("syncer offline conduit, del ipset err: %s", err)
					continue
				}
			}
			syncer.cache = append(syncer.cache[:i], syncer.cache[i+1:]...)
			break
		}
	}
}

func (syncer *syncer) sync(syncMode int) {
	ticker := time.NewTicker(10 * time.Second)
	for {
		<-ticker.C
		if syncMode&SyncModeUp != 0 {
			err := syncer.report()
			if err != nil {
				log.Errorf("syncer sync, report agent err: %s", err)
				continue
			}
		}
		if syncMode&SyncModeDown != 0 {
			err := syncer.pullCluster()
			if err != nil {
				log.Errorf("syncer sync, pull cluster err: %s", err)
				continue
			}
		}
	}
}

func (syncer *syncer) report() error {
	// conduit network
	// currently we ignore bridges, and all local networks should be accessable by conduit
	networks, err := network.ListNetworks()
	if err != nil {
		return err
	}
	request := &proto.ReportNetworksRequest{
		MachineID: syncer.machineid,
		IPNets:    networks,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := syncer.end.NewRequest(data)
	rsp, err := syncer.end.Call(context.TODO(), proto.RPCReportNetworks, req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return err
	}
	return nil
}

func (syncer *syncer) pullCluster() error {
	request := &proto.PullClusterRequest{
		MachineID: syncer.machineid,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := syncer.end.NewRequest(data)
	rsp, err := syncer.end.Call(context.TODO(), proto.RPCPullCluster, req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return err
	}
	data = rsp.Data()
	response := &proto.PullClusterResponse{}
	err = json.Unmarshal(data, response)
	if err != nil {
		return err
	}
	syncer.mtx.Lock()
	defer syncer.mtx.Unlock()

	removes, adds := compareConduits(syncer.cache, response.Conduits)
	syncer.cache = response.Conduits
	for _, remove := range removes {
		err = syncer.repo.DelIPSetIP(remove.IP)
		if err != nil {
			log.Errorf("syncer pull cluster, del ipset err: %s", err)
			continue
		}
	}
	for _, add := range adds {
		err = syncer.repo.AddIPSetIP(add.IP)
		if err != nil {
			log.Errorf("syncer pull cluster, add ipset err: %s", err)
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
