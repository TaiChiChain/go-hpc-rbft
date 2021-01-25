package rbft

import (
	"testing"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

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

	newPrimaryIndex := 1
	var ntfMsgs []*pb.ConsensusMessage
	recoveryDone := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	for index := range rbfts {
		event := &LocalEvent{
			Service:   CoreRbftService,
			EventType: CoreHighWatermarkEvent,
			Event:     rbfts[index].h,
		}
		rbfts[index].processEvent(event)
		ntf := nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION, ntf.Type)
		ntfMsgs = append(ntfMsgs, ntf)
	}
	for index := range rbfts {
		for j := range ntfMsgs {
			if j == index {
				continue
			}
			quorumRe := rbfts[index].processEvent(ntfMsgs[j])
			if quorumRe != nil {
				done := &LocalEvent{
					Service:   RecoveryService,
					EventType: NotificationQuorumEvent,
				}
				assert.Equal(t, done, quorumRe)
				if index == newPrimaryIndex {
					ev := rbfts[index].processEvent(quorumRe)
					assert.Equal(t, recoveryDone, ev)
				} else {
					rbfts[index].processEvent(quorumRe)
				}
				break
			}
		}
	}

	nv := nodes[newPrimaryIndex].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, nv.Type)
	for index := range rbfts {
		if index == newPrimaryIndex {
			continue
		}
		ev := rbfts[index].processEvent(nv)
		assert.Equal(t, recoveryDone, ev)
	}

	for index := range rbfts {
		rbfts[index].processEvent(recoveryDone)
	}

	for index := range rbfts {
		assert.Equal(t, uint64(40), rbfts[index].h)
	}
}

func TestCluster_CheckpointToViewChange(t *testing.T) {
	// test sample ===========================================================================================
	//
	// condition: 1) a cluster with 4 replicas, {node1, node2, node3, node4};
	//            2) current view is 0, which means node1 is the primary;
	//            2) every one finished the consensus on block n;
	//            3) 3 of them, such as {node1, node3, node4}, have send the checkpoint;
	//            4) last one, such as node2, hasn't send checkpoint yet
	//            5) trigger view-change, and the node2, which hasn't send checkpoint, become the next leader
	// expected: for node2 should open the high-watermark timer
	// ========================================================================================================

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

	missingNodeIndex := 1
	for key := range rbfts[missingNodeIndex].storeMgr.chkpts {
		if key == 0 {
			continue
		}
		delete(rbfts[missingNodeIndex].storeMgr.chkpts, key)
	}

	vcEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeTimerEvent,
	}
	vcQuorum := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	vcDone := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangedEvent,
	}

	var vcMsgSet []*pb.ConsensusMessage
	for index := range rbfts {
		rbfts[index].processEvent(vcEvent)
		assert.False(t, rbfts[index].timerMgr.getTimer(highWatermarkTimer))

		vcMsg := nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_VIEW_CHANGE, vcMsg.Type)
		vcMsgSet = append(vcMsgSet, vcMsg)
	}

	for index := range rbfts {
		for i, vcMsg := range vcMsgSet {
			if index == i {
				continue
			}
			done := rbfts[index].processEvent(vcMsg)
			if done != nil {
				assert.Equal(t, vcQuorum, done)
				break
			}
		}
	}

	for index := range rbfts {
		done := rbfts[index].processEvent(vcQuorum)
		if done != nil {
			assert.Equal(t, missingNodeIndex, index)
			assert.Equal(t, vcDone, done)
		}
	}

	nvMsg := nodes[missingNodeIndex].broadcastMessageCache
	for index := range rbfts {
		if index == missingNodeIndex {
			continue
		}
		done := rbfts[index].processEvent(nvMsg)
		assert.Equal(t, vcDone, done)
	}

	rbfts[missingNodeIndex].processEvent(vcDone)
	assert.True(t, rbfts[missingNodeIndex].timerMgr.getTimer(highWatermarkTimer))
}

func TestCluster_ReceiveNotificationBeforeStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	initRecoveryEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryInitEvent,
	}

	quorumEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: NotificationQuorumEvent,
	}

	doneEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}

	rbfts[0].atomicOff(Pending)
	rbfts[0].processEvent(initRecoveryEvent)

	ntfMsgNode1 := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, ntfMsgNode1.Type)

	// pending replicas receive message
	for index := range rbfts {
		if index == 0 {
			continue
		}
		rbfts[index].node.Step(ntfMsgNode1)
	}

	rbfts[1].atomicOff(Pending)
	rbfts[1].recoveryMgr.syncReceiver.Range(rbfts[1].readMap)
	rbfts[1].processEvent(initRecoveryEvent)

	ntfMsgNode2 := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, ntfMsgNode2.Type)

	// pending replicas receive message
	for index := range rbfts {
		if index == 0 || index == 1 {
			continue
		}
		rbfts[index].node.Step(ntfMsgNode2)
	}
	// started replicas receive message
	rbfts[0].processEvent(ntfMsgNode2)

	rbfts[2].atomicOff(Pending)
	rbfts[2].recoveryMgr.syncReceiver.Range(rbfts[2].readMap)
	quorumNode3 := rbfts[2].processEvent(initRecoveryEvent)
	assert.Equal(t, quorumEvent, quorumNode3)

	ntfMsgNode3 := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, ntfMsgNode3.Type)

	// pending replicas receive message
	for index := range rbfts {
		if index == 0 || index == 1 || index == 2 {
			continue
		}
		rbfts[index].node.Step(ntfMsgNode2)
	}
	// started replicas receive message
	quorumNode1 := rbfts[0].processEvent(ntfMsgNode3)
	assert.Equal(t, quorumEvent, quorumNode1)
	quorumNode2 := rbfts[1].processEvent(ntfMsgNode3)
	assert.Equal(t, quorumEvent, quorumNode2)

	rbfts[3].atomicOff(Pending)
	rbfts[3].recoveryMgr.syncReceiver.Range(rbfts[3].readMap)
	quorumNode4 := rbfts[3].processEvent(initRecoveryEvent)
	assert.Equal(t, quorumEvent, quorumNode4)

	doneNode1 := rbfts[1].processEvent(quorumEvent)
	assert.Equal(t, doneEvent, doneNode1)
	nvMsg := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, nvMsg.Type)

	for index := range rbfts {
		rbfts[index].processEvent(quorumEvent)
	}

	for index := range rbfts {
		if index == 1 {
			continue
		}
		doneNode := rbfts[index].processEvent(nvMsg)
		assert.Equal(t, doneEvent, doneNode)
	}

	for index := range rbfts {
		rbfts[index].processEvent(doneEvent)
	}

	for index := range rbfts {
		assert.Equal(t, uint64(1), rbfts[index].view)
		assert.True(t, rbfts[index].isNormal())
	}
}
