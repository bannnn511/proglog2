package discovery

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"net"
	"testing"
	"time"
)

type handler struct {
	joins  chan map[string]string
	leaves chan string
}

func (h handler) Join(name, addr string) error {
	if h.joins != nil {
		joined := map[string]string{
			name: addr,
		}
		h.joins <- joined
		fmt.Println("joined")
	}

	return nil
}

func (h handler) Leave(name string) error {
	if h.leaves != nil {
		h.leaves <- name
	}

	return nil
}

func TestMembership_setupSerf(t *testing.T) {
	type fields struct {
		Config  Config
		handler Handler
		serf    *serf.Serf
		events  chan serf.Event
		logger  *zap.Logger
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "test setup serf",
			fields: fields{
				Config:  Config{},
				handler: &handler{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Membership{
				Config:  tt.fields.Config,
				handler: tt.fields.handler,
			}
			if err := m.setupSerf(); err != nil {
				t.Errorf("setupSerf() error = %v", err)
			}
		})
	}
}

func setupMember(t *testing.T, members []*Membership) ([]*Membership, *handler) {
	id := len(members)
	port, err := getFreePort()
	require.NoError(t, err, "getFreePort")
	addr := fmt.Sprintf("%s:%d", "127.0.0.1", port)
	config := &Config{
		NodeName: fmt.Sprintf("node name %d", id),
		BindAddr: addr,
		Tags: map[string]string{
			"rpc_addr": addr,
		},
	}

	h := &handler{}
	if len(members) == 0 {
		h.joins = make(chan map[string]string, 3)
		h.leaves = make(chan string, 3)
	} else {
		config.StartJoinAddresses = []string{members[0].BindAddr}
	}

	m, err := New(h, *config)
	require.NoError(t, err, "New()")
	members = append(members, m)

	return members, h
}

func TestMemberShip(t *testing.T) {
	m, h := setupMember(t, nil)
	m, _ = setupMember(t, m)
	m, _ = setupMember(t, m)

	fmt.Println("=====================================", len(h.joins))
	require.Eventually(
		t,
		func() bool {
			return len(h.joins) == 2 &&
				len(m[0].Members()) == 3 &&
				len(h.leaves) == 0
		},
		time.Second*3,
		time.Millisecond*250,
	)

	require.NoError(t, m[2].Leave())

	require.Eventually(
		t,
		func() bool {
			return len(m[0].Members()) == 2 &&
				len(h.leaves) == 1
		},
		time.Second*3,
		time.Millisecond*250,
	)

}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
