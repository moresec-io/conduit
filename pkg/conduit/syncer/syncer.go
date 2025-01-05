package syncer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/repo"
	"github.com/moresec-io/conduit/pkg/network"
	"github.com/moresec-io/conduit/pkg/proto"
	"github.com/moresec-io/conduit/pkg/utils"
	"github.com/singchia/geminio"
	gclient "github.com/singchia/geminio/client"
)

const (
	SyncModeUp = 1 << iota
	SyncModeDown
)

type Syncer interface {
	ReportServer(request *proto.ReportServerRequest) (*proto.ReportServerResponse, error)
	ReportClient(request *proto.ReportClientRequest) (*proto.ReportClientResponse, error)
}

func NewSyncer(conf *config.Config, repo repo.Repo, syncMode int) (Syncer, error) {
	return newsyncer(conf, repo, syncMode)
}

type syncer struct {
	machineid string
	end       geminio.End

	repo repo.Repo

	mtx   sync.RWMutex
	cache []proto.Conduit // key: machineid, value: conduits
	// client certs
	caPool     *x509.CertPool
	clientCert *tls.Certificate
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
		log.Errorf("new syncer, geminio dial manager err: %s, sync mode: %d", err, syncMode)
		return nil, err
	}
	syncer.end = end

	// only downlink cares about other conduits online/offline
	if syncMode&SyncModeDown != 0 {
		err = end.Register(context.TODO(), proto.RPCSyncConduitOnline, syncer.syncConduitOnline)
		if err != nil {
			log.Errorf("new syncer, register sync conduit online err: %s", err)
			return nil, err
		}
		err = end.Register(context.TODO(), proto.RPCSyncConduitOffline, syncer.syncConduitOffline)
		if err != nil {
			log.Errorf("new syncer, register sync conduit offline err: %s", err)
			return nil, err
		}
		err = end.Register(context.TODO(), proto.RPCSyncConduitNetworksChanged, syncer.syncConduitNetworksChanged)
		if err != nil {
			log.Errorf("new syncer, register sync conduit networks changed err: %s", err)
			return nil, err
		}
	}

	go syncer.sync(syncMode)
	return syncer, nil
}

func (syncer *syncer) ReportServer(request *proto.ReportServerRequest) (*proto.ReportServerResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		log.Errorf("syncer report server, json marshal err: %s", err)
		return nil, err
	}
	req := syncer.end.NewRequest(data)
	rsp, err := syncer.end.Call(context.TODO(), proto.RPCReportServer, req)
	if err != nil {
		log.Errorf("syncer report server, call rpc err: %s", err)
		return nil, err
	}
	if rsp.Error() != nil {
		log.Errorf("syncer report server, response err: %s", err)
		return nil, err
	}
	data = rsp.Data()
	response := &proto.ReportServerResponse{}
	err = json.Unmarshal(data, response)
	if err != nil {
		log.Errorf("syncer report server, json unmarshal response err: %s", err)
		return nil, err
	}
	return response, nil
}

func (syncer *syncer) ReportClient(request *proto.ReportClientRequest) (*proto.ReportClientResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		log.Errorf("syncer report client, json marshal err: %s", err)
		return nil, err
	}
	req := syncer.end.NewRequest(data)
	rsp, err := syncer.end.Call(context.TODO(), proto.RPCReportClient, req)
	if err != nil {
		log.Errorf("syncer report client, call rpc err: %s", err)
		return nil, err
	}
	if rsp.Error() != nil {
		log.Errorf("syncer report client, response err: %s", err)
		return nil, err
	}
	// manager returned ca and cert for client
	data = rsp.Data()
	response := &proto.ReportClientResponse{}
	err = json.Unmarshal(data, response)
	if err != nil {
		log.Errorf("syncer report client, json unmarshal err: %s", err)
		return nil, err
	}
	// der format to x509 certiface
	x509cert, err := x509.ParseCertificate(response.TLS.CA)
	if err != nil {
		log.Errorf("syncer report client, x509 parse certificate err: %s", err)
		return nil, err
	}
	caPool := x509.NewCertPool()
	caPool.AddCert(x509cert)
	// keep ca
	syncer.caPool = caPool
	// keep client certificate, der format to rsa private key
	privateKey, err := x509.ParsePKCS1PrivateKey(response.TLS.Key)
	if err != nil {
		log.Errorf("syncer report client, x509 parse pkcs #1 private key err: %s", err)
		return nil, err
	}
	clientCert := &tls.Certificate{
		Certificate: [][]byte{response.TLS.Cert},
		PrivateKey:  privateKey,
	}
	syncer.clientCert = clientCert
	return response, nil
}

