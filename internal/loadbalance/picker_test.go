package loadbalance_test

import (
	"proglog/internal/loadbalance"
	"testing"

	"google.golang.org/grpc/attributes"

	"google.golang.org/grpc/resolver"

	"google.golang.org/grpc/balancer/base"

	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/balancer"
)

func TestPickerNoSubAvailable(t *testing.T) {
	picker := loadbalance.Picker{}
	for _, method := range []string{
		"/log.vX.Log/Produce",
		"/log.vX.Log/Consume",
	} {
		info := balancer.PickInfo{
			FullMethodName: method,
		}

		pick, err := picker.Pick(info)
		require.Equal(t, balancer.ErrNoSubConnAvailable, err)
		require.Nil(t, pick.SubConn)
	}
}

func TestPickerProducesToLeader(t *testing.T) {
	picker, subConns := setupTest(t)

	type test struct {
		name   string
		method string
	}

	testcases := []test{
		{
			name:   "test produce pick leader",
			method: "/log.vX.Log/Produce",
		},
		{
			name:   "test consume pick Leader",
			method: "/log.vX.Log/Produce",
		},
	}

	for _, tCase := range testcases {
		t.Run(tCase.name, func(t *testing.T) {
			info := balancer.PickInfo{
				FullMethodName: tCase.method,
			}
			pick, err := picker.Pick(info)
			require.NoError(t, err)
			require.Equal(t, subConns[0], pick.SubConn)
		})
	}
}

func TestPickerProducesToFollower(t *testing.T) {
	picker, subConns := setupTest(t)

	type test struct {
		name   string
		method string
	}

	testcases := []test{
		{
			name:   "test produce pick follower",
			method: "/log.vX.Log/Consume",
		},
		{
			name:   "test consume pick follower",
			method: "/log.vX.Log/Consume",
		},
	}

	for i, tCase := range testcases {
		t.Run(tCase.name, func(t *testing.T) {
			info := balancer.PickInfo{
				FullMethodName: tCase.method,
			}
			pick, err := picker.Pick(info)
			require.NoError(t, err)
			require.Equal(t, subConns[i%2+1], pick.SubConn)
		})
	}
}

func setupTest(t *testing.T) (*loadbalance.Picker, []*subconn) {
	t.Helper()

	var subConInfos []*subconn
	picker := loadbalance.Picker{}
	readyCs := make(map[balancer.SubConn]base.SubConnInfo)
	for i := 0; i < 3; i++ {
		subConn := subconn{}
		scInfo := base.SubConnInfo{
			Address: resolver.Address{
				Attributes: attributes.New("is_leader", i == 0),
			},
		}
		readyCs[&subConn] = scInfo
		subConn.UpdateAddresses([]resolver.Address{scInfo.Address})
		subConInfos = append(subConInfos, &subConn)
	}

	info := base.PickerBuildInfo{ReadySCs: readyCs}
	picker.Build(info)

	return &picker, subConInfos
}

type subconn struct {
	addresses []resolver.Address
}

func (s *subconn) UpdateAddresses(addresses []resolver.Address) {
	s.addresses = addresses
}

func (s *subconn) Connect() {
}
