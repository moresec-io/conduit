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

	// inflight ends
	mtx        sync.RWMutex
	machineIDs map[uint64]string    // key: clientID; value: machineID
	ends       map[string]*endNtime // key: machineID; value: end and create time
	conduits   map[string]Conduit   // key: machineID; value: Conduit
}

func NewConduitManager(conf *config.Config, repo repo.Repo, cms cms.CMS, tmr timer.Timer) (*ConduitManager, error) {
	listen := &conf.ConduitManager.Listen

	cm := &ConduitManager{
		tmr:                   tmr,
		repo:                  repo,
		idFactory:             id.DefaultIncIDCounter,
		UnimplementedDelegate: &delegate.UnimplementedDelegate{},
		ends:                  map[string]*endNtime{},
	}
	ln, err := network.Listen(listen)
	if err != nil {
		log.Errorf("conduit manager listen err: %s", err)
		return nil, err
	}
	cm.ln = ln
	return cm, nil
}

func (cm *ConduitManager) Serve() error {
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
	port, err := strconv.Atoi(portstr)
	if err != nil {
		rsp.SetError(err)
		return
	}
	ip := net.ParseIP(host)
	// set ip as cert san
	cert, err := cm.cms.GetCert(ip)
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

	serverConfig := &ServerConfig{
		Host: host,
		Port: port,
		Cert: cert,
	}

	cm.mtx.Lock()
	defer cm.mtx.Unlock()

	end, ok := cm.ends[request.MachineID]
	if !ok {
		conduit, ok := cm.conduits[request.MachineID]
		if !ok {
			rsp.SetError(errors.New("end not found"))
			return
		}
		conduit.SetServer(serverConfig)
		return
	}

	conduit := NewConduit(end.end)
	conduit.SetServer(serverConfig)
	cm.conduits[request.MachineID] = conduit
	// delete after transfer to conduits
	delete(cm.ends, request.MachineID)
}

func (cm *ConduitManager) ReportNetworks(_ context.Context, req geminio.Request, rsp geminio.Response) {
	request := &proto.ReportNetworksRequest{}
	err := json.Unmarshal(req.Data(), request)
	if err != nil {
		rsp.SetError(err)
		return
	}
}

func (cm *ConduitManager) PullCluster(context.Context, geminio.Request, geminio.Response) {}

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
		// it's normal to be here
		return nil
	}
	if conduit.IsServer() {
		// notify all clients
	}
	return nil
}
