package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/manager/apis"
	"github.com/moresec-io/conduit/pkg/manager/cms"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"github.com/moresec-io/conduit/pkg/manager/repo"
	"github.com/moresec-io/conduit/pkg/network"
	"github.com/moresec-io/conduit/pkg/proto"
	"github.com/singchia/geminio"
	"github.com/singchia/geminio/delegate"
	"github.com/singchia/geminio/pkg/id"
	"github.com/singchia/geminio/server"
	"github.com/singchia/go-timer/v2"
)

type endNtime struct {
	end        geminio.End
	createTime time.Time
}

type eventType int

const (
	_ eventType = iota
	eventTypeServerOnline
	eventTypeServerOffline
	eventTypeServerNetworkChanged
)

type event struct {
	eventType eventType
	conduit   Conduit
}

type ConduitManager struct {
	*delegate.UnimplementedDelegate
	ln        net.Listener
	repo      repo.Repo
	tmr       timer.Timer
	cms       cms.CMS
	idFactory id.IDFactory
	// event channel
	eventCh chan *event

	// inflight ends
	mtx        sync.RWMutex
	machineIDs map[uint64]string    // key: clientID; value: machineID
	ends       map[string]*endNtime // key: machineID; value: end and create time
	conduits   map[string]Conduit   // key: machineID; value: Conduit
}

func NewConduitManager(conf *config.Config, repo repo.Repo, cms cms.CMS, tmr timer.Timer) (*ConduitManager, error) {
	listen := &conf.ConduitManager.Listen

	cm := &ConduitManager{
		UnimplementedDelegate: &delegate.UnimplementedDelegate{},
		tmr:                   tmr,
		repo:                  repo,
		idFactory:             id.DefaultIncIDCounter,
		eventCh:               make(chan *event, 1024),
		machineIDs:            map[uint64]string{},
		ends:                  map[string]*endNtime{},
		conduits:              map[string]Conduit{},
	}
	ln, err := network.Listen(listen)
	if err != nil {
		log.Errorf("conduit manager listen err: %s", err)
		return nil, err
	}
	cm.ln = ln
	go cm.notify()

	return cm, nil
}

func (cm *ConduitManager) Serve() {
	go func() error {
		for {
			conn, err := cm.ln.Accept()
			if err != nil {
				if !strings.Contains(err.Error(), apis.ErrStrUseOfClosedConnection) {
					return err
				}
				break
			}
			go cm.handleConn(conn)
		}
		return nil
	}()
}

func (cm *ConduitManager) notify() {
	for {
		event, ok := <-cm.eventCh
		if !ok {
			return
		}
		switch event.eventType {
		case eventTypeServerOffline:
			for _, conduit := range cm.conduits {
				if conduit.IsClient() {
					if conduit.MachineID() == event.conduit.MachineID() {
						// ignore the event source conduit
						continue
					}
					// notify all clients
					err := conduit.ServerOffline(event.conduit.MachineID())
					if err != nil {
						log.Errorf("conduit manager, call conduit server offline err: %s", err)
					}
				}
			}
		case eventTypeServerOnline:
			for _, conduit := range cm.conduits {
				if conduit.IsClient() {
					if conduit.MachineID() == event.conduit.MachineID() {
						// ignore the event source conduit
						continue
					}
					// notify all clients
					err := conduit.ServerOnline(&proto.Conduit{
						MachineID: event.conduit.MachineID(),
						Network:   event.conduit.GetServerConfig().Network,
						Addr:      event.conduit.GetServerConfig().Addr,
						IPs:       event.conduit.GetServerConfig().IPs,
					})
					if err != nil {
						log.Errorf("conduit manager, call conduit server online err: %s", err)
					}
				}
			}
		case eventTypeServerNetworkChanged:
			for _, conduit := range cm.conduits {
				if conduit.MachineID() == event.conduit.MachineID() {
					// ignore the event source conduit
					continue
				}
				// notify all clients
				err := conduit.ServerNetworksChanged(conduit.MachineID(), event.conduit.GetServerConfig().IPs)
				if err != nil {
					log.Errorf("conduit manager, call conduit server network changed err: %s", err)
				}
			}
		}
	}
}

