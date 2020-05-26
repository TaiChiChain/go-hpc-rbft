package rbft

import (
	"errors"
	"github.com/ultramesh/flato-event/inner/protos"
	"testing"
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// A normal process of consensus
func TestRBFT_processEvent_NormalConsensusP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	//******************************************************
	requestsTmp := mockRequestList
	// The first sent
	reqBatchTmp := &pb.RequestBatch{
		BatchHash:       calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
		RequestHashList: []string{"Hash11", "Hash12"},
		RequestList:     mockRequestLists,
		LocalList:       []bool{true, true},
		Timestamp:       1,
		SeqNo:           1,
	}

	// Start:
	// In processEvent function
	// Receive a consensusEvent which Type is RequestSet(isn't local msg)
	// Call processReqSetEvent to process it
	//
	// Req Process:
	// In processReqSetEvent function
	// As for Normal case, the primary submit Reqs to txpool
	// Assuming that txpool has already make three batches, primary receives them
	// Call postBatches to post them
	//
	// Batches to sendPrePrepare:
	// In postBatches function, call recvRequestBatch to deal with the batches
	// In recvRequestBatch function, deal with one batch to prepare prePrepare msg
	// Call maybeSendPrePrepare(here is cacheBatch processing)
	// In maybeSendPrePrepare function,
	// Give batch.seqNo, it is rbft.batchMgr.SeqNo+1
	// rbft.storeMgr stores the batch and storage StoreState it
	// Finally send prePrepare
	// After sending prePrepare, rbft.batchMgr.SeqNo is equal to new batch
	// It means that rbft.batchMgr.SeqNo++
	// As for there are three batches, rbft.batchMgr.SeqNo==3
	e := &pb.RequestSet{
		Requests: requestsTmp,
		Local:    false,
	}
	rbft.on(Normal)
	rbft.peerPool.ID = 1
	assert.Equal(t, uint64(0), rbft.batchMgr.seqNo)
	rbft.processEvent(e)
	assert.Equal(t, reqBatchTmp, rbft.storeMgr.batchStore[reqBatchTmp.BatchHash])
	assert.Equal(t, uint64(3), rbft.batchMgr.seqNo)
	// Now rbft.batchMgr.seqNo=3
	// Other cases:
	// There are batches in BatchCache
	// Firstly process the batch in cache
	// Thought we send 4 batches(1 from cache, 3 from txPool), only the one in cache have new digest
	// So that rbft.batchMgr.seqNo=4
	batch := &pb.RequestBatch{
		BatchHash:       calculateMD5Hash([]string{"HashC1", "HashC2"}, 0),
		RequestHashList: []string{"HashC1", "HashC2"},
		RequestList:     mockRequestLists,
		LocalList:       []bool{true, true},
		Timestamp:       0,
	}
	cacheBatch := []*pb.RequestBatch{batch}
	rbft.batchMgr.cacheBatch = cacheBatch
	// As a consensusMessage to process
	payloadReq, _ := proto.Marshal(e)
	eventReq := &pb.ConsensusMessage{
		Type:    pb.Type_REQUEST_SET,
		Epoch:   uint64(0),
		Payload: payloadReq,
	}
	rbft.processEvent(eventReq)
	assert.Equal(t, uint64(4), rbft.batchMgr.seqNo)
	// Cover processReqSetEvent, postBatches, recvRequestBatch, sendPrePrepare
	// maybeSendPrePrepare in batch_mgr.go
	//******************************************************

	// After primary has send prePrepare
	// Replica node2 recvPrePrepare
	//*******************************************************
	hashBatch := &pb.HashBatch{
		RequestHashList:            []string{"Hash11", "Hash12"},
		DeDuplicateRequestHashList: []string{"Hash11", "Hash12"},
		Timestamp:                  1,
	}
	preprep := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 10,
		BatchDigest:    calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
		HashBatch:      hashBatch,
	}
	payload, _ := proto.Marshal(preprep)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_PRE_PREPARE,
		Epoch:   uint64(0),
		Payload: payload,
	}
	rbft.peerPool.ID = uint64(2)
	// Node2 receive prePrepare, then sendPrepare
	rbft.processEvent(event)
	cert := rbft.storeMgr.getCert(preprep.View, preprep.SequenceNumber, preprep.BatchDigest)
	assert.Equal(t, true, cert.sentPrepare)

	prepNode2 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      rbft.peerPool.ID,
	}
	prepNode1 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      uint64(1),
	}
	prepNode3 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      uint64(3),
	}
	// Node2 has received prepare from itself, then receives quorum prepare
	assert.Equal(t, true, cert.prepare[*prepNode2])
	// Receive from node1
	payload, _ = proto.Marshal(prepNode1)
	event = &pb.ConsensusMessage{
		Type:    pb.Type_PREPARE,
		Epoch:   uint64(0),
		Payload: payload,
	}
	rbft.processEvent(event)
	// Receive from node3
	payload, _ = proto.Marshal(prepNode3)
	event = &pb.ConsensusMessage{
		Type:    pb.Type_PREPARE,
		Epoch:   uint64(0),
		Payload: payload,
	}
	rbft.processEvent(event)
	// When reach quorum Prepare, sendCommit and recvCommit
	// Send commit from rbft.storeMgr.batchStore(needn't recovery)
	assert.Equal(t, true, cert.sentCommit)

	commitNode1 := &pb.Commit{
		ReplicaId:      1,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}
	commitNode3 := &pb.Commit{
		ReplicaId:      3,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}

	// Node2 received commit from itself, after receive quorum commits
	// it could commitPendingBlocks
	payload, _ = proto.Marshal(commitNode1)
	event = &pb.ConsensusMessage{
		Type:    pb.Type_COMMIT,
		Epoch:   uint64(0),
		Payload: payload,
	}
	rbft.processEvent(event)
	rbft.processEvent(event)
	// send repeat commit, cannot be committed
	assert.Equal(t, false, rbft.committed(commitNode1.BatchDigest, commitNode1.View, commitNode1.SequenceNumber))
	// Receive from node3
	payload, _ = proto.Marshal(commitNode3)
	event = &pb.ConsensusMessage{
		Type:    pb.Type_COMMIT,
		Epoch:   uint64(0),
		Payload: payload,
	}

	// assuming that last exec is 9, then execute this req
	// with digest calculateMD5Hash([]string{"Hash11", "Hash12"}, 1)
	// after executed, delete from outstanding req batch
	// cert.executed is true
	stateTmp := &pb.ServiceState{}
	stateTmp.MetaState = &pb.MetaState{
		Applied: 10,
		Digest:  calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
	}

	go func() {
		rbft.cpChan <- stateTmp
		rbft.exec.setLastExec(uint64(9))
		// committed, then make checkpoint and send it
		rbft.processEvent(event)
		d := calculateMD5Hash([]string{"Hash11", "Hash12"}, 1)
		assert.Nil(t, rbft.storeMgr.outstandingReqBatches[d])
		assert.Equal(t, true, cert.sentExecute)
		assert.Nil(t, rbft.exec.currentExec)
		assert.Equal(t, preprep.BatchDigest, rbft.storeMgr.chkpts[preprep.SequenceNumber])
		// As for checkpoint recv, test in TestRBFT_recvCheckpoint1~3
	}()
}