// client only
func (syncer *syncer) syncConduitOnline(_ context.Context, req geminio.Request, rsp geminio.Response) {
	data := req.Data()
	request := &proto.SyncConduitOnlineRequest{}
	err := json.Unmarshal(data, request)
	if err != nil {
		log.Errorf("syncer conduit online, json unmarshal err: %s", err)
		rsp.SetError(err)
		return
	}
	conduit := request.Conduit
	syncer.mtx.Lock()
	defer syncer.mtx.Unlock()

	for _, elem := range syncer.cache {
		if elem.MachineID == conduit.MachineID {
			// found and unchanged
			ok := compareConduit(&elem, conduit)
			if ok {
				log.Infof("syncer conduit online, conduit: %s unchanged", conduit.MachineID)
				return
			}
			// del ips
			for _, ip := range elem.IPs {
				// del policy
				syncer.repo.DelIPPolicy(ip.String())
				// del ipset
				err := syncer.repo.DelIPSetIP(ip)
				if err != nil {
					log.Errorf("syncer online conduit, del ipset ip err: %s", err)
					continue
				}
			}
			log.Infof("syncer conduit online, conduit: %s deleted ips: %v", conduit.MachineID, elem.IPs)
			// add new ips
			elem.IPs = conduit.IPs
			for _, ip := range elem.IPs {
				// add policy
				syncer.repo.AddIPPolicy(ip.String(), &repo.Policy{
					PeerDialConfig: &network.DialConfig{
						TLS: &network.TLS{
							Enable:             true,
							MTLS:               true,
							CAPool:             syncer.caPool,
							Certs:              []tls.Certificate{*syncer.clientCert},
							InsecureSkipVerify: false,
						},
					},
				})
				// add ipset
				err := syncer.repo.AddIPSetIP(ip)
				if err != nil {
					log.Errorf("syncer online conduit, add ipset ip err: %s", err)
					// TODO handle the inconsistency
					continue
				}
			}
			log.Infof("syncer conduit online, conduit: %s add ips: %v success", conduit.MachineID, elem.IPs)
			return
		}
	}
	// add new conduit
	for _, ip := range conduit.IPs {
		// add policy
		syncer.repo.AddIPPolicy(ip.String(), &repo.Policy{
			PeerDialConfig: &network.DialConfig{
				TLS: &network.TLS{
					Enable:             true,
					MTLS:               true,
					CAPool:             syncer.caPool,
					Certs:              []tls.Certificate{*syncer.clientCert},
					InsecureSkipVerify: false,
				},
			},
		})
		// add ipset
		err := syncer.repo.AddIPSetIP(ip)
		if err != nil {
			log.Errorf("syncer online conduit, add ipset ip err: %s", err)
			// TODO handle the inconsistency
			continue
		}
	}
	syncer.cache = append(syncer.cache, proto.Conduit{
		MachineID: conduit.MachineID,
		Network:   conduit.Network,
		Addr:      conduit.Addr,
		IPs:       conduit.IPs,
	})
	log.Infof("syncer conduit online, add new conduit: %s, addr: %s, ips: %v success", conduit.MachineID, conduit.Addr, conduit.IPs)
}

