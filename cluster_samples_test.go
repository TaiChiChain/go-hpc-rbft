package rbft

import (
	"fmt"
	"testing"

	"github.com/ultramesh/flato-common/types"
	"github.com/ultramesh/flato-common/types/protos"
	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// todo trigger step by step
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

func TestCluster_TFStart_TFStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tf := newTestFramework(4, false)

	tf.frameworkStart()
	for _, tn := range tf.TestNode {
		flag := tn.close == nil
		assert.False(t, flag)
	}

	tf.frameworkStop()
	for _, tn := range tf.TestNode {
		flag := tn.close == nil
		assert.True(t, flag)
	}
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

	for index, chkpt := range retMessagesCtx[pb.Type_CHECKPOINT] {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(chkpt)
	}

	for _, retMessages := range retMessageSet {
		for index, chkpt := range retMessages[pb.Type_CHECKPOINT] {
			if index == 2 {
				continue
			}
			rbfts[2].processEvent(chkpt)
		}
	}

	ev := <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(1), rbfts[2].epoch)
	assert.Equal(t, uint64(60), rbfts[2].exec.lastExec)
}

func TestCluster_FetchCheckpointAtRestart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	ctx := newCTX(defaultValidatorSet)
	executeExceptN(t, rbfts, nodes, ctx, true, 2)

	var retMessageSet []map[pb.Type][]*pb.ConsensusMessage
	for i := 0; i < 2; i++ {
		tx := newTx()
		seqValue := uint64(10 * (i + 1))
		setClusterExecExcept(rbfts, nodes, seqValue-1, 2)
		retMessages := executeExceptN(t, rbfts, nodes, tx, true, 2)
		retMessageSet = append(retMessageSet, retMessages)
	}

	// just trigger the fetch checkpoint for restarting node
	rbfts[2].epochMgr.configBatchToCheck = &pb.MetaState{
		Applied: uint64(1),
		Digest:  "tmp-hash",
	}
	_ = rbfts[2].start()
	close(rbfts[2].close)
	fetchEv := <-rbfts[2].recvChan
	rbfts[2].processEvent(fetchEv)
	fetchCheckpointMsg := nodes[2].broadcastMessageCache
	assert.True(t, rbfts[2].in(initialCheck))
	assert.Equal(t, pb.Type_FETCH_CHECKPOINT, fetchCheckpointMsg.Type)

	fetchedCheckpoints := make([]*pb.ConsensusMessage, 4)
	for index, rbft := range rbfts {
		if index == 2 {
			continue
		}
		rbft.processEvent(fetchCheckpointMsg)
		fetchedCheckpoints[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_CHECKPOINT, fetchedCheckpoints[index].Type)
	}

	for index, chkpt := range fetchedCheckpoints {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(chkpt)
	}

	recoveryMsg := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, recoveryMsg.Type)

	notificationRsp := make([]*pb.ConsensusMessage, 4)
	for index, rbft := range rbfts {
		if index == 2 {
			continue
		}
		rbft.processEvent(recoveryMsg)
		notificationRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, notificationRsp[index].Type)
	}

	for index, rsp := range notificationRsp {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(rsp)
	}

	ev := <-rbfts[2].recvChan
	done := rbfts[2].processEvent(ev)
	expectEv := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	assert.Equal(t, expectEv, done)

	rbfts[2].processEvent(done)

	assert.Equal(t, uint64(1), rbfts[2].epoch)
	assert.Equal(t, uint64(20), rbfts[2].exec.lastExec)

	msg := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_RECOVERY_FETCH_QPC, msg.Type)
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
		for index, chkpt := range retMessages[pb.Type_CHECKPOINT] {
			if index == 2 {
				continue
			}
			rbfts[2].processEvent(chkpt)
		}
	}
	fmt.Println(len(rbfts[2].recvChan))

	for index, chkpt := range retMessagesCtx[pb.Type_CHECKPOINT] {
		if index == 2 {
			continue
		}
		rbfts[2].processEvent(chkpt)
	}

	fmt.Println(len(rbfts[2].recvChan))

	ev := <-rbfts[2].recvChan
	rbfts[2].processEvent(ev)
	assert.Equal(t, uint64(51), rbfts[2].epoch)
	assert.Equal(t, uint64(51), rbfts[2].exec.lastExec)
}