//============================================
// Basic Tools
//============================================

func TestRBFT_newRBFT(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, conf := newTestRBFT(ctrl)

	// Normal case
	structName, nilElems, err := checkNilElems(rbft)
	if err == nil {
		assert.Equal(t, "rbftImpl", structName)
		assert.Nil(t, nilElems)
	}

	cpChan := make(chan *pb.ServiceState)
	confC := make(chan *pb.ReloadFinished)
	// Nil Peers
	conf.Peers = nil
	rbft, err = newRBFT(cpChan, confC, conf)
	assert.Equal(t, errors.New("nil peers"), err)

	// Is a New Node
	conf.Peers = peerSet
	conf.ID = 4
	conf.IsNew = true
	rbft, _ = newRBFT(cpChan, confC, conf)
	assert.Equal(t, 3, rbft.N)
	assert.Equal(t, true, rbft.in(isNewNode))
}

//============================================
// Post Tools
// Test Case: Should receive target Event from rbft.recvChan
//============================================

func TestRBFT_reportStateUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)
	rbft.h = 200

	state2 := &pb.ServiceState{}
	state2.MetaState = &pb.MetaState{
		Applied: 2,
		Digest:  "test",
	}

	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event:     state2,
	}

	rbft.reportStateUpdated(state2)
	obj := <-rbft.recvChan
	assert.Equal(t, event, obj)
}

