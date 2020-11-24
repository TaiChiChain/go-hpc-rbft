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

func TestCluster_MissingCheckpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)
	var retMessageSet []map[pb.Type][]*pb.ConsensusMessage
	for i := 0; i < 40; i++ {
		tx := newTx()
		retMessages := execute(t, rbfts, nodes, tx, false)
		retMessageSet = append(retMessageSet, retMessages)
	}

	var vcEvents []*pb.ConsensusMessage
	for index := range rbfts {
		event := &LocalEvent{
			Service:   CoreRbftService,
			EventType: CoreHighWatermarkEvent,
			Event:     rbfts[index].h,
		}
		rbfts[index].processEvent(event)
		vc := nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_VIEW_CHANGE, vc.Type)
		vcEvents = append(vcEvents, vc)
	}
	for index := range rbfts {
		for j := range vcEvents {
			if j == index {
				continue
			}
			quorumVC := rbfts[index].processEvent(vcEvents[j])
			if quorumVC != nil {
				done := &LocalEvent{
					Service:   ViewChangeService,
					EventType: ViewChangeQuorumEvent,
				}
				assert.Equal(t, done, quorumVC)
				rbfts[index].processEvent(quorumVC)
				break
			}
		}
	}
	nv := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, nv.Type)
	for index := range rbfts {
		rbfts[index].processEvent(nv)
	}

	for index := range rbfts {
		assert.Equal(t, uint64(40), rbfts[index].h)
	}
}

//******************************************************************************************************************
// some tools for unit tests
//******************************************************************************************************************
func unlockCluster(rbfts []*rbftImpl) {
	for index := range rbfts {
		rbfts[index].atomicOff(Pending)
		rbfts[index].setNormal()
	}
}

func setClusterViewExcept(rbfts []*rbftImpl, nodes []*testNode, view uint64, noSet int) {
	for index := range rbfts {
		if index == noSet {
			continue
		}
		rbfts[index].setView(view)
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
