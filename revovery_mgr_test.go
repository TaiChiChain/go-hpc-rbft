package rbft

import (
	"testing"

	"github.com/hyperchain/go-hpc-common/types/protos"
	pb "github.com/hyperchain/go-hpc-rbft/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRecovery_ClusterInitRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	notificationSet := make([]*pb.ConsensusMessage, 4)
	rbfts[0].initRecovery()
	notificationSet[0] = nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationSet[0].Type)

	rbfts[1].processEvent(notificationSet[0])
	notificationSet[1] = nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationSet[1].Type)

	rbfts[2].processEvent(notificationSet[0])
	notificationSet[2] = nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationSet[2].Type)

	rbfts[3].processEvent(notificationSet[1])
	rbfts[3].processEvent(notificationSet[2])
	notificationSet[3] = nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationSet[3].Type)

	doneNotificationQuorum := &LocalEvent{
		Service:   RecoveryService,
		EventType: NotificationQuorumEvent,
	}
	for index := range rbfts {
		for j := range notificationSet {
			done := rbfts[index].processEvent(notificationSet[j])
			if done != nil {
				assert.Equal(t, doneNotificationQuorum, done)
				break
			}
		}
	}

	for index := range rbfts {
		rbfts[index].processEvent(doneNotificationQuorum)
	}
	nv := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, nv.Type)

	doneRecovery := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	for index := range rbfts {
		if index == 1 {
			continue
		}
		done := rbfts[index].processEvent(nv)
		assert.Equal(t, doneRecovery, done)
	}

	fetchPQCSet := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		rbfts[index].processEvent(doneRecovery)
		fetchPQCSet[index] = nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_RECOVERY_FETCH_QPC, fetchPQCSet[index].Type)
	}
}

func TestRecovery_ReplicaRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	tx1 := newTx()
	executeExceptN(t, rbfts, nodes, tx1, false, 3)

	tx2 := newTx()
	execute(t, rbfts, nodes, tx2, false)

	rbfts[3].initRecovery()
	notificationNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationNode4.Type)
	assert.Equal(t, uint64(1), rbfts[3].view)

	notificationRsp := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(notificationNode4)
		notificationRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, notificationRsp[index].Type)
	}

	var node4FetchPQC *pb.ConsensusMessage
	for index := range notificationRsp {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(notificationRsp[index])
	}

	// replica 3 fetch missing request batch 1
	fetchMissingReqBatch := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_FETCH_REQUEST_BATCH, fetchMissingReqBatch.Type)
	rbfts[0].processEvent(fetchMissingReqBatch)

	// primary 1 return fetch request batch 1
	sendReqBatch := nodes[0].unicastMessageCache
	assert.Equal(t, pb.Type_SEND_REQUEST_BATCH, sendReqBatch.Type)

	// replica 3 receive send request batch and finish recovery.
	recoveryDone := rbfts[3].processEvent(sendReqBatch)

	recoveryDoneTag := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	assert.Equal(t, recoveryDoneTag, recoveryDone)
	rbfts[3].processEvent(recoveryDone)
	node4FetchPQC = nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_RECOVERY_FETCH_QPC, node4FetchPQC.Type)

	returnRecoveryPQC := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(node4FetchPQC)
		returnRecoveryPQC[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_RECOVERY_RETURN_QPC, returnRecoveryPQC[index].Type)
	}

	for index := range returnRecoveryPQC {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(returnRecoveryPQC[index])
	}

	assert.Equal(t, uint64(2), rbfts[3].exec.lastExec)
}

func TestRecovery_PrimaryRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	setClusterViewExcept(rbfts, nodes, uint64(4), 0)
	rbfts[0].setView(uint64(3))
	rbfts[0].initRecovery()
	notificationNode1 := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationNode1.Type)

	notificationRsp := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 0 {
			continue
		}
		rbfts[index].processEvent(notificationNode1)
		notificationRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, notificationRsp[index].Type)
	}

	for index := range notificationRsp {
		if index == 0 {
			continue
		}
		rbfts[0].processEvent(notificationRsp[index])
	}
	vc := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vc.Type)
}