func TestRBFT_postRequests(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rSet := &pb.RequestSet{
		Requests: mockRequestLists,
		Local:    true,
	}

	rbft, _ := newTestRBFT(ctrl)

	rbft.postRequests(mockRequestLists)
	obj := <-rbft.recvChan

	assert.Equal(t, rSet, obj)
}

func TestRBFT_postMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rbft, _ := newTestRBFT(ctrl)

	go rbft.postMsg([]byte("postMsg"))
	obj := <-rbft.recvChan
	assert.Equal(t, []byte("postMsg"), obj)
}

//============================================
// Get Status
//============================================

func TestRBFT_getStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	var status NodeStatus

	rbft.atomicOn(InViewChange)
	status = rbft.getStatus()
	assert.Equal(t, InViewChange, int(status.Status))
	rbft.atomicOff(InViewChange)

	rbft.atomicOn(InRecovery)
	status = rbft.getStatus()
	assert.Equal(t, InRecovery, int(status.Status))
	rbft.atomicOff(InRecovery)

	rbft.atomicOn(StateTransferring)
	status = rbft.getStatus()
	assert.Equal(t, StateTransferring, int(status.Status))
	rbft.atomicOff(StateTransferring)

	rbft.atomicOn(PoolFull)
	status = rbft.getStatus()
	assert.Equal(t, PoolFull, int(status.Status))
	rbft.atomicOff(PoolFull)

	rbft.atomicOn(Pending)
	status = rbft.getStatus()
	assert.Equal(t, Pending, int(status.Status))
	rbft.atomicOff(Pending)

	rbft.atomicOn(InConfChange)
	status = rbft.getStatus()
	assert.Equal(t, InConfChange, int(status.Status))
	rbft.atomicOff(InConfChange)

	rbft.on(Normal)
	status = rbft.getStatus()
	assert.Equal(t, Normal, int(status.Status))
}

//============================================
// General Event Process Method
//============================================

func TestRBFT_processNullRequset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	msg := &pb.NullRequest{ReplicaId: uint64(1)}

	// If success process it, mode NeedSyncState will on
	rbft.on(Normal)

	rbft.atomicOn(InRecovery)
	rbft.off(NeedSyncState)
	rbft.processNullRequest(msg)
	assert.Equal(t, false, rbft.in(NeedSyncState))
	rbft.atomicOff(InRecovery)

	rbft.atomicOn(InViewChange)
	rbft.off(NeedSyncState)
	rbft.processNullRequest(msg)
	assert.Equal(t, false, rbft.in(NeedSyncState))
	rbft.atomicOff(InViewChange)

	rbft.atomicOn(InEpochSync)
	rbft.off(NeedSyncState)
	rbft.processNullRequest(msg)
	assert.Equal(t, false, rbft.in(NeedSyncState))
	rbft.atomicOff(InEpochSync)

	// not primary
	rbft.setView(uint64(3))
	rbft.processNullRequest(msg)
	assert.Equal(t, false, rbft.in(NeedSyncState))
	rbft.setView(uint64(0))

	// Call trySyncState, set NeedSyncState true
	msg.ReplicaId = uint64(1)
	rbft.processNullRequest(msg)
	assert.Equal(t, true, rbft.in(NeedSyncState))
}

