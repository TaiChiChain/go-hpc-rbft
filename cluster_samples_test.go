package rbft

import (
	"testing"

	"github.com/golang/mock/gomock"
	consensus "github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
	"github.com/stretchr/testify/assert"
)

func TestCluster_SendTx_InitCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster[consensus.Transaction](rbfts)

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

func TestCluster_SyncEpochThenSyncBlockWithCheckpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)

	ctx := newCTX(defaultValidatorSet)
	retMessagesCtx := executeExceptN(t, rbfts, nodes, ctx, true, 2)

	var retMessageSet []map[consensus.Type][]*consensusMessageWrapper
	for i := 0; i < 6; i++ {
		tx := newTx()
		seqValue := uint64(10 * (i + 1))
		setClusterExecExcept(rbfts, nodes, seqValue-1, 2)
		retMessages := executeExceptN(t, rbfts, nodes, tx, true, 2)
		retMessageSet = append(retMessageSet, retMessages)
	}

	for index, chkpt := range retMessagesCtx[consensus.Type_SIGNED_CHECKPOINT] {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(chkpt)
	}

	for _, retMessages := range retMessageSet {
		for index, chkpt := range retMessages[consensus.Type_SIGNED_CHECKPOINT] {
			if index == 2 {
				continue
			}
			rbfts[2].processEvent(chkpt)
		}
	}

	// lagging node start retrieve epoch change proof.
	epochRequestMsg := nodes[2].unicastMessageCache
	assert.Equal(t, consensus.Type_EPOCH_CHANGE_REQUEST, epochRequestMsg.Type)

	var rspList []*consensusMessageWrapper
	for index := range rbfts {
		if index == 2 {
			continue
		}
		// response epoch change proof.
		rbfts[index].processEvent(epochRequestMsg)
		rspMsg := nodes[index].unicastMessageCache
		assert.Equal(t, consensus.Type_EPOCH_CHANGE_PROOF, rspMsg.Type)
		rspList = append(rspList, rspMsg)
	}

	// process epoch change proof and enter sync chain.
	for _, rsp := range rspList {
		ev := rbfts[2].processEvent(rsp)
		if ev != nil {
			assert.Equal(t, EpochSyncEvent, ev.(*LocalEvent).EventType)
			rbfts[2].processEvent(ev)
			break
		}
	}
	// first state update to 1(last epoch change height), then trigger recovery view change.
	ev := <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(2), rbfts[2].epoch)
	assert.Equal(t, uint64(1), rbfts[2].h)
	rvc := nodes[2].broadcastMessageCache
	// try recovery view change after epoch change.
	assert.Equal(t, consensus.Type_VIEW_CHANGE, rvc.Type)
	var newViewRspList []*consensusMessageWrapper
	for index := range rbfts {
		if index == 2 {
			continue
		}
		// response fetch view response.
		rbfts[index].processEvent(rvc)
		rspMsg := nodes[index].unicastMessageCache
		assert.Equal(t, consensus.Type_RECOVERY_RESPONSE, rspMsg.Type)
		newViewRspList = append(newViewRspList, rspMsg)
	}

	// process fetch view response and enter sync chain.
	for _, rsp := range newViewRspList {
		ev = rbfts[2].processEvent(rsp)
		if ev != nil {
			assert.Equal(t, ViewChangeDoneEvent, ev.(*LocalEvent).EventType)
			rbfts[2].processEvent(ev)
			break
		}
	}

	// then state update to 60
	ev = <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(60), rbfts[2].h)
}

func TestCluster_SyncBlockThenSyncEpochWithCheckpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)

	// normal nodes execute normal txs to height 50.
	var retMessageSet []map[consensus.Type][]*consensusMessageWrapper
	for i := 0; i < 5; i++ {
		tx := newTx()
		seqValue := uint64(10 * (i + 1))
		setClusterExecExcept(rbfts, nodes, seqValue-1, 2)
		retMessages := executeExceptN(t, rbfts, nodes, tx, true, 2)
		retMessageSet = append(retMessageSet, retMessages)
	}

	// node2 receives checkpoints of height 50 and trigger vc and sync chain.
	for _, retMessages := range retMessageSet {
		for index, chkpt := range retMessages[consensus.Type_SIGNED_CHECKPOINT] {
			if index == 2 {
				continue
			}
			rbfts[2].processEvent(chkpt)
		}
	}

	vcMsg := nodes[2].broadcastMessageCache
	assert.Equal(t, consensus.Type_VIEW_CHANGE, vcMsg.Type)

	// first state update to 50(last checkpoint height).
	ev := <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(1), rbfts[2].epoch)
	assert.Equal(t, uint64(50), rbfts[2].h)

	// normal nodes execute config txs to height 51 and broadcast recovery vc to view
	// change.
	ctx := newCTX(defaultValidatorSet)
	retMessagesCtx := executeExceptN(t, rbfts, nodes, ctx, true, 2)

	// node2 receives recovery view change and found self's epoch behind.
	for index, recoveryVCS := range retMessagesCtx[consensus.Type_VIEW_CHANGE] {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(recoveryVCS)
	}

	// lagging node start retrieve epoch change proof.
	epochRequestMsg := nodes[2].unicastMessageCache
	assert.Equal(t, consensus.Type_EPOCH_CHANGE_REQUEST, epochRequestMsg.Type)

	var rspList []*consensusMessageWrapper
	for index := range rbfts {
		if index == 2 {
			continue
		}
		// response epoch change proof.
		rbfts[index].processEvent(epochRequestMsg)
		rspMsg := nodes[index].unicastMessageCache
		assert.Equal(t, consensus.Type_EPOCH_CHANGE_PROOF, rspMsg.Type)
		rspList = append(rspList, rspMsg)
	}

	// process epoch change proof and enter sync chain.
	for _, rsp := range rspList {
		ev = rbfts[2].processEvent(rsp)
		if ev != nil {
			assert.Equal(t, EpochSyncEvent, ev.(*LocalEvent).EventType)
			rbfts[2].processEvent(ev)
			break
		}
	}
	// first state update to 1(last epoch change height), then trigger recovery view change.
	ev = <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(2), rbfts[2].epoch)
	assert.Equal(t, uint64(51), rbfts[2].h)
	rvc := nodes[2].broadcastMessageCache
	// try recovery view change after epoch change.
	assert.Equal(t, consensus.Type_VIEW_CHANGE, rvc.Type)
	var newViewRspList []*consensus.ConsensusMessage
	for index := range rbfts {
		if index == 2 {
			continue
		}
		// response fetch view response.
		rbfts[index].processEvent(rvc)
		rspMsg := nodes[index].unicastMessageCache
		assert.Equal(t, consensus.Type_RECOVERY_RESPONSE, rspMsg.Type)
		newViewRspList = append(newViewRspList, rspMsg.ConsensusMessage)
	}

	// process fetch view response and enter sync chain.
	for _, rsp := range newViewRspList {
		ev = rbfts[2].processEvent(rsp)
		if ev != nil {
			assert.Equal(t, ViewChangeDoneEvent, ev.(*LocalEvent).EventType)
			rbfts[2].processEvent(ev)
			break
		}
	}

}