func unlockCluster(rbfts []*rbftImpl) {
	for index := range rbfts {
		rbfts[index].atomicOff(Pending)
		rbfts[index].setNormal()
	}
}

func setClusterExec(rbfts []*rbftImpl, nodes []*testNode, seq uint64) {
	setClusterExecExcept(rbfts, nodes, seq, len(rbfts))
}

func setClusterExecExcept(rbfts []*rbftImpl, nodes []*testNode, seq uint64, noExec int) {
	for index := range rbfts {
		if index == noExec {
			continue
		}
		rbfts[index].exec.setLastExec(seq)
		rbfts[index].batchMgr.setSeqNo(seq)
		pos := seq / 10 * 10
		rbfts[index].moveWatermarks(pos)
		nodes[index].Applied = seq
	}
}

//******************************************************************************************************************
// assume that, it is in view 0, and the primary is node1, then we will process transactions:
// 1) "execute", we process transactions normally
// 2) "executeExceptPrimary", the backup replicas will process tx, but the primary only send a pre-prepare message
// 3) "executeExceptN", the node "N" will be abnormal and others will process transactions normally
//******************************************************************************************************************

func execute(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool) map[pb.Type][]*pb.ConsensusMessage {
	return executeExceptN(t, rbfts, nodes, tx, checkpoint, len(rbfts))
}

func executeExceptPrimary(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool) map[pb.Type][]*pb.ConsensusMessage {
	return executeExceptN(t, rbfts, nodes, tx, checkpoint, 0)
}

func executeExceptN(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool, notExec int) map[pb.Type][]*pb.ConsensusMessage {

	retMessages := make(map[pb.Type][]*pb.ConsensusMessage)

	rbfts[0].batchMgr.requestPool.AddNewRequest(tx, false, true)
	rbfts[1].batchMgr.requestPool.AddNewRequest(tx, false, true)
	rbfts[2].batchMgr.requestPool.AddNewRequest(tx, false, true)
	rbfts[3].batchMgr.requestPool.AddNewRequest(tx, false, true)

	batchTimerEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	rbfts[0].processEvent(batchTimerEvent)

	preprepMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_PRE_PREPARE, preprepMsg.Type)
	retMessages[pb.Type_PRE_PREPARE] = []*pb.ConsensusMessage{preprepMsg}

	prepMsg := make([]*pb.ConsensusMessage, len(rbfts))
	commitMsg := make([]*pb.ConsensusMessage, len(rbfts))
	checkpointMsg := make([]*pb.ConsensusMessage, len(rbfts))

	for index := range rbfts {
		if index == 0 || index == notExec {
			continue
		}
		rbfts[index].processEvent(preprepMsg)
		prepMsg[index] = nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_PREPARE, prepMsg[index].Type)
	}
	// for pre-prepare message
	// a primary won't process a pre-prepare, so that the prepMsg[0] will always be empty
	assert.Equal(t, (*pb.ConsensusMessage)(nil), prepMsg[0])
	retMessages[pb.Type_PREPARE] = prepMsg

	for index := range rbfts {
		if index == notExec {
			continue
		}
		for j := range prepMsg {
			if j == 0 || j == notExec {
				continue
			}
			rbfts[index].processEvent(prepMsg[j])
		}
		commitMsg[index] = nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_COMMIT, commitMsg[index].Type)
	}
	retMessages[pb.Type_COMMIT] = commitMsg

	for index := range rbfts {
		if index == notExec {
			continue
		}
		for j := range commitMsg {
			if j == notExec {
				continue
			}
			rbfts[index].processEvent(commitMsg[j])
		}
		if checkpoint {
			checkpointMsg[index] = nodes[index].broadcastMessageCache
			assert.Equal(t, pb.Type_CHECKPOINT, checkpointMsg[index].Type)
		}
	}
	retMessages[pb.Type_CHECKPOINT] = checkpointMsg

	if checkpoint {
		if types.IsConfigTx(tx) {
			for index := range rbfts {
				if index == notExec {
					continue
				}
				for j := range checkpointMsg {
					if j == notExec {
						continue
					}
					rbfts[index].processEvent(checkpointMsg[j])
				}
			}
		} else {
			for index := range rbfts {
				if index == notExec {
					continue
				}
				for j := range checkpointMsg {
					if j == notExec {
						continue
					}
					rbfts[index].processEvent(checkpointMsg[j])
				}
			}
		}
	}

	return retMessages
}