func TestRBFT_handleNullRequestTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.atomicOn(InRecovery)
	rbft.handleNullRequestTimerEvent()
	assert.Equal(t, uint64(0), rbft.view)
	rbft.atomicOff(InRecovery)

	rbft.atomicOn(InViewChange)
	rbft.handleNullRequestTimerEvent()
	assert.Equal(t, uint64(0), rbft.view)
	rbft.atomicOff(InViewChange)

	rbft.atomicOn(InEpochSync)
	rbft.handleNullRequestTimerEvent()
	assert.Equal(t, uint64(0), rbft.view)
	rbft.atomicOff(InEpochSync)

	rbft.setView(uint64(3))
	rbft.handleNullRequestTimerEvent()
	assert.Equal(t, uint64(4), rbft.view)
}

//=============================================================================
// process request set and batch methods
//=============================================================================

func TestRBFT_processReqSetEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	req := &pb.RequestSet{
		Requests: mockRequestLists,
		Local:    false,
	}

	rbft.off(Normal)
	rbft.on(SkipInProgress)
	assert.Nil(t, rbft.processReqSetEvent(req))

	rbft.off(SkipInProgress)
	rbft.atomicOn(PoolFull)
	assert.Nil(t, rbft.processReqSetEvent(req))

	rbft.on(Normal)
	rbft.atomicOff(PoolFull)
	assert.Nil(t, rbft.processReqSetEvent(req))

	rbft.setView(1)
	assert.Nil(t, rbft.processReqSetEvent(req))
}

func TestRBFT_processOutOfDateReqs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// rbft.batchMgr.requestPool.IsPoolFull() mock return false
	// so that if reach that sentence, will off PoolFull state
	rbft.atomicOn(PoolFull)
	rbft.off(Normal)
	rbft.processOutOfDateReqs()
	assert.Equal(t, true, rbft.atomicIn(PoolFull))

	rbft.on(Normal)
	rbft.processOutOfDateReqs()
	assert.Equal(t, false, rbft.atomicIn(PoolFull))
}

//============================================
// Execute Transactions
//============================================

