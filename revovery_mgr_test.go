package rbft

import (
	"testing"

	"github.com/hyperchain/go-hpc-common/types/protos"
	pb "github.com/hyperchain/go-hpc-rbft/v2/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/v2/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRecovery_ClusterInitRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)
	clusterInitRecovery(t, nodes, rbfts, -1)

	for index := range rbfts {
		assert.EqualValues(t, 1, rbfts[index].view)
	}
}

func TestRecovery_ReplicaSingleRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)
	clusterInitRecovery(t, nodes, rbfts, 3)

	tx1 := newTx()
	executeExceptN(t, rbfts, nodes, tx1, false, 3)

	tx2 := newTx()
	executeExceptN(t, rbfts, nodes, tx2, false, 3)

	// node4 single recovery.
	rbfts[3].sendViewChange(true)
	vcNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode4.Type)
	assert.Equal(t, uint64(1), rbfts[3].view)

	// normal nodes response.
	fetchViewRsp := make([]*consensusMessageWrapper, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(vcNode4)
		fetchViewRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_RECOVERY_RESPONSE, fetchViewRsp[index].Type)
	}

	for index := range fetchViewRsp {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(fetchViewRsp[index])
	}

	// node4 finish recovery.
	var node4FetchPQC *consensusMessageWrapper
	vcDoneEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeDoneEvent,
	}
	rbfts[3].processEvent(vcDoneEvent)
	node4FetchPQC = nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_FETCH_PQC_REQUEST, node4FetchPQC.Type)

	fetchPQCRsp := make([]*consensusMessageWrapper, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(node4FetchPQC)
		fetchPQCRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_FETCH_PQC_RESPONSE, fetchPQCRsp[index].Type)
	}

	for index := range fetchPQCRsp {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(fetchPQCRsp[index])
	}

	assert.Equal(t, uint64(2), rbfts[3].exec.lastExec)
}

func TestRecovery_Disconnect_ClusterRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// mock primary node2 crash, null request timeout...

	// node1 try vc.
	rbfts[0].sendViewChange()
	vcNode1 := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode1.Type)
	assert.Equal(t, uint64(2), rbfts[0].view)

	// node3 try vc.
	rbfts[2].sendViewChange()
	vcNode3 := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode3.Type)
	assert.Equal(t, uint64(2), rbfts[2].view)

	// node4 try vc.
	rbfts[3].sendViewChange()
	vcNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode4.Type)
	assert.Equal(t, uint64(2), rbfts[3].view)

	// node1 received vc from node3, node4, trigger vc quorum
	rbfts[0].processEvent(vcNode4)
	vcQuorumNode1 := rbfts[0].processEvent(vcNode3)
	assert.Equal(t, ViewChangeQuorumEvent, vcQuorumNode1.(*LocalEvent).EventType)

	// node3 received vc from node1
	rbfts[2].processEvent(vcNode1)

	// node4 received vc from node1
	rbfts[3].processEvent(vcNode1)
	// node3 disconnect node4......so, node3 and node4 cannot reach view change quorum

	// node1 cannot receive NewView from new primary node3, NewViewTimer expired,
	// advance view and retry vc
	event := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeTimerEvent,
		Event:     nextDemandNewView(rbfts[0].view),
	}
	rbfts[0].processEvent(event)
	vcNode1 = nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode1.Type)
	assert.Equal(t, uint64(3), rbfts[0].view)

	// node3 received vc from node1 with higher view, try fetch view.
	rbfts[2].processEvent(vcNode1)
	fetchViewNode3 := nodes[2].unicastMessageCache
	assert.Equal(t, pb.Type_FETCH_VIEW, fetchViewNode3.Type)
	assert.Equal(t, uint64(2), rbfts[2].view)

	// node4 received vc from node1 with higher view, try fetch view.
	rbfts[3].processEvent(vcNode1)
	fetchViewNode4 := nodes[3].unicastMessageCache
	assert.Equal(t, pb.Type_FETCH_VIEW, fetchViewNode4.Type)
	assert.Equal(t, uint64(2), rbfts[3].view)

	// node1 received fetch view from node3 and node4, response QuorumViewChange
	rbfts[0].processEvent(fetchViewNode3)
	fetchViewNode3Response := nodes[0].unicastMessageCache
	rbfts[0].processEvent(fetchViewNode4)
	fetchViewNode4Response := nodes[0].unicastMessageCache

	// node3 process QuorumViewChange and find itself become new primary.
	rbfts[2].processEvent(fetchViewNode3Response)
	newView3 := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, newView3.Type)
	assert.Equal(t, uint64(2), rbfts[2].view)
	assert.Equal(t, true, rbfts[2].isNormal())

	// node4 process QuorumViewChange and enter view change quorum status.
	rbfts[3].processEvent(fetchViewNode4Response)

	// node4 re-connect to node3 and process new view
	vcDoneNode4 := rbfts[3].processEvent(newView3)
	vcDoneEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeDoneEvent,
	}
	assert.Equal(t, vcDoneNode4.(*LocalEvent).EventType, vcDoneEvent.EventType)
	rbfts[3].processEvent(vcDoneEvent)
	assert.Equal(t, uint64(2), rbfts[3].view)
	assert.Equal(t, true, rbfts[3].isNormal())
}

