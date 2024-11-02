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

	mtx   sync.RWMutex
	cache []proto.Conduit // key: machineid, value: ipnets
	// client use cert
	clientTLS *proto.TLS
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
	syncer.clientTLS = response.TLS
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

	for _, oldone := range syncer.cache {
		if oldone.MachineID == request.Conduit.MachineID {
			ok := compareConduit(&oldone, request.Conduit)
			if ok {
				return
			}
			// update
			for _, ip := range oldone.IPs {
				syncer.repo.DelIPPolicy(ip.String())
			}
			for _, ip := range request.Conduit.IPs {
				syncer.repo.AddIPPolicy(ip.String(), &repo.Policy{
					PeerDialConfig: &network.DialConfig{
						TLS: &network.TLS{
							Enable: true,
							MTLS:   true,
							// TODO
						},
					},
				})
			}
			return
		}
	}
	// add new conduit
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
			for _, remove := range oldone.IPs {
				err = syncer.repo.DelIPSetIP(remove)
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
	// currently we ignore networks
	networks, err := network.ListIPs()
	if err != nil {
		return err
	}
	request := &proto.ReportNetworksRequest{
		MachineID: syncer.machineid,
		IPs:       networks,
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

	removes, adds := compareConduits(syncer.cache, response.Cluster)
	syncer.cache = response.Cluster
	// updates
	for _, remove := range removes {
		for _, ip := range remove.IPs {
			err = syncer.repo.DelIPSetIP(ip)
			if err != nil {
				log.Errorf("syncer pull cluster, del ipset err: %s", err)
				continue
			}
		}
	}
	for _, add := range adds {
		for _, ip := range add.IPs {
			err = syncer.repo.AddIPSetIP(ip)
			if err != nil {
				log.Errorf("syncer pull cluster, add ipset err: %s", err)
				continue
			}
		}
	}
	return nil
}

// TODO change the logic
func compareConduits(old, new []proto.Conduit) ([]proto.Conduit, []proto.Conduit) {
	keeps := map[string]struct{}{}
	removes := []proto.Conduit{}
	adds := []proto.Conduit{}

	for _, oldone := range old {
		found := false
		for _, newone := range new {
			if oldone.MachineID == newone.MachineID {
				if !compareConduit(&oldone, &newone) {
					removes = append(removes, oldone)
				} else {
					// keeps store old ones
					keeps[oldone.MachineID] = struct{}{}
				}
				found = true
				break
			}
		}
		if !found {
			removes = append(removes, oldone)
		}
	}

	for _, newone := range new {
		_, found := keeps[newone.MachineID]
		if !found {
			adds = append(adds, newone)
		}
	}
	return removes, adds
}

func compareConduit(old, new *proto.Conduit) bool {
	if old.Addr != new.Addr ||
		old.Network != new.Network ||
		!compareNets(old.IPs, new.IPs) {
		return false
	}
	return true
}

func compareNets(old, new []net.IP) bool {
	if len(old) != len(new) {
		return false
	}
	for _, oldip := range old {
		found := false
		for _, newip := range new {
			if oldip.Equal(newip) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