func TestRBFT_commitPendingBlocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// Struct of certTmp which stored in rbft.storeMgr.certStore
	// A test when batch digest is ""
	// Other cases have been test in normal consensus case
	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 15,
		BatchDigest:    "",
		HashBatch:      &pb.HashBatch{Timestamp: 10086},
	}
	prePareTmp1 := &pb.Prepare{
		ReplicaId:      1,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	prePareTmp2 := &pb.Prepare{
		ReplicaId:      2,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	prePareTmp3 := &pb.Prepare{
		ReplicaId:      3,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	commitTmp1 := &pb.Commit{
		ReplicaId:      1,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	commitTmp2 := &pb.Commit{
		ReplicaId:      2,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	commitTmp3 := &pb.Commit{
		ReplicaId:      3,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	msgIDTmp := msgID{
		v: prePrepareTmp.View,
		n: prePrepareTmp.SequenceNumber,
		d: prePrepareTmp.BatchDigest,
	}
	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: false,
		prepare:     map[pb.Prepare]bool{*prePareTmp1: true, *prePareTmp2: true, *prePareTmp3: true},
		sentCommit:  false,
		commit:      map[pb.Commit]bool{*commitTmp1: true, *commitTmp2: true, *commitTmp3: true},
		sentExecute: false,
	}
	rbft.exec.setLastExec(uint64(14))
	rbft.storeMgr.committedCert[msgIDTmp] = ""
	rbft.storeMgr.certStore[msgIDTmp] = certTmp

	// outstandingReqBatch has been deleted for it has already been executed
	// tag in cert is true
	// rbft.exec.currentExec turn to nil
	i := uint64(10)
	rbft.exec.currentExec = &i
	rbft.commitPendingBlocks()
	assert.Nil(t, rbft.storeMgr.outstandingReqBatches[""])
	assert.Equal(t, true, certTmp.sentExecute)
	assert.Nil(t, rbft.exec.currentExec)
}

func TestRBFT_filterExecutableReqs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	var digest string
	var deDuplicateRequestHashes []string

	var executableReqs []*protos.Transaction
	var executableLocalList []bool

	digest = "digest"
	deDuplicateRequestHashes = []string{"de"}

	rbft.storeMgr.batchStore[digest] = &pb.RequestBatch{
		RequestHashList: []string{"request hash list"},
		RequestList:     mockRequestList,
		Timestamp:       time.Now().UnixNano(),
		SeqNo:           2,
		LocalList:       []bool{true},
		BatchHash:       "hash",
	}

	executableReqs, executableLocalList = rbft.filterExecutableTxs(digest, deDuplicateRequestHashes)
	assert.Equal(t, mockRequestList, executableReqs)
	assert.Equal(t, []bool{true}, executableLocalList)
}

func TestRBFT_findNextCommitReq(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	var find bool
	var idx msgID
	var cer *msgCert

	IDTmp := msgID{
		v: 1,
		n: 20,
		d: "msg",
	}

	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      2,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}

	prepareTmp1 := pb.Prepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	prepareTmp2 := pb.Prepare{
		ReplicaId:      3,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	prepareTmp3 := pb.Prepare{
		ReplicaId:      4,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	prepareMapTmp := map[pb.Prepare]bool{
		prepareTmp1: true,
		prepareTmp2: true,
		prepareTmp3: true,
	}

	commitTmp1 := pb.Commit{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	commitTmp2 := pb.Commit{
		ReplicaId:      3,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	commitTmp3 := pb.Commit{
		ReplicaId:      4,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	commitMapTmp := map[pb.Commit]bool{
		commitTmp1: true,
		commitTmp2: true,
		commitTmp3: true,
	}

	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: false,
		prepare:     prepareMapTmp,
		sentCommit:  false,
		commit:      commitMapTmp,
		sentExecute: false,
	}

	rbft.off(SkipInProgress)
	rbft.storeMgr.committedCert[IDTmp] = "committed"
	find, _, _ = rbft.findNextCommitTx()
	assert.Equal(t, false, find)

	certTmp.sentExecute = true
	rbft.storeMgr.certStore[IDTmp] = certTmp
	find, _, _ = rbft.findNextCommitTx()
	assert.Equal(t, false, find)

	certTmp.sentExecute = false
	find, _, _ = rbft.findNextCommitTx()
	assert.Equal(t, false, find)

	rbft.exec.lastExec++
	rbft.on(SkipInProgress)
	find, _, _ = rbft.findNextCommitTx()
	assert.Equal(t, false, find)

	rbft.off(SkipInProgress)
	batch := &pb.RequestBatch{
		RequestHashList: []string{"request hash list"},
		RequestList:     mockRequestList,
		Timestamp:       time.Now().UnixNano(),
		SeqNo:           2,
		LocalList:       []bool{true},
		BatchHash:       "msg",
	}
	rbft.storeMgr.batchStore["msg"] = batch
	find, idx, cer = rbft.findNextCommitTx()
	assert.Equal(t, false, find)
	assert.Equal(t, IDTmp, idx)
	assert.Equal(t, certTmp, cer)
}

//============================================
// Checkpoint issues
//============================================

func TestRBFT_checkpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	state := &pb.ServiceState{}
	state.MetaState = &pb.MetaState{
		Applied: 1,
		Digest:  "checkpoint msg",
	}
	rbft.checkpoint(uint64(10), state)
	assert.Equal(t, state.MetaState.Digest, rbft.storeMgr.chkpts[uint64(10)])
}

func TestRBFT_witnessCheckpointWeakCert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	chkptSelf := &pb.Checkpoint{
		ReplicaId:      2,
		SequenceNumber: 2,
		Digest:         "chkpt msg",
	}
	chkptReplica := &pb.Checkpoint{
		ReplicaId:      1,
		SequenceNumber: 2,
		Digest:         "chkpt msg",
	}
	rbft.storeMgr.checkpointStore[*chkptSelf] = true
	rbft.storeMgr.checkpointStore[*chkptReplica] = true

	rbft.on(SkipInProgress)
	rbft.atomicOff(StateTransferring)
	rbft.witnessCheckpointWeakCert(chkptSelf)
	assert.Equal(t, chkptSelf.Digest, rbft.storeMgr.highStateTarget.targetMessage.digest)
	assert.Equal(t, true, rbft.atomicIn(StateTransferring))
}

func TestRBFT_moveWatermarks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "msg",
		HashBatch:      &pb.HashBatch{Timestamp: 10086},
	}
	prePareTmp := pb.Prepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	commitTmp := pb.Commit{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	msgIDTmp := msgID{
		v: 0,
		n: 2,
		d: "msg",
	}
	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: false,
		prepare:     map[pb.Prepare]bool{prePareTmp: true},
		sentCommit:  false,
		commit:      map[pb.Commit]bool{commitTmp: true},
		sentExecute: false,
	}

	// Delete rbft.storeMgr.certStore[msgIdTmp]
	rbft.storeMgr.certStore[msgIDTmp] = certTmp
	rbft.moveWatermarks(uint64(16))
	assert.Equal(t, (*msgCert)(nil), rbft.storeMgr.certStore[msgIDTmp])

	batchTmp := &pb.RequestBatch{
		RequestHashList: nil,
		RequestList:     nil,
		Timestamp:       10086,
		SeqNo:           0,
		LocalList:       nil,
		BatchHash:       "",
	}

	// Delete rbft.storeMgr.batchStore["msg"]
	rbft.storeMgr.batchStore["msg"] = batchTmp
	rbft.moveWatermarks(uint64(16))
	assert.Equal(t, (*pb.RequestBatch)(nil), rbft.storeMgr.batchStore["msg"])

	// Delete rbft.storeMgr.committedCert[msgIdTmp]
	committedCertTmp := map[msgID]string{msgIDTmp: "msg"}
	rbft.storeMgr.committedCert = committedCertTmp
	rbft.moveWatermarks(uint64(64))
	assert.Equal(t, "", rbft.storeMgr.committedCert[msgIDTmp])

	// Delete rbft.storeMgr.checkpointStore[cp]
	cp := pb.Checkpoint{
		ReplicaId:      1,
		SequenceNumber: 2,
		Digest:         "msg",
	}
	rbft.storeMgr.checkpointStore[cp] = true
	rbft.moveWatermarks(uint64(64))
	assert.Equal(t, false, rbft.storeMgr.checkpointStore[cp])
}

func TestRBFT_updateHighStateTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.storeMgr.highStateTarget = &stateUpdateTarget{
		targetMessage: targetMessage{
			height: uint64(4),
			digest: "target",
		},
		replicas: nil,
	}

	target := &stateUpdateTarget{
		targetMessage: targetMessage{
			height: uint64(3),
			digest: "target",
		},
		replicas: nil,
	}

	rbft.updateHighStateTarget(target)
	assert.Equal(t, uint64(4), rbft.storeMgr.highStateTarget.height)

	target.height = uint64(6)
	rbft.updateHighStateTarget(target)
	assert.Equal(t, uint64(6), rbft.storeMgr.highStateTarget.height)
}

func TestRBFT_tryStateTransfer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	targetMsg := targetMessage{
		height: uint64(5),
		digest: "msg",
	}
	target := &stateUpdateTarget{targetMessage: targetMsg}
	rbft.off(SkipInProgress)
	rbft.atomicOff(StateTransferring)
	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "msg",
		HashBatch:      &pb.HashBatch{Timestamp: 10086},
	}
	prePareTmp := pb.Prepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	commitTmp := pb.Commit{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	msgIDTmp := msgID{
		v: 0,
		n: 2,
		d: "msg",
	}
	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: false,
		prepare:     map[pb.Prepare]bool{prePareTmp: true},
		sentCommit:  false,
		commit:      map[pb.Commit]bool{commitTmp: true},
		sentExecute: false,
	}

	// To The End and clean cert with seqNo>lastExec
	rbft.storeMgr.certStore[msgIDTmp] = certTmp
	rbft.exec.setLastExec(uint64(1))
	rbft.tryStateTransfer(target)
	assert.Equal(t, (*msgCert)(nil), rbft.storeMgr.certStore[msgIDTmp])

	// if rbft.atomicIn(StateTransferring)
	rbft.storeMgr.certStore[msgIDTmp] = certTmp
	rbft.exec.setLastExec(uint64(1))
	rbft.atomicOn(StateTransferring)
	rbft.tryStateTransfer(target)
	assert.Equal(t, certTmp, rbft.storeMgr.certStore[msgIDTmp])

	// if target == nil
	// if rbft.storeMgr.highStateTarget == nil
	rbft.atomicOff(StateTransferring)
	target = nil
	rbft.storeMgr.highStateTarget = nil
	rbft.tryStateTransfer(target)
	assert.Equal(t, certTmp, rbft.storeMgr.certStore[msgIDTmp])
}