func TestRecovery_PrimaryRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)
	clusterInitRecovery(t, nodes, rbfts, -1)

	primaryIndex := 1
	// primary send recovery view change.
	rbfts[primaryIndex].sendViewChange(true)
	primaryVC := nodes[primaryIndex].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, primaryVC.Type)

	// all nodes trigger view change because of primary recovery.
	vcRsp := make([]*consensusMessageWrapper, 4)
	for index := range rbfts {
		if index == primaryIndex {
			continue
		}
		rbfts[index].processEvent(primaryVC)
		vcRsp[index] = nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_VIEW_CHANGE, vcRsp[index].Type)
	}
}

func TestRecovery_NormalCheckpointFailing_Recovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)
	clusterInitRecovery(t, nodes, rbfts, -1)

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

	// mock node4 recovery view change.
	rbfts[3].sendViewChange(true)
	vcNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode4.Type)
	assert.Equal(t, uint64(2), rbfts[3].view)

	// normal nodes response.
	fetchViewRsp := make([]*consensusMessageWrapper, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(vcNode4)
		fetchViewRsp[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_RECOVERY_RESPONSE, fetchViewRsp[index].Type)
	}

	// node4 process responses and trigger sync chain to initial checkpoint height 10.
	for index := range fetchViewRsp {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(fetchViewRsp[index])
	}

	// node4 finish sync chain to height 10.
	msg := <-rbfts[3].recvChan
	event := msg.(*LocalEvent)
	ss := event.Event.(*types.ServiceState)
	assert.Equal(t, uint64(10), ss.MetaState.Height)

	// node4 finish recovery.
	recoveryDone := rbfts[3].processEvent(event)
	assert.Equal(t, uint64(10), rbfts[3].h)
	assert.Equal(t, uint64(10), rbfts[3].exec.lastExec)

	recoveryVCDoneEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeDoneEvent,
	}
	assert.Equal(t, recoveryVCDoneEvent, recoveryDone)

	// node4 fetch PQC.
	rbfts[3].processEvent(recoveryDone)
	node4FetchPQC := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_FETCH_PQC_REQUEST, node4FetchPQC.Type)

	// normal nodes response PQC.
	returnRecoveryPQC := make([]*consensusMessageWrapper, 4)
	for index := range rbfts {
		if index == 3 {
			continue
		}
		rbfts[index].processEvent(node4FetchPQC)
		returnRecoveryPQC[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_FETCH_PQC_RESPONSE, returnRecoveryPQC[index].Type)
	}

	// node4 process PQC and catch up to the latest height.
	for index := range returnRecoveryPQC {
		if index == 3 {
			continue
		}
		rbfts[3].processEvent(returnRecoveryPQC[index])
	}
	assert.Equal(t, uint64(13), rbfts[3].exec.lastExec)
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

	syncStateResponse := make([]*consensusMessageWrapper, 4)
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

	syncStateResponse := make([]*consensusMessageWrapper, 4)
	for index := range rbfts {
		rbfts[index].processEvent(node4SyncStateReq)
		syncStateResponse[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_SYNC_STATE_RESPONSE, syncStateResponse[index].Type)
	}

	for index := range syncStateResponse {
		rbfts[3].processEvent(syncStateResponse[index])
	}

	vcNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode4.Type)
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

	syncStateResponse := make([]*consensusMessageWrapper, 4)
	for index := range rbfts {
		rbfts[index].processEvent(node4SyncStateReq)
		syncStateResponse[index] = nodes[index].unicastMessageCache
		assert.Equal(t, pb.Type_SYNC_STATE_RESPONSE, syncStateResponse[index].Type)
	}

	for index := range syncStateResponse {
		rbfts[0].processEvent(syncStateResponse[index])
	}

	vcNode1 := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode1.Type)
}