func (cm *ConduitManager) handleConn(conn net.Conn) error {
	// options for geminio End
	opt := server.NewEndOptions()
	opt.SetTimer(cm.tmr)
	opt.SetDelegate(cm)
	end, err := server.NewEndWithConn(conn, opt)
	if err != nil {
		log.Errorf("conduit manager handle conn, geminio server new err: %s", err)
		return err
	}
	err = cm.register(end)
	if err != nil {
		log.Errorf("conduit manager handle conn, register err: %s", err)
		return err
	}
	log.Infof("conduit manager handle conn: %s", conn.RemoteAddr().String())

	cm.mtx.Lock()
	defer cm.mtx.Unlock()
	cm.ends[string(end.Meta())] = &endNtime{
		end:        end,
		createTime: time.Now(),
	}
	cm.machineIDs[end.ClientID()] = string(end.Meta())
	return nil
}

func (cm *ConduitManager) register(end geminio.End) error {
	// register ReportNetworks function
	err := end.Register(context.TODO(), proto.RPCReportNetworks, cm.ReportNetworks)
	if err != nil {
		log.Errorf("conduit manager register, register ReportConduit err: %s", err)
		return err
	}
	// register ReportClient function
	err = end.Register(context.TODO(), proto.RPCReportClient, cm.ReportClient)
	if err != nil {
		log.Errorf("conduit manager register, register ReportClient err: %s", err)
		return err
	}
	// register ReportServer function
	err = end.Register(context.TODO(), proto.RPCReportServer, cm.ReportServer)
	if err != nil {
		log.Errorf("conduit manager register, register ReportServer err: %s", err)
		return err
	}
	// register PullCluster function
	err = end.Register(context.TODO(), proto.RPCPullCluster, cm.PullCluster)
	if err != nil {
		log.Errorf("conduit manager register, register PullCluster err: %s", err)
		return err
	}

	log.Infof("conduit manager register functions for end: %s success", end.RemoteAddr().String())
	return nil
}