func TestRBFT_recvStateUpdatedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// normal
	rbft.h = 20
	ss0 := &pb.ServiceState{
		VSet: nil,
	}
	ss0.MetaState = &pb.MetaState{
		Applied: uint64(30),
		Digest:  "block-number-30",
	}
	rbft.node.currentState = ss0
	ret0 := rbft.recvStateUpdatedEvent(ss0)
	assert.Equal(t, nil, ret0)

	// in recovery
	rbft.h = uint64(20)
	rbft.view = uint64(3)
	ss1 := &pb.ServiceState{
		VSet: nil,
	}
	ss1.MetaState = &pb.MetaState{
		Applied: uint64(30),
		Digest:  "block-number-30",
	}
	rbft.node.currentState = ss1
	rbft.atomicOn(InRecovery)
	ret1 := rbft.recvStateUpdatedEvent(ss1)
	expRet1 := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	rbft.atomicOff(InRecovery)
	assert.Equal(t, expRet1, ret1)

	// state update target < h
	rbft.h = 50
	rbft.storeMgr.highStateTarget = &stateUpdateTarget{
		targetMessage: targetMessage{
			height: uint64(45),
			digest: "block-number-45",
		},
		replicas: nil,
	}
	ss3 := &pb.ServiceState{
		VSet: nil,
	}
	ss3.MetaState = &pb.MetaState{
		Applied: uint64(42),
		Digest:  "block-number-42",
	}
	rbft.atomicOn(StateTransferring)
	rbft.recvStateUpdatedEvent(ss3)
	assert.Equal(t, uint64(42), rbft.exec.lastExec)
	assert.Equal(t, true, rbft.atomicIn(StateTransferring))

	ss4 := &pb.ServiceState{
		VSet: nil,
	}
	ss4.MetaState = &pb.MetaState{
		Applied: uint64(45),
		Digest:  "block-number-45",
	}
	rbft.atomicOn(StateTransferring)
	rbft.recvStateUpdatedEvent(ss4)
	assert.Equal(t, uint64(45), rbft.exec.lastExec)
	assert.Equal(t, false, rbft.atomicIn(StateTransferring))

	// errors
	ss5 := &pb.ServiceState{
		VSet: nil,
	}
	ss5.MetaState = &pb.MetaState{
		Applied: uint64(46),
		Digest:  "block-number-46",
	}
	rbft.atomicOn(StateTransferring)
	rbft.exec.setLastExec(uint64(0))
	rbft.recvStateUpdatedEvent(ss5)
	assert.Equal(t, uint64(0), rbft.exec.lastExec)
	assert.Equal(t, true, rbft.atomicIn(StateTransferring))

	rbft.atomicOn(StateTransferring)
	rbft.storeMgr.highStateTarget = nil
	rbft.exec.setLastExec(uint64(0))
	rbft.recvStateUpdatedEvent(ss5)
	assert.Equal(t, uint64(0), rbft.exec.lastExec)
	assert.Equal(t, true, rbft.atomicIn(StateTransferring))
}

