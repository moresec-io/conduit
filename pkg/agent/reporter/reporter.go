package reporter

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/agent/config"
	"github.com/moresec-io/conduit/pkg/proto"
	"github.com/moresec-io/conduit/pkg/utils"
	"github.com/singchia/geminio"
	"github.com/singchia/geminio/client"
)

type Reporter struct {
	machineid string
	end       geminio.End

	mtx     sync.RWMutex
	cluster map[string]net.IPNet // key: machineid, value: ipnets
}

func NewReporter(conf *config.Config) (*Reporter, error) {
	id, err := machineid.ID()
	if err != nil {
		return nil, err
	}
	dialer := func() (net.Conn, error) {
		return utils.DialRandom(&config.Conf.Conduit.Dial)
	}
	opt := client.NewEndOptions()
	opt.SetMeta([]byte(id))
	end, err := client.NewEndWithDialer(dialer, opt)
	if err != nil {
		return nil, err
	}
	return &Reporter{
		machineid: id,
		end:       end,
		cluster:   make(map[string]net.IPNet),
	}, nil
}

func (reporter *Reporter) sync() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		<-ticker.C
		err := reporter.reportAgent()
		if err != nil {
			log.Errorf("reporter sync, report agent err: %s", err)
			continue
		}
	}
}

func (reporter *Reporter) reportAgent() error {
	// agent network
	networks, err := utils.ListNetworks()
	if err != nil {
		return err
	}
	request := &proto.ReportAgentRequest{
		MachineID: reporter.machineid,
		IPNets:    networks,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := reporter.end.NewRequest(data)
	rsp, err := reporter.end.Call(context.TODO(), proto.RPCReportAgent, req)
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

	for _, agent := range response.Agents {
		for machineid, nets := range reporter.cluster {
			if agent.MachineID == machineid {
			}
		}
	}
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
