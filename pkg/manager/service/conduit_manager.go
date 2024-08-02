package service

import (
	"context"
	"encoding/json"
	"net"
	"strings"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/manager/apis"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"github.com/moresec-io/conduit/pkg/manager/repo"
	"github.com/moresec-io/conduit/pkg/proto"
	"github.com/moresec-io/conduit/pkg/utils"
	"github.com/singchia/geminio"
	"github.com/singchia/geminio/delegate"
	"github.com/singchia/geminio/pkg/id"
	"github.com/singchia/geminio/server"
	"github.com/singchia/go-timer/v2"
)

type ConduitManager struct {
	*delegate.UnimplementedDelegate
	ln        net.Listener
	repo      repo.Repo
	tmr       timer.Timer
	idFactory id.IDFactory
}

func NewConduitManager(conf *config.Config, repo repo.Repo, tmr timer.Timer) (*ConduitManager, error) {
	listen := &conf.ConduitManager.Listen

	cm := &ConduitManager{
		tmr:                   tmr,
		repo:                  repo,
		idFactory:             id.DefaultIncIDCounter,
		UnimplementedDelegate: &delegate.UnimplementedDelegate{},
	}
	ln, err := utils.Listen(listen)
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
	return nil
}

func (cm *ConduitManager) register(end geminio.End) error {
	err := end.Register(context.TODO(), proto.RPCReportConduit, cm.ReportConduit)
	if err != nil {
		log.Errorf("conduit manager register, register ReportConduit err: %s", err)
		return err
	}
	err = end.Register(context.TODO(), proto.RPCPullCluster, cm.PullCluster)
	if err != nil {
		log.Errorf("conduit manager register, register PullCluster err: %s", err)
		return err
	}
	return nil
}

func (cm *ConduitManager) ReportConduit(_ context.Context, req geminio.Request, rsp geminio.Response) {
	request := &proto.ReportConduitRequest{}
	err := json.Unmarshal(req.Data(), request)
	if err != nil {
		rsp.SetError(err)
		return
	}
}

func (cm *ConduitManager) PullCluster(context.Context, geminio.Request, geminio.Response) {}