func TestRBFT_recvCheckpoint2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	chkptSeqNo := uint64(10)
	chkptDigest := "msg"
	checkpoint1 := &pb.Checkpoint{
		ReplicaId:      1,
		SequenceNumber: chkptSeqNo,
		Digest:         chkptDigest,
	}
	checkpoint3 := &pb.Checkpoint{
		ReplicaId:      3,
		SequenceNumber: chkptSeqNo,
		Digest:         chkptDigest,
	}
	checkpoint4 := &pb.Checkpoint{
		ReplicaId:      4,
		SequenceNumber: chkptSeqNo,
		Digest:         chkptDigest,
	}

	rbft.recvCheckpoint(checkpoint1)
	rbft.recvCheckpoint(checkpoint3)
	// Node has not reached the chkpt
	// Open skip in progress, could set h to target
	rbft.on(SkipInProgress)
	// Case1: in recovery
	rbft.atomicOn(InRecovery)
	rbft.recvCheckpoint(checkpoint4)
	// move to n
	assert.Equal(t, uint64(10), rbft.h)
}

func TestRBFT_recvCheckpoint3(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	chkptSeqNo := uint64(30)
	chkptDigest := "msg"
	checkpoint1 := &pb.Checkpoint{
		ReplicaId:      1,
		SequenceNumber: chkptSeqNo,
		Digest:         chkptDigest,
	}
	checkpoint3 := &pb.Checkpoint{
		ReplicaId:      3,
		SequenceNumber: chkptSeqNo,
		Digest:         chkptDigest,
	}
	checkpoint4 := &pb.Checkpoint{
		ReplicaId:      4,
		SequenceNumber: chkptSeqNo,
		Digest:         chkptDigest,
	}

	rbft.recvCheckpoint(checkpoint1)
	rbft.recvCheckpoint(checkpoint3)
	// Node has not reached the chkpt
	// Open skip in progress, could set h to target
	rbft.on(SkipInProgress)
	// Case2: not in recovery
	rbft.atomicOff(InRecovery)
	// But just fell behind larger than 20 blocks, update
	rbft.recvCheckpoint(checkpoint4)
	// move to n
	assert.Equal(t, uint64(30), rbft.h)
}

