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
		Event:     uint64(0),
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

func TestCluster_ViewChange_StateUpdate_Timeout_StateUpdated_Replica(t *testing.T) {
	// test sample 1 for flato-3235
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	for i := 0; i < 40; i++ {
		tx := newTx()
		checkpoint := (i+1)%10 == 0
		if i < 30 {
			execute(t, rbfts, nodes, tx, checkpoint)
		} else {
			executeExceptN(t, rbfts, nodes, tx, checkpoint, 2)
		}
	}
	assert.Equal(t, uint64(40), rbfts[0].h)
	assert.Equal(t, uint64(40), rbfts[1].h)
	assert.Equal(t, uint64(30), rbfts[2].h)
	assert.Equal(t, uint64(40), rbfts[3].h)

	var vcMsgs []*pb.ConsensusMessage
	for index := range rbfts {
		rbfts[index].sendViewChange()
		vcMsg := nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_VIEW_CHANGE, vcMsg.Type)
		vcMsgs = append(vcMsgs, vcMsg)
	}

	vcQuorum := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}

	for index := range rbfts {
		for j := range vcMsgs {
			if index == j {
				continue
			}
			qe := rbfts[index].processEvent(vcMsgs[j])
			if qe != nil {
				assert.Equal(t, vcQuorum, qe)
				break
			}
		}
	}

	for index := range rbfts {
		rbfts[index].processEvent(vcQuorum)
	}
	nvMsg := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, nvMsg.Type)

	vcDone := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangedEvent,
	}
	for index := range rbfts {
		if index == 1 {
			continue
		}
		vd := rbfts[index].processEvent(nvMsg)
		if index != 2 {
			assert.Equal(t, vcDone, vd)
		} else {
			assert.Nil(t, vd)
		}
	}
	msg := <-rbfts[2].recvChan
	event := msg.(*LocalEvent)
	vd := rbfts[2].processEvent(event)
	assert.Equal(t, vcDone, vd)

	for index := range rbfts {
		rbfts[index].processEvent(vd)
	}
	assert.True(t, rbfts[0].isNormal())
	assert.True(t, rbfts[1].isNormal())
	assert.True(t, rbfts[2].isNormal())
	assert.True(t, rbfts[3].isNormal())
}

func TestCluster_ViewChange_StateUpdate_Timeout_StateUpdated_Primary(t *testing.T) {
	// test sample 2 for flato-3235
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	for i := 0; i < 40; i++ {
		tx := newTx()
		checkpoint := (i+1)%10 == 0
		if i < 30 {
			execute(t, rbfts, nodes, tx, checkpoint)
		} else {
			executeExceptN(t, rbfts, nodes, tx, checkpoint, 1)
		}
	}
	assert.Equal(t, uint64(40), rbfts[0].h)
	assert.Equal(t, uint64(30), rbfts[1].h)
	assert.Equal(t, uint64(40), rbfts[2].h)
	assert.Equal(t, uint64(40), rbfts[3].h)

	var vcMsgs []*pb.ConsensusMessage
	for index := range rbfts {
		rbfts[index].sendViewChange()
		vcMsg := nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_VIEW_CHANGE, vcMsg.Type)
		vcMsgs = append(vcMsgs, vcMsg)
	}

	vcQuorum := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}

	for index := range rbfts {
		for j := range vcMsgs {
			if index == j {
				continue
			}
			qe := rbfts[index].processEvent(vcMsgs[j])
			if qe != nil {
				assert.Equal(t, vcQuorum, qe)
				break
			}
		}
	}

	for index := range rbfts {
		nodes[index].broadcastMessageCache = nil
		rbfts[index].processEvent(vcQuorum)
	}
	assert.Nil(t, nodes[1].broadcastMessageCache)

	nvTimeoutEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeTimerEvent,
		Event:     nextDemandNewView(1),
	}
	// node 3 crash
	rbfts[0].processEvent(nvTimeoutEvent)
	newVCNode1 := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, newVCNode1.Type)

	rbfts[2].processEvent(nvTimeoutEvent)
	newVCNode3 := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, newVCNode3.Type)

	rbfts[0].processEvent(newVCNode3)
	rbfts[2].processEvent(newVCNode1)

	nodes[1].broadcastMessageCache = nil
	rbfts[1].processEvent(newVCNode1)
	rbfts[1].processEvent(newVCNode3)
	assert.Nil(t, nodes[1].broadcastMessageCache)

	msg := <-rbfts[1].recvChan
	event := msg.(*LocalEvent)
	retNode2 := rbfts[1].processEvent(event)
	assert.Equal(t, vcQuorum, retNode2)
	newVCNode2 := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, newVCNode2.Type)

	retNode1 := rbfts[0].processEvent(newVCNode2)
	assert.Equal(t, vcQuorum, retNode1)
	retNode3 := rbfts[2].processEvent(newVCNode2)
	assert.Equal(t, vcQuorum, retNode3)

	vcDone := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangedEvent,
	}
	for index := range rbfts {
		if index == 3 {
			// node 3 crash
			continue
		}
		done := rbfts[index].processEvent(vcQuorum)
		if index == 2 {
			assert.Equal(t, vcDone, done)
		}
	}
	newNVMsg := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, newNVMsg.Type)

	for index := range rbfts {
		if index == 2 || index == 3 {
			continue
		}
		done := rbfts[index].processEvent(newNVMsg)
		assert.Equal(t, vcDone, done)
	}

	for index := range rbfts {
		if index == 3 {
			continue
			// node 3 crash
		}
		rbfts[index].processEvent(vcDone)
	}

	assert.True(t, rbfts[0].isNormal())
	assert.True(t, rbfts[1].isNormal())
	assert.True(t, rbfts[2].isNormal())
}

