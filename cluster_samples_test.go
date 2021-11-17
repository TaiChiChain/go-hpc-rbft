package rbft

import (
	"testing"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestCluster_SendTx_InitCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	assert.Equal(t, rbfts[0].h, uint64(0))
	assert.Equal(t, rbfts[1].h, uint64(0))
	assert.Equal(t, rbfts[2].h, uint64(0))
	assert.Equal(t, rbfts[3].h, uint64(0))

	ctx := newCTX(defaultValidatorSet)
	execute(t, rbfts, nodes, ctx, true)

	tx := newTx()
	execute(t, rbfts, nodes, tx, false)

	assert.Equal(t, uint64(1), rbfts[0].h)
	assert.Equal(t, uint64(1), rbfts[1].h)
	assert.Equal(t, uint64(1), rbfts[2].h)
	assert.Equal(t, uint64(1), rbfts[3].h)

	assert.Equal(t, uint64(2), rbfts[0].exec.lastExec)
	assert.Equal(t, uint64(2), rbfts[1].exec.lastExec)
	assert.Equal(t, uint64(2), rbfts[2].exec.lastExec)
	assert.Equal(t, uint64(2), rbfts[3].exec.lastExec)

	setClusterExec(rbfts, nodes, uint64(59))
	assert.Equal(t, uint64(50), rbfts[0].h)
	assert.Equal(t, uint64(50), rbfts[1].h)
	assert.Equal(t, uint64(50), rbfts[2].h)
	assert.Equal(t, uint64(50), rbfts[3].h)

	tx2 := newTx()
	execute(t, rbfts, nodes, tx2, true)

	assert.Equal(t, uint64(60), rbfts[0].h)
	assert.Equal(t, uint64(60), rbfts[1].h)
	assert.Equal(t, uint64(60), rbfts[2].h)
	assert.Equal(t, uint64(60), rbfts[3].h)

	assert.Equal(t, uint64(60), rbfts[0].exec.lastExec)
	assert.Equal(t, uint64(60), rbfts[1].exec.lastExec)
	assert.Equal(t, uint64(60), rbfts[2].exec.lastExec)
	assert.Equal(t, uint64(60), rbfts[3].exec.lastExec)
}

func TestCluster_SyncSmallEpochWithCheckpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	ctx := newCTX(defaultValidatorSet)
	retMessagesCtx := executeExceptN(t, rbfts, nodes, ctx, true, 2)

	var retMessageSet []map[pb.Type][]*pb.ConsensusMessage
	for i := 0; i < 6; i++ {
		tx := newTx()
		seqValue := uint64(10 * (i + 1))
		setClusterExecExcept(rbfts, nodes, seqValue-1, 2)
		retMessages := executeExceptN(t, rbfts, nodes, tx, true, 2)
		retMessageSet = append(retMessageSet, retMessages)
	}

	for index, chkpt := range retMessagesCtx[pb.Type_SIGNED_CHECKPOINT] {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(chkpt)
	}

	for _, retMessages := range retMessageSet {
		for index, chkpt := range retMessages[pb.Type_SIGNED_CHECKPOINT] {
			if index == 2 {
				continue
			}
			rbfts[2].processEvent(chkpt)
		}
	}

	ntfMsg := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, ntfMsg.Type)

	var rspList []*pb.ConsensusMessage
	for index := range rbfts {
		if index == 2 {
			continue
		}
		rbfts[index].processEvent(ntfMsg)
		rspMsg := nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, rspMsg.Type)
		rspList = append(rspList, rspMsg)
	}

	for _, rsp := range rspList {
		rbfts[2].processEvent(rsp)
	}
	// first state update to 50( which is larger than initial high watermark 40)
	ev := <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	// then state update to 60
	ev = <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(1), rbfts[2].epoch)
	assert.Equal(t, uint64(60), rbfts[2].h)
}

func TestCluster_SyncLargeEpochWithCheckpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	var retMessageSet []map[pb.Type][]*pb.ConsensusMessage
	for i := 0; i < 5; i++ {
		tx := newTx()
		seqValue := uint64(10 * (i + 1))
		setClusterExecExcept(rbfts, nodes, seqValue-1, 2)
		retMessages := executeExceptN(t, rbfts, nodes, tx, true, 2)
		retMessageSet = append(retMessageSet, retMessages)
	}

	ctx := newCTX(defaultValidatorSet)
	retMessagesCtx := executeExceptN(t, rbfts, nodes, ctx, true, 2)

	for _, retMessages := range retMessageSet {
		for index, chkpt := range retMessages[pb.Type_SIGNED_CHECKPOINT] {
			if index == 2 {
				continue
			}
			rbfts[2].processEvent(chkpt)
		}
	}

	for index, chkpt := range retMessagesCtx[pb.Type_SIGNED_CHECKPOINT] {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(chkpt)
	}

	ntfMsg := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, ntfMsg.Type)

	var rspList []*pb.ConsensusMessage
	for index := range rbfts {
		if index == 2 {
			continue
		}
		rbfts[index].processEvent(ntfMsg)
		rspMsg := nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, rspMsg.Type)
		rspList = append(rspList, rspMsg)
	}

	for _, rsp := range rspList {
		rbfts[2].processEvent(rsp)
	}
	// first state update to 50( which is larger than initial high watermark 40)
	ev := <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	// then state update to 51
	ev = <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(51), rbfts[2].epoch)
	assert.Equal(t, uint64(51), rbfts[2].exec.lastExec)
}