func TestRecovery_NormalCheckpointFailing_Recovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	txSet := make([]*protos.Transaction, 13)
	for index := 0; index < 13; index++ {
		txSet[index] = newTx()
	}
	for index := range txSet {
		if index == 9 {
			break
		}
		execute(t, rbfts, nodes, txSet[index], false)
	}

	executeExceptN(t, rbfts, nodes, txSet[9], true, 3)
	execute(t, rbfts, nodes, txSet[10], false)
	execute(t, rbfts, nodes, txSet[11], false)
	execute(t, rbfts, nodes, txSet[12], false)

	rbfts[3].initRecovery()
	notificationNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationNode4.Type)
	assert.Equal(t, uint64(1), rbfts[3].view)

	notificationRsp := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(notificationNode4)
		notificationRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, notificationRsp[index].Type)
	}

	//var node4FetchPQC *pb.ConsensusMessage
	for index := range notificationRsp {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(notificationRsp[index])
	}

	msg := <-rbfts[3].recvChan
	event := msg.(*LocalEvent)
	ss := event.Event.(*types.ServiceState)
	assert.Equal(t, uint64(10), ss.MetaState.Height)

	recoveryDone := rbfts[3].processEvent(event)
	assert.Equal(t, uint64(10), rbfts[3].h)
	assert.Equal(t, uint64(10), rbfts[3].exec.lastExec)

	recoveryDoneEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	assert.Equal(t, recoveryDoneEvent, recoveryDone)

	rbfts[3].processEvent(recoveryDone)
	node4FetchPQC := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_RECOVERY_FETCH_QPC, node4FetchPQC.Type)

	returnRecoveryPQC := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(node4FetchPQC)
		returnRecoveryPQC[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_RECOVERY_RETURN_QPC, returnRecoveryPQC[index].Type)
	}

	for index := range returnRecoveryPQC {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(returnRecoveryPQC[index])
	}
	assert.Equal(t, uint64(13), rbfts[3].exec.lastExec)
}

func TestRecovery_EpochCheckpointFailing_Recovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	ctx := newCTX(defaultValidatorSet)
	executeExceptN(t, rbfts, nodes, ctx, true, 3)

	tx := newTx()
	executeExceptN(t, rbfts, nodes, tx, false, 3)

	rbfts[3].initRecovery()
	preNotificationNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, preNotificationNode4.Type)
	assert.Equal(t, uint64(1), rbfts[3].view)

	preNotificationRsp := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(preNotificationNode4)
		preNotificationRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, preNotificationRsp[index].Type)
	}

	for index := range preNotificationRsp {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(preNotificationRsp[index])
	}

	ev1 := <-rbfts[3].recvChan
	lv, isLocalEvent := ev1.(*LocalEvent)
	assert.Equal(t, true, isLocalEvent)
	assert.Equal(t, CoreStateUpdatedEvent, lv.EventType)
	rbfts[3].processEvent(ev1)

	notificationNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, notificationNode4.Type)
	assert.Equal(t, uint64(1), rbfts[3].view)

	notificationRsp := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(notificationNode4)
		notificationRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_NOTIFICATION_RESPONSE, notificationRsp[index].Type)
	}

	for index := range notificationRsp {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(notificationRsp[index])
	}

	// replica 3 fetch missing request batch 2
	fetchMissingReqBatch := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_FETCH_REQUEST_BATCH, fetchMissingReqBatch.Type)
	rbfts[0].processEvent(fetchMissingReqBatch)

	// primary 1 return fetch request batch 2
	sendReqBatch := nodes[0].unicastMessageCache
	assert.Equal(t, pb.Type_SEND_REQUEST_BATCH, sendReqBatch.Type)

	// replica 3 receive send request batch and finish recovery.
	recoveryDone := rbfts[3].processEvent(sendReqBatch)

	recoveryDoneTag := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	assert.Equal(t, recoveryDoneTag, recoveryDone)
	rbfts[3].processEvent(recoveryDone)
	node4FetchPQC := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_RECOVERY_FETCH_QPC, node4FetchPQC.Type)

	returnRecoveryPQC := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(node4FetchPQC)
		returnRecoveryPQC[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_RECOVERY_RETURN_QPC, returnRecoveryPQC[index].Type)
	}

	for index := range returnRecoveryPQC {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(returnRecoveryPQC[index])
	}

	assert.Equal(t, uint64(2), rbfts[3].exec.lastExec)
}

