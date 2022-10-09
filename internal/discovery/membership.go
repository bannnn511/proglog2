package discovery

import (
	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
	"net"
)

type Membership struct {
	Config
	handler Handler
	serf    *serf.Serf
	events  chan serf.Event
	logger  *zap.Logger
}

type Config struct {
	NodeName           string
	BindAddr           string
	Tags               map[string]string
	StartJoinAddresses []string
}

type Handler interface {
	Join(name, addr string) error
	Leave(name string) error
}

// New creates membership and setup serf.
func New(handler Handler, config Config) (*Membership, error) {
	logger, _ := zap.NewProduction()
	mem := &Membership{
		Config:  config,
		handler: handler,
		logger:  logger.Named("membership"),
	}

	err := mem.setupSerf()
	if err != nil {
		return nil, err
	}

	return mem, nil
}

// setupSerf setup configuration for serf and run goroutine for eventHandler().
// if StartJoinAddresses is not nil then joins clusters in StartJoinAddresses.
// If you want to join a cluster, StartJoinAddresses only needs 1 node address inside the cluster.
// Serf gossip protocol will take care of the rest to joins your node to the cluster.
func (m *Membership) setupSerf() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", m.BindAddr)
	if err != nil {
		return err
	}
	serfConfig := serf.DefaultConfig()

	serfConfig.MemberlistConfig.BindAddr = tcpAddr.IP.String()
	serfConfig.MemberlistConfig.BindPort = tcpAddr.Port
	m.events = make(chan serf.Event)
	serfConfig.EventCh = m.events
	serfConfig.Tags = m.Tags
	serfConfig.NodeName = m.NodeName
	serfConfig.Init()
	mSerf, err := serf.Create(serfConfig)
	if err != nil {
		return err
	}
	m.serf = mSerf

	go m.eventHandler()

	if m.StartJoinAddresses != nil {
		_, err := m.serf.Join(m.StartJoinAddresses, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// eventHandler() handle serf events: joins, leaves and failed of memberships.
func (m *Membership) eventHandler() {
	for event := range m.events {
		switch event.EventType() {
		case serf.EventMemberJoin:
			for _, mem := range event.(serf.MemberEvent).Members {
				if m.isLocal(mem) {
					continue
				}
				m.handleJoin(&mem)
			}
		case serf.EventMemberLeave, serf.EventMemberFailed:
			for _, mem := range event.(serf.MemberEvent).Members {
				if m.isLocal(mem) {
					return
				}
				m.handleLeave(&mem)
			}
		}
	}
}

// isLocal check is a serf.Member is local member of current membership's serf.
func (m *Membership) isLocal(mem serf.Member) bool {
	return m.serf.LocalMember().Name == mem.Name
}

// Members return membership of serf.
func (m *Membership) Members() []serf.Member {
	return m.serf.Members()
}

// handleJoin handles serf.EventMemberJoin.
func (m *Membership) handleJoin(mem *serf.Member) {
	if err := m.handler.Join(mem.Name, mem.Tags["rpc_addr"]); err != nil {
		m.logError(err, "failed to joins", *mem)
	}
}

func (m *Membership) Leave() error {
	return m.serf.Leave()
}

// handleJoin handles serf.EventMemberLeave and serf.EventMemberFailed.
func (m *Membership) handleLeave(mem *serf.Member) {
	if err := m.handler.Leave(mem.Name); err != nil {
		m.logError(err, "failed to leaves", *mem)
	}
}

func (m *Membership) logError(err error, msg string, member serf.Member) {
	m.logger.Error(
		msg,
		zap.Error(err),
		zap.String("name", member.Name),
		zap.String("rpc_addr", member.Tags["rpc_addr"]),
	)
}