func TestCluster_Checkpoint_in_StateUpdating(t *testing.T) {
	// flato-3711
	// test for update high target while transferring for efficient state-update instance initiation.

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	var retMessageSet []map[pb.Type][]*pb.ConsensusMessage
	for i := 0; i < 50; i++ {
		tx := newTx()
		checkpoint := (i+1)%10 == 0
		retMessages := executeExceptN(t, rbfts, nodes, tx, checkpoint, 1)
		retMessageSet = append(retMessageSet, retMessages)
	}

	for _, retMessages := range retMessageSet {
		for index, chkpt := range retMessages[pb.Type_CHECKPOINT] {
			if chkpt == nil {
				continue
			}
			if index == 1 {
				continue
			}
			rbfts[1].processEvent(chkpt)
		}
	}

	notificationMsg := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationMsg.Type)

	for index := range rbfts {
		if index == 1 {
			continue
		}
		rbfts[index].processEvent(notificationMsg)
		rsp := nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, rsp.Type)
		rbfts[1].processEvent(rsp)
	}

	var retMessageSet2 []map[pb.Type][]*pb.ConsensusMessage
	for i := 0; i < 10; i++ {
		tx := newTx()
		checkpoint := (i+1)%10 == 0
		retMessages := executeExceptN(t, rbfts, nodes, tx, checkpoint, 1)
		retMessageSet2 = append(retMessageSet2, retMessages)
	}

	for _, retMessages := range retMessageSet2 {
		for index, chkpt := range retMessages[pb.Type_CHECKPOINT] {
			if chkpt == nil {
				continue
			}
			if index == 1 {
				continue
			}
			rbfts[1].processEvent(chkpt)
		}
	}

	updatedEv1 := <-rbfts[1].recvChan
	assert.Equal(t, CoreStateUpdatedEvent, updatedEv1.(*LocalEvent).EventType)

	rbfts[1].processEvent(updatedEv1)

	var retMessageSet3 []map[pb.Type][]*pb.ConsensusMessage
	for i := 0; i < 10; i++ {
		tx := newTx()
		checkpoint := (i+1)%10 == 0
		retMessages := executeExceptN(t, rbfts, nodes, tx, checkpoint, 1)
		retMessageSet3 = append(retMessageSet3, retMessages)
	}

	for _, retMessages := range retMessageSet3 {
		for index, chkpt := range retMessages[pb.Type_CHECKPOINT] {
			if chkpt == nil {
				continue
			}
			if index == 1 {
				continue
			}
			rbfts[1].processEvent(chkpt)
		}
	}

	updatedEv2 := <-rbfts[1].recvChan
	assert.Equal(t, CoreStateUpdatedEvent, updatedEv2.(*LocalEvent).EventType)
	rbfts[1].processEvent(updatedEv2)

	updatedEv3 := <-rbfts[1].recvChan
	assert.Equal(t, CoreStateUpdatedEvent, updatedEv3.(*LocalEvent).EventType)
	rbfts[1].processEvent(updatedEv3)
	assert.Equal(t, uint64(70), rbfts[1].h)
}