func (cm *ConduitManager) ReportClient(_ context.Context, req geminio.Request, rsp geminio.Response) {
	request := &proto.ReportClientRequest{}
	err := json.Unmarshal(req.Data(), request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	cert, err := cm.cms.GetClientCert()
	if err != nil {
		rsp.SetError(err)
		return
	}
	response := &proto.ReportClientResponse{
		TLS: &proto.TLS{
			CA:   cert.CA,
			Cert: cert.Cert,
			Key:  cert.Key,
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		rsp.SetError(err)
		return
	}
	rsp.SetData(data)

	cm.mtx.Lock()
	defer cm.mtx.Unlock()

	end, ok := cm.ends[request.MachineID]
	if !ok {
		conduit, ok := cm.conduits[request.MachineID]
		if !ok {
			rsp.SetError(errors.New("end not found"))
			return
		}
		conduit.SetClient()
		return
	}

	conduit := NewConduit(end.end)
	conduit.SetClient()
	cm.conduits[request.MachineID] = conduit
	// delete after transfer to conduits
	delete(cm.ends, request.MachineID)
}

// server report to manager
func (cm *ConduitManager) ReportServer(_ context.Context, req geminio.Request, rsp geminio.Response) {
	request := &proto.ReportServerRequest{}
	err := json.Unmarshal(req.Data(), request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	// request.Addr should be ip:port, conduit server side's listen addr must be specific
	host, portstr, err := net.SplitHostPort(request.Addr)
	if err != nil {
		rsp.SetError(err)
		return
	}
	_, err = strconv.Atoi(portstr)
	if err != nil {
		rsp.SetError(err)
		return
	}
	ip := net.ParseIP(host)
	// set ip as cert san
	cert, err := cm.cms.GetServerCert(ip)
	if err != nil {
		rsp.SetError(err)
		return
	}

	// TODO transcation for reponse and cache
	response := &proto.ReportServerResponse{
		TLS: &proto.TLS{
			CA:   cert.CA,
			Cert: cert.Cert,
			Key:  cert.Key,
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		rsp.SetError(err)
		return
	}
	rsp.SetData(data)

	// store
	serverConfig := &ServerConfig{
		Network: request.Network,
		Addr:    request.Addr,
		Cert:    cert,
		IPs:     request.IPs,
	}

	cm.mtx.Lock()
	defer cm.mtx.Unlock()
	// cache it
	end, ok := cm.ends[request.MachineID]
	if !ok {
		conduit, ok := cm.conduits[request.MachineID]
		if !ok {
			rsp.SetError(errors.New("end not found"))
			return
		}
		conduit.SetServer(serverConfig)
		// server conduit online event
		cm.eventCh <- &event{
			eventType: eventTypeServerOnline,
			conduit:   conduit,
		}
	} else {
		conduit := NewConduit(end.end)
		conduit.SetServer(serverConfig)
		cm.conduits[request.MachineID] = conduit
		// delete after transfer to conduits
		delete(cm.ends, request.MachineID)
	}
}

// server report to manager
func (cm *ConduitManager) ReportNetworks(_ context.Context, req geminio.Request, rsp geminio.Response) {
	request := &proto.ReportNetworksRequest{}
	err := json.Unmarshal(req.Data(), request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	// update server networks
	cm.mtx.RLock()
	conduit, ok := cm.conduits[request.MachineID]
	if !ok {
		rsp.SetError(errors.New("end not found"))
		cm.mtx.RUnlock()
		return
	}
	conduit.SetServerIPs(request.IPs)
	cm.mtx.RUnlock()

	// server conduit network update event
	cm.eventCh <- &event{
		eventType: eventTypeServerNetworkChanged,
		conduit:   conduit,
	}
}

func (cm *ConduitManager) PullCluster(_ context.Context, req geminio.Request, rsp geminio.Response) {
	request := &proto.PullClusterRequest{}
	err := json.Unmarshal(req.Data(), request)
	if err != nil {
		rsp.SetError(err)
		return
	}
	// pull all server conduits
	cm.mtx.RLock()
	conduits := []proto.Conduit{}
	for _, conduit := range cm.conduits {
		if conduit.IsServer() {
			conduits = append(conduits, proto.Conduit{
				MachineID: conduit.MachineID(),
				Network:   conduit.GetServerConfig().Network,
				Addr:      conduit.GetServerConfig().Addr,
				IPs:       conduit.GetServerConfig().IPs,
			})
		}
	}
	cm.mtx.RUnlock()

	// return to clients
	response := &proto.PullClusterResponse{
		Cluster: conduits,
	}
	data, err := json.Marshal(response)
	if err != nil {
		rsp.SetError(err)
		return
	}
	rsp.SetData(data)
}

// connection layer offline
func (cm *ConduitManager) ConnOffline(cb delegate.ConnDescriber) error {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()

	log.Infof("conduit manager conn: %s offline", cb.RemoteAddr().String())

	machineID, ok := cm.machineIDs[cb.ClientID()]
	if !ok {
		log.Errorf("conduit manager conn: %s offline, but machineID not found")
		return nil
	}
	// delete inflight ends
	delete(cm.ends, machineID)
	// delete stored conduit
	conduit, ok := cm.conduits[machineID]
	if !ok {
		// it's normal to be here when end connected but not registered
		return nil
	}
	if conduit.IsServer() {
		// notify all clients
		cm.eventCh <- &event{
			eventType: eventTypeServerOffline,
			conduit:   conduit,
		}
	}
	return nil
}

func (cm *ConduitManager) Close() {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()

	for _, end := range cm.ends {
		end.end.Close()
	}

	for _, conduit := range cm.conduits {
		conduit.Close()
	}
}