// client only
func (syncer *syncer) syncConduitOffline(_ context.Context, req geminio.Request, rsp geminio.Response) {
	data := req.Data()
	request := &proto.SyncConduitOfflineRequest{}
	err := json.Unmarshal(data, request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	syncer.mtx.Lock()
	defer syncer.mtx.Unlock()

	for i, oldone := range syncer.cache {
		if oldone.MachineID == request.MachineID {
			for _, ip := range oldone.IPs {
				// del policy
				syncer.repo.DelIPPolicy(ip.String())
				// del ipset
				err = syncer.repo.DelIPSetIP(ip)
				if err != nil {
					log.Errorf("syncer offline conduit, del ipset err: %s", err)
					continue
				}
			}
			syncer.cache = append(syncer.cache[:i], syncer.cache[i+1:]...)
			log.Infof("syncer conduit offline, del conduit: %s success", request.MachineID)
			return
		}
	}
	log.Warnf("syncer conduit offline, conduit: %s to delete not found", request.MachineID)
}

// client only
func (syncer *syncer) syncConduitNetworksChanged(_ context.Context, req geminio.Request, rsp geminio.Response) {
	data := req.Data()
	request := &proto.SyncConduitNetworksChangedRequest{}
	err := json.Unmarshal(data, request)
	if err != nil {
		rsp.SetError(err)
		return
	}

	syncer.mtx.Lock()
	defer syncer.mtx.Unlock()

	for _, elem := range syncer.cache {
		if elem.MachineID == request.MachineID {
			// del old ips
			for _, ip := range elem.IPs {
				// del policy
				syncer.repo.DelIPPolicy(ip.String())
				// del ipset
				err = syncer.repo.DelIPSetIP(ip)
				if err != nil {
					log.Errorf("syncer conduit network changed, del ipset err: %s", err)
					continue
				}
			}
			// add new ips
			elem.IPs = request.IPs
			for _, ip := range elem.IPs {
				// add policy
				syncer.repo.AddIPPolicy(ip.String(), &repo.Policy{
					PeerDialConfig: &network.DialConfig{
						TLS: &network.TLS{
							Enable:             true,
							MTLS:               true,
							CAPool:             syncer.caPool,
							Certs:              []tls.Certificate{*syncer.clientCert},
							InsecureSkipVerify: false,
						},
					},
				})
				// add ipset
				err := syncer.repo.AddIPSetIP(ip)
				if err != nil {
					log.Errorf("syncer conduit network changed, add ipset ip err: %s", err)
					continue
				}
			}
			break
		}
	}
}

func (syncer *syncer) sync(syncMode int) {
	ticker := time.NewTicker(60 * time.Second)
	for {
		<-ticker.C
		if syncMode&SyncModeUp != 0 {
			err := syncer.reportNetworks()
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

// server report local networks to manager
func (syncer *syncer) reportNetworks() error {
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

// client pull cluster
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
			// del policy
			syncer.repo.DelIPPolicy(ip.String())
			// del ipset
			err = syncer.repo.DelIPSetIP(ip)
			if err != nil {
				log.Errorf("syncer pull cluster, del ipset err: %s", err)
				continue
			}
		}
	}
	for _, add := range adds {
		for _, ip := range add.IPs {
			syncer.repo.AddIPPolicy(ip.String(), &repo.Policy{
				PeerDialConfig: &network.DialConfig{
					TLS: &network.TLS{
						Enable:             true,
						MTLS:               true,
						CAPool:             syncer.caPool,
						Certs:              []tls.Certificate{*syncer.clientCert},
						InsecureSkipVerify: false,
					},
				},
			})
			// add ipset
			err = syncer.repo.AddIPSetIP(ip)
			if err != nil {
				log.Errorf("syncer pull cluster, add ipset err: %s", err)
				continue
			}
		}
	}
	return nil
}

// TODO optimize the logic
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
		!utils.CompareNets(old.IPs, new.IPs) {
		return false
	}
	return true
}