func TestCluster_InitRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	init := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryInitEvent,
		Event:     uint64(0),
	}

	// replica 2&3 process init recovery directly and broadcast notification in view=1
	rbfts[1].processEvent(init)
	rbfts[2].processEvent(init)
	notifications := make([]*pb.ConsensusMessage, len(nodes))
	notifications[1] = nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notifications[1].Type)
	notifications[2] = nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notifications[2].Type)

	// replica 1 received notifications from 2&3 and generate a notification in view=1
	rbfts[0].processEvent(notifications[1])
	quorum0 := rbfts[0].processEvent(notifications[2])
	assert.Equal(t, NotificationQuorumEvent, quorum0.(*LocalEvent).EventType)
	notifications[0] = nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notifications[0].Type)

	// delayed timeout for init recovery event on replica 1
	// we should reject it
	assert.Equal(t, uint64(1), rbfts[0].view)
	rbfts[0].processEvent(init)
	assert.Equal(t, uint64(1), rbfts[0].view)

	// replica 4 received notifications from 2&3 and generate a notification in view=1
	rbfts[3].processEvent(notifications[1])
	quorum3 := rbfts[3].processEvent(notifications[2])
	assert.Equal(t, NotificationQuorumEvent, quorum3.(*LocalEvent).EventType)
	notifications[3] = nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notifications[3].Type)

	// delayed timeout for init recovery event on replica 4
	// we should reject it
	assert.Equal(t, uint64(1), rbfts[3].view)
	rbfts[0].processEvent(init)
	assert.Equal(t, uint64(1), rbfts[3].view)

	// replica 1 receives notifications
	rbfts[0].processEvent(notifications[3])

	// replica 2 receives notifications
	rbfts[1].processEvent(notifications[0])
	quorum1 := rbfts[1].processEvent(notifications[2])
	assert.Equal(t, NotificationQuorumEvent, quorum1.(*LocalEvent).EventType)
	rbfts[1].processEvent(notifications[3])

	// replica 3 receives notifications
	rbfts[2].processEvent(notifications[0])
	quorum2 := rbfts[2].processEvent(notifications[1])
	assert.Equal(t, NotificationQuorumEvent, quorum2.(*LocalEvent).EventType)
	rbfts[2].processEvent(notifications[3])

	// replica 4 receives notifications
	rbfts[3].processEvent(notifications[0])

	// notification quorum event
	quorumNotification := &LocalEvent{Service: RecoveryService, EventType: NotificationQuorumEvent}
	for index := range rbfts {
		finished := rbfts[index].processEvent(quorumNotification)

		if index == 1 {
			assert.Equal(t, RecoveryDoneEvent, finished.(*LocalEvent).EventType)
		}
	}

	// new view message
	newView := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, newView.Type)

	for index := range rbfts {
		if index == 1 {
			continue
		}

		finished := rbfts[index].processEvent(newView)
		assert.Equal(t, RecoveryDoneEvent, finished.(*LocalEvent).EventType)
	}

	// recovery done event
	done := &LocalEvent{Service: RecoveryService, EventType: RecoveryDoneEvent}
	for index := range rbfts {
		rbfts[index].processEvent(done)
	}

	// check status
	for index := range rbfts {
		assert.Equal(t, uint64(1), rbfts[index].view)
		assert.True(t, rbfts[index].isNormal())
	}
}
