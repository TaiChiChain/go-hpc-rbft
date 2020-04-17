package rbft

import (
	"testing"

	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestStatusMgr_inOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:          1,
		Hash:        "node1",
		IsNew:       false,
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: tx,
	}

	cpChan := make(chan *pb.ServiceState)
	confC := make(chan bool)
	rbft, _ := newRBFT(cpChan, confC, conf)

	rbft.on(InViewChange)
	assert.Equal(t, true, rbft.inOne(InViewChange, InRecovery, InUpdatingN))
}

func TestStatusMgr_setState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:          1,
		Hash:        "node1",
		IsNew:       false,
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: tx,
	}

	cpChan := make(chan *pb.ServiceState)
	confC := make(chan bool)
	rbft, _ := newRBFT(cpChan, confC, conf)

	rbft.setNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.setFull()
	assert.Equal(t, true, rbft.in(PoolFull))
}

func TestStatusMgr_maybeSetNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:          1,
		Hash:        "node1",
		IsNew:       false,
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: tx,
	}

	cpChan := make(chan *pb.ServiceState)
	confC := make(chan bool)
	rbft, _ := newRBFT(cpChan, confC, conf)

	rbft.off(InRecovery)
	rbft.off(InUpdatingN)
	rbft.off(InViewChange)
	rbft.off(StateTransferring)
	rbft.off(Pending)
	rbft.maybeSetNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.on(InRecovery)
	rbft.maybeSetNormal()
	assert.Equal(t, true, rbft.in(Normal))
}