func TestRecovery_SyncStateToStateUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	tx := newTx()
	executeExceptN(t, rbfts, nodes, tx, false, 3)
	metaS := &types.MetaState{
		Height: uint64(1),
		Digest: "wrong-digest",
	}
	rbfts[3].node.currentState = &types.ServiceState{
		MetaState: metaS,
	}

	syncEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}
	rbfts[3].processEvent(syncEvent)
	node4SyncStateReq := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_SYNC_STATE, node4SyncStateReq.Type)

	syncStateResponse := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		rbfts[index].processEvent(node4SyncStateReq)
		syncStateResponse[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_SYNC_STATE_RESPONSE, syncStateResponse[index].Type)
	}

	for index := range syncStateResponse {
		rbfts[3].processEvent(syncStateResponse[index])
	}

	msg := <-rbfts[3].recvChan
	event := msg.(*LocalEvent)
	ss := event.Event.(*types.ServiceState)
	assert.Equal(t, uint64(1), ss.MetaState.Height)

	assert.Equal(t, uint64(0), rbfts[3].exec.lastExec)
	rbfts[3].processEvent(msg)
	assert.Equal(t, uint64(1), rbfts[3].exec.lastExec)
}

func TestRecovery_SyncStateToSyncEpoch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	ctx := newCTX(defaultValidatorSet)
	executeExceptN(t, rbfts, nodes, ctx, true, 3)
	metaS := &types.MetaState{
		Height: uint64(1),
		Digest: "wrong-digest",
	}
	rbfts[3].node.currentState = &types.ServiceState{
		MetaState: metaS,
	}

	syncEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}

	rbfts[3].processEvent(syncEvent)
	node4SyncStateReq := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_SYNC_STATE, node4SyncStateReq.Type)

	syncStateResponse := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		rbfts[index].processEvent(node4SyncStateReq)
		syncStateResponse[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_SYNC_STATE_RESPONSE, syncStateResponse[index].Type)
	}

	for index := range syncStateResponse {
		rbfts[3].processEvent(syncStateResponse[index])
	}

	msg := <-rbfts[3].recvChan
	event := msg.(*LocalEvent)
	ss := event.Event.(*types.ServiceState)
	assert.Equal(t, uint64(1), ss.MetaState.Height)

	assert.Equal(t, uint64(0), rbfts[3].exec.lastExec)
	rbfts[3].processEvent(msg)
	assert.Equal(t, uint64(1), rbfts[3].exec.lastExec)
}

func TestRecovery_ReplicaSyncStateToRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	tx := newTx()
	executeExceptN(t, rbfts, nodes, tx, false, 3)

	syncEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}

	rbfts[3].processEvent(syncEvent)
	node4SyncStateReq := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_SYNC_STATE, node4SyncStateReq.Type)

	syncStateResponse := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		rbfts[index].processEvent(node4SyncStateReq)
		syncStateResponse[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_SYNC_STATE_RESPONSE, syncStateResponse[index].Type)
	}

	for index := range syncStateResponse {
		rbfts[3].processEvent(syncStateResponse[index])
	}

	node4Notification := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_NOTIFICATION, node4Notification.Type)
}

func TestRecovery_PrimarySyncStateToRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	tx := newTx()
	executeExceptPrimary(t, rbfts, nodes, tx, false)

	syncEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}

	rbfts[0].processEvent(syncEvent)
	node4SyncStateReq := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_SYNC_STATE, node4SyncStateReq.Type)

	syncStateResponse := make([]*pb.ConsensusMessage, 4)
	for index := range rbfts {
		rbfts[index].processEvent(node4SyncStateReq)
		syncStateResponse[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_SYNC_STATE_RESPONSE, syncStateResponse[index].Type)
	}

	for index := range syncStateResponse {
		rbfts[0].processEvent(syncStateResponse[index])
	}

	node4Notification := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, node4Notification.Type)
}