func TestRBFT_weakCheckpointSetOutOfRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	cp := &pb.Checkpoint{
		ReplicaId:      3,
		SequenceNumber: 20,
		Digest:         "",
	}
	var flag bool

	// storage for CheckBiggerNumber
	// Case some value in storage but state is normal, not out of range
	rbft.storeMgr.hChkpts[uint64(3)] = uint64(20)
	flag = rbft.weakCheckpointSetOutOfRange(cp)
	assert.Equal(t, false, flag)
	assert.Equal(t, uint64(0), rbft.storeMgr.hChkpts[uint64(3)])

	// Case: be out of range, but not reach th oneCorrectQuorum
	cp.SequenceNumber = 100
	flag = rbft.weakCheckpointSetOutOfRange(cp)
	assert.Equal(t, false, flag)
	assert.Equal(t, uint64(100), rbft.storeMgr.hChkpts[uint64(3)])

	// Case: f+1 replicas out of range
	// Now current node(node 2) might fallen behind
	// Here, will delete the stored msg that not out of range(node 5)(assuming there is node 5)
	rbft.storeMgr.hChkpts[uint64(1)] = uint64(100)
	rbft.storeMgr.hChkpts[uint64(3)] = uint64(100)
	rbft.storeMgr.hChkpts[uint64(4)] = uint64(100)
	rbft.storeMgr.hChkpts[uint64(5)] = uint64(20)
	flag = rbft.weakCheckpointSetOutOfRange(cp)
	assert.Equal(t, true, flag)
	assert.Equal(t, uint64(0), rbft.storeMgr.hChkpts[uint64(5)])
}
