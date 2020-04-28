// Copyright 2016-2017 Hyperchain Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rbft

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ultramesh/flato-event/inner/protos"
	"github.com/ultramesh/flato-rbft/external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpool "github.com/ultramesh/flato-txpool"

	"github.com/gogo/protobuf/proto"
)

// Config contains the parameters to start a RAFT instance.
type Config struct {
	// ID is the num-identity of the local rbft.peerPool.localIDde. ID cannot be 0.
	ID uint64

	// Hash is the true-identity of the local rbft.peerPool.localIDde. Hash cannot be null
	Hash string

	// Epoch is the epoch of such node
	Epoch uint64

	// peers contains the IDs and hashes of all nodes (including self) in the RBFT cluster. It
	// should be set when starting/restarting a RBFT cluster or after application layer has
	// finished add/delete node.
	// It's application's responsibility to ensure a consistent peer list among cluster.
	Peers []*pb.Peer

	// IsNew indicates if current node is a new joint node or not.
	IsNew bool

	// Applied is last applied index of application service which should be assigned when node
	// restart.
	Applied uint64

	// K decides how long this checkpoint period is.
	K uint64

	// LogMultiplier is used to calculate max log size in memory: k*logMultiplier.
	LogMultiplier uint64

	// SetSize is the max size of a request set to broadcast among cluster.
	SetSize int

	// SetTimeout is the max time duration before one generating a request set.
	SetTimeout time.Duration

	// BatchTimeout is the max time duration before primary generating a request batch.
	BatchTimeout time.Duration

	// RequestTimeout is the max time duration before one reach the consensus on one batch.
	RequestTimeout time.Duration

	// NullRequestTimeout is the time duration one waits for primary's null request before
	// send viewChange.
	NullRequestTimeout time.Duration

	// VCPeriod is the period between automatic viewChanges.
	// Default value is 0 means close automatic viewChange.
	VCPeriod uint64

	// VcResendTimeout is the time duration one wait for a viewChange quorum before resending
	// the same viewChange.
	VcResendTimeout time.Duration

	// CleanVCTimeout is the time duration ro clear out-of-date viewChange messages.
	CleanVCTimeout time.Duration

	// NewViewTimeout is the time duration one waits for newView messages in viewChange.
	NewViewTimeout time.Duration

	// FirstRequestTimeout is the time duration backup nodes wait for the first request from
	// primary before send viewChange.
	FirstRequestTimeout time.Duration

	// SyncStateTimeout is the time duration one wait for same syncStateResponse.
	SyncStateTimeout time.Duration

	// SyncStateRestartTimeout is the time duration one resend syncState request after last
	// successful sync.
	SyncStateRestartTimeout time.Duration

	// RecoveryTimeout is the time duration one wait for a notification quorum before resending
	// the same notification.
	RecoveryTimeout time.Duration

	// UpdateTimeout is the time duration one wait for a add/delete event finished.
	UpdateTimeout time.Duration

	// CheckPoolTimeout is the time duration one check for out-of-date requests in request pool cyclically.
	CheckPoolTimeout time.Duration

	// External is the application helper interfaces which must be implemented by application before
	// initializing RBFT service.
	External external.ExternalStack

	// RequestPool helps managing the requests, detect and discover duplicate requests.
	RequestPool txpool.TxPool

	// Logger is the logger used to record logger in RBFT.
	Logger Logger
}

// rbftImpl is the core struct of RBFT service, which handles all functions about consensus.
type rbftImpl struct {
	node *node

	f             int    // max number of byzantine nodes we can tolerate
	N             int    // max number of consensus nodes in the network
	h             uint64 // low watermark
	K             uint64 // how long this checkpoint period is
	logMultiplier uint64 // use this value to calculate log size : k*logMultiplier
	L             uint64 // log size: k*logMultiplier
	view          uint64 // current view
	epoch         uint64 // track node's epoch number

	status      *statusManager   // keep all basic status of rbft in this object
	timerMgr    *timerManager    // manage rbft event timers
	exec        *executor        // manage transaction execution
	storeMgr    *storeManager    // manage memory log storage
	batchMgr    *batchManager    // manage request batch related issues
	recoveryMgr *recoveryManager // manage recovery issues
	vcMgr       *vcManager       // manage viewchange issues
	peerPool    *peerPool        // manage node status including route table, the connected peers and so on
	epochMgr    *epochManager    // manage epoch issues
	storage     external.Storage // manage non-volatile storage of consensus log

	external external.ExternalStack // manage interaction with application layer

	recvChan chan interface{}      // channel to receive ordered consensus messages and local events
	cpChan   chan *pb.ServiceState // channel to wait for local checkpoint event
	confChan chan bool             // channel to track config transaction execute
	close    chan bool             // channel to close this event process

	viewLock  sync.RWMutex // mutex to set value of view
	hLock     sync.RWMutex // mutex to set value of h
	epochLock sync.RWMutex // mutex to set value of view

	config Config // get configuration info
	logger Logger // write logger to record some info
}

// newRBFT init the RBFT instance
func newRBFT(cpChan chan *pb.ServiceState, confC chan bool, c Config) (*rbftImpl, error) {
	recvC := make(chan interface{})
	rbft := &rbftImpl{
		epoch:    c.Epoch,
		config:   c,
		logger:   c.Logger,
		external: c.External,
		storage:  c.External,
		recvChan: recvC,
		cpChan:   cpChan,
		confChan: confC,
		close:    make(chan bool),
	}

	N := len(c.Peers)
	// new node turn on isNewNode flag to start add node process after recovery.
	if c.IsNew {
		// new node excludes itself when start node until finish updateN.
		N--
	}

	rbft.N = N
	rbft.f = (rbft.N - 1) / 3
	rbft.K = c.K
	rbft.logMultiplier = c.LogMultiplier
	rbft.L = rbft.logMultiplier * rbft.K

	if c.Peers == nil {
		return nil, fmt.Errorf("nil peers")
	}

	// genesis node must have itself included in router.
	if !c.IsNew {
		found := false
		for _, p := range c.Peers {
			if p.Hash == c.Hash {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("peers: %+v doesn't contain self ID: %s", c.Peers, c.Hash)
		}
	}

	// new timer manager
	rbft.timerMgr = newTimerMgr(rbft.recvChan, c)
	rbft.initTimers()

	// new status manager
	rbft.status = newStatusMgr()
	rbft.initStatus()
	// new node turn on isNewNode flag to start add node process after recovery.
	if c.IsNew {
		rbft.logger.Notice("Replica starts as a new node")
		rbft.on(isNewNode)
	}

	// new executor
	rbft.exec = newExecutor()

	// new store manager
	rbft.storeMgr = newStoreMgr(c)

	// new batch manager
	rbft.batchMgr = newBatchManager(rbft.recvChan, c)

	// new recovery manager
	rbft.recoveryMgr = newRecoveryMgr(c)

	// new viewChange manager
	rbft.vcMgr = newVcManager(c)

	// new epoch manager
	rbft.epochMgr = newEpochManager(c)

	// new peer pool
	rbft.peerPool = newPeerPool(c)

	// restore state from consensus database
	// TODO(move to node interface?)
	rbft.exec.setLastExec(c.Applied)

	// update viewChange seqNo after restore state which may update seqNo
	rbft.updateViewChangeSeqNo(rbft.exec.lastExec, rbft.K)

	// init message event converter
	rbft.initMsgEventMap()

	rbft.logger.Infof("RBFT Max number of validating peers (N) = %v", rbft.N)
	rbft.logger.Infof("RBFT Max number of failing peers (f) = %v", rbft.f)
	rbft.logger.Infof("RBFT byzantine flag = %v", rbft.in(byzantine))
	rbft.logger.Infof("RBFT Checkpoint period (K) = %v", rbft.K)
	rbft.logger.Infof("RBFT Log multiplier = %v", rbft.logMultiplier)
	rbft.logger.Infof("RBFT log size (L) = %v", rbft.L)
	rbft.logger.Infof("RBFT localID: %d", rbft.peerPool.localID)

	return rbft, nil
}

// start initializes and starts the consensus service
func (rbft *rbftImpl) start() error {
	// exit pending status after start rbft to avoid missing consensus messages from other nodes.
	rbft.off(Pending)

	rbft.logger.Noticef("--------RBFT starting, nodeID: %d--------", rbft.peerPool.localID)

	if err := rbft.restoreState(); err != nil {
		rbft.logger.Errorf("Replica restore state failed: %s", err)
		return err
	}

	// start listen consensus event
	go rbft.listenEvent()

	// NOTE!!! must use goroutine to post the event
	// to avoid blocking the rbft service.
	if !rbft.in(isNewNode) {
		initRecoveryEvent := &LocalEvent{
			Service:   RecoveryService,
			EventType: RecoveryInitEvent,
		}
		go rbft.postMsg(initRecoveryEvent)
	} else {
		// if it's a new node, lock the consensus and start epoch sync
		rbft.tryEpochSync()
	}

	return nil
}

// stop stops the consensus service
func (rbft *rbftImpl) stop() {
	rbft.logger.Notice("RBFT stopping...")

	// stop listen consensus event
	if rbft.close != nil {
		close(rbft.close)
		rbft.close = nil
	}

	// stop all timer event
	rbft.timerMgr.Stop()

	rbft.logger.Noticef("======== RBFT stopped!")
}

// RecvMsg receives and processes messages from other peers
func (rbft *rbftImpl) step(msg *pb.ConsensusMessage) {
	if rbft.in(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, reject consensus messages", rbft.peerPool.localID)
		return
	}

	if msg.Type == pb.Type_REQUEST_SET {
		set := &pb.RequestSet{}
		err := proto.Unmarshal(msg.Payload, set)
		if err != nil {
			rbft.logger.Errorf("processConsensus, unmarshal error: %s", err)
			return
		}
		set.Local = false
		// nolint: errcheck
		go rbft.postMsg(msg)
	} else {
		go rbft.postMsg(msg)
	}
}

// reportStateUpdated informs RBFT stateUpdated event.
func (rbft *rbftImpl) reportStateUpdated(state *pb.ServiceState) {
	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event:     state,
	}

	go rbft.postMsg(event)
}

// postRequests informs RBFT requests event which is posted from application layer.
func (rbft *rbftImpl) postRequests(requests []*protos.Transaction) {
	rSet := &pb.RequestSet{
		Requests: requests,
		Local:    true,
	}
	go rbft.postMsg(rSet)
}

// postRequests informs RBFT batch event which is generated by request pool.
func (rbft *rbftImpl) postBatches(batches []*txpool.RequestHashBatch) {
	for _, batch := range batches {
		_ = rbft.recvRequestBatch(batch)

		// if primary has generated a config batch, turn into config change mode
		if rbft.batchMgr.requestPool.IsConfigBatch(batch.BatchHash) {
			rbft.logger.Noticef("Primary %d has generated a config batch, start config change", rbft.peerPool.localID)
			rbft.on(InConfChange)
		}
	}
}

// postRequests informs RBFT batch event which is generated by request pool.
func (rbft *rbftImpl) postConfState(cc *pb.ConfState) {
	found := false
	for _, p := range cc.QuorumRouter.Peers {
		if p.Hash == rbft.peerPool.localHash {
			found = true
		}
	}
	if !found {
		rbft.logger.Criticalf("%s cannot find self id in quorum routers: %+v", rbft.peerPool.localHash, cc.QuorumRouter.Peers)
		rbft.on(Pending)
		return
	}
	rbft.peerPool.initPeers(cc.QuorumRouter.Peers)
	return
}

// postMsg posts messages to main loop.
func (rbft *rbftImpl) postMsg(msg interface{}) {
	rbft.recvChan <- msg
}

// getStatus returns the current consensus status.
// NOTE. This function may be invoked by application in another go-routine
// rather than the main event loop go-routine, so we need to protect
// safety of `rbft.peerPool.noMap`, `rbft.view` and `rbft.Status`
func (rbft *rbftImpl) getStatus() (status NodeStatus) {

	rbft.viewLock.RLock()
	status.View = rbft.view
	rbft.viewLock.RUnlock()

	rbft.hLock.RLock()
	status.H = rbft.h
	rbft.hLock.RUnlock()

	status.ID = rbft.peerPool.localID
	switch {
	case rbft.in(InConfChange):
		status.Status = InConfChange
	case rbft.in(InViewChange):
		status.Status = InViewChange
	case rbft.in(InRecovery):
		status.Status = InRecovery
	case rbft.in(StateTransferring):
		status.Status = StateTransferring
	case rbft.isPoolFull():
		status.Status = PoolFull
	case rbft.in(Pending):
		status.Status = Pending
	default:
		status.Status = Normal
	}

	return
}

// =============================================================================
// general event process method
// =============================================================================
// listenEvent listens and dispatches messages according to their types
func (rbft *rbftImpl) listenEvent() {
	for {
		select {
		case <-rbft.close:
			return
		case obj := <-rbft.recvChan:
			var next consensusEvent
			var ok bool
			if next, ok = obj.(consensusEvent); !ok {
				rbft.logger.Error("Can't recognize event type")
				return
			}
			for {
				next = rbft.processEvent(next)
				if next == nil {
					break
				}
			}

		}
	}
}

// processEvent process consensus messages and local events cyclically.
func (rbft *rbftImpl) processEvent(ee consensusEvent) consensusEvent {
	switch e := ee.(type) {

	case *pb.RequestSet:
		if e.Local == true {
			rbft.broadcastReqSet(e)
		}

		rbft.processReqSetEvent(e)

		return nil

	case *LocalEvent:
		return rbft.dispatchLocalEvent(e)

	case *pb.ConsensusMessage:
		return rbft.dispatchConsensusMessage(e)

	default:
		rbft.logger.Errorf("Can't recognize event type of %v.", e)
		return nil
	}
}

func (rbft *rbftImpl) dispatchConsensusMessage(msg *pb.ConsensusMessage) consensusEvent {
	// if the consensus message comes from larger epoch and current node isn't a new node,
	// track the message for that current node might be out of epoch
	if msg.Epoch > rbft.epoch {
		rbft.checkIfOutOfEpoch(msg)
	}

	// A node in different epoch or in epoch sync will reject normal consensus messages, except:
	// For epoch check:   {EpochCheck, EpochCheckResponse},
	// For sync state:    {SyncState, SyncStateResponse},
	// For fetch missing: {FetchMissingRequests, SendMissingRequests},
	// For txs requests:  {RequestSet},
	if msg.Epoch != rbft.epoch || rbft.in(InEpochSync) {
		switch msg.Type {
		case pb.Type_EPOCH_CHECK:
		case pb.Type_EPOCH_CHECK_RESPONSE:
		case pb.Type_SYNC_STATE:
		case pb.Type_SYNC_STATE_RESPONSE:
		case pb.Type_FETCH_MISSING_REQUESTS:
		case pb.Type_SEND_MISSING_REQUESTS:
		case pb.Type_REQUEST_SET:
		default:
			rbft.logger.Debugf("Replica %d in epoch %d reject msg from epoch %d",
				rbft.peerPool.localID, rbft.epoch, msg.Epoch)
			return nil
		}
	}

	if msg.Type == pb.Type_REQUEST_SET {
		if msg.Epoch != rbft.epoch {
			rbft.logger.Debugf("Replica %d in epoch %d reject msg from epoch %d",
				rbft.peerPool.localID, rbft.epoch, msg.Epoch)
			return nil
		}
		// REQUEST_SET event need to be parsed first any reset Local variable if any.
		set := &pb.RequestSet{}
		err := proto.Unmarshal(msg.Payload, set)
		if err != nil {
			rbft.logger.Errorf("Unmarshal request set failed: %s", err)
			return nil
		}
		set.Local = false
		return rbft.processReqSetEvent(set)
	}

	next, _ := rbft.msgToEvent(msg)
	return rbft.dispatchConsensusMsg(next)
}

// dispatchCoreRbftMsg dispatch core RBFT consensus messages.
func (rbft *rbftImpl) dispatchCoreRbftMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *pb.NullRequest:
		return rbft.processNullRequest(et)
	case *pb.PrePrepare:
		return rbft.recvPrePrepare(et)
	case *pb.Prepare:
		return rbft.recvPrepare(et)
	case *pb.Commit:
		return rbft.recvCommit(et)
	case *pb.Checkpoint:
		return rbft.recvCheckpoint(et)
	case *pb.FetchMissingRequests:
		return rbft.recvFetchMissingTxs(et)
	case *pb.SendMissingRequests:
		return rbft.recvSendMissingTxs(et)
	}
	return nil
}

//=============================================================================
// null request methods
//=============================================================================
// processNullRequest process null request when it come
func (rbft *rbftImpl) processNullRequest(msg *pb.NullRequest) consensusEvent {
	if rbft.in(InRecovery) {
		rbft.logger.Infof("Replica %d is in recovery, reject null request from replica %d", rbft.peerPool.localID, msg.ReplicaId)
		return nil
	}

	if rbft.in(InViewChange) {
		rbft.logger.Infof("Replica %d is in viewChange, reject null request from replica %d", rbft.peerPool.localID, msg.ReplicaId)
		return nil
	}

	if rbft.in(InEpochSync) {
		rbft.logger.Infof("Replica %d is in epochSync, reject null request from replica %d", rbft.peerPool.localID, msg.ReplicaId)
		return nil
	}

	if !rbft.isPrimary(msg.ReplicaId) { // only primary could send a null request
		rbft.logger.Warningf("Replica %d received null request from replica %d who is not primary", rbft.peerPool.localID, msg.ReplicaId)
		return nil
	}
	// if receiver is not primary, stop firstRequestTimer started after this replica finished recovery
	rbft.stopFirstRequestTimer()

	rbft.logger.Infof("Replica %d received null request from primary %d", rbft.peerPool.localID, msg.ReplicaId)

	rbft.trySyncState()
	rbft.nullReqTimerReset()

	return nil
}

// handleNullRequestEvent triggered by null request timer, primary needs to send a null request
// and replica needs to send view change
func (rbft *rbftImpl) handleNullRequestTimerEvent() {

	if rbft.in(InRecovery) {
		rbft.logger.Debugf("Replica %d try to nullRequestHandler, but it's in recovery", rbft.peerPool.localID)
		return
	}

	if rbft.in(InViewChange) {
		rbft.logger.Debugf("Replica %d try to nullRequestHandler, but it's in viewChange", rbft.peerPool.localID)
		return
	}

	if rbft.in(InEpochSync) {
		rbft.logger.Infof("Replica %d try to nullRequestHandler, but it's in epochSync", rbft.peerPool.localID)
		return
	}

	if !rbft.isPrimary(rbft.peerPool.localID) {
		// replica expects a null request, but primary never sent one
		rbft.logger.Warningf("Replica %d null request timer expired, sending viewChange", rbft.peerPool.localID)
		rbft.sendViewChange()
	} else {
		rbft.logger.Infof("Primary %d null request timer expired, sending null request", rbft.peerPool.localID)
		rbft.sendNullRequest()

		rbft.trySyncState()
	}
}

// sendNullRequest is for primary peer to send null when nullRequestTimer booms
func (rbft *rbftImpl) sendNullRequest() {

	nullRequest := &pb.NullRequest{
		ReplicaId: rbft.peerPool.localID,
	}
	payload, err := proto.Marshal(nullRequest)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_NULL_REQUEST Marshal Error: %s", err)
		return
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_NULL_REQUEST,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	rbft.nullReqTimerReset()
}

//=============================================================================
// process request set and batch methods
//=============================================================================

//processReqSetEvent process received requestSet event
func (rbft *rbftImpl) processReqSetEvent(req *pb.RequestSet) consensusEvent {
	// if current node is in abnormal or in config change, add normal txs into txPool without generate batches.
	// besides, reject ctx if it is processing a ctx at the moment
	if !rbft.isNormal() || rbft.in(InConfChange) {
		// if nodes in skipInProgress, it cannot handle txs anymore
		if rbft.in(SkipInProgress) {
			return nil
		}
		// if pool already full, rejects the tx, unless it's from RPC because of time difference
		if rbft.isPoolFull() && !req.Local {
			return nil
		}
		for _, tx := range req.Requests {
			if protos.IsConfigTx(tx) {
				// if it's already in config change, reject another config tx
				if rbft.in(InConfChange) {
					rbft.logger.Debugf("Replica %d is processing a ctx, reject another one", rbft.peerPool.localID)
					return nil
				}
			}
			// for nodes in abnormal status, add requests to requestPool without generate batch.
			rbft.batchMgr.requestPool.AddNewRequest(tx, false, req.Local)
		}
	} else {
		// primary nodes would check if this transaction triggered generating a batch or not
		if rbft.isPrimary(rbft.peerPool.localID) {
			// start batch timer when this node receives the first transaction of a batch
			if !rbft.batchMgr.isBatchTimerActive() {
				rbft.startBatchTimer()
			}
			for _, tx := range req.Requests {
				batches := rbft.batchMgr.requestPool.AddNewRequest(tx, true, req.Local)

				// If this transaction triggers generating a batch, stop batch timer
				if len(batches) != 0 {
					rbft.stopBatchTimer()
					rbft.postBatches(batches)
				}
			}
		} else {
			for _, tx := range req.Requests {
				rbft.batchMgr.requestPool.AddNewRequest(tx, false, req.Local)
			}
		}
	}

	if rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setFull()
	}

	return nil
}

// processOutOfDateReqs process the out-of-date requests in requestPool, get the remained txs from pool,
// then broadcast all the remained requests that generate by itself to others
func (rbft *rbftImpl) processOutOfDateReqs() {

	// if rbft is in abnormal, reject process remained reqs
	if !rbft.isNormal() {
		rbft.logger.Warningf("Replica %d is in abnormal, reject broadcast remained reqs", rbft.peerPool.localID)
		return
	}

	reqs, err := rbft.batchMgr.requestPool.FilterOutOfDateRequests()
	if err != nil {
		rbft.logger.Warningf("Replica %d get the remained reqs failed, error: %v", rbft.peerPool.localID, err)
	}

	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	reqLen := len(reqs)
	if reqLen == 0 {
		rbft.logger.Debugf("Replica %d in normal finds 0 remained reqs, need not broadcast to others", rbft.peerPool.localID)
		return
	}

	setSize := rbft.config.SetSize
	rbft.logger.Debugf("Replica %d in normal finds %d remained reqs, broadcast to others split by setSize %d if needed", rbft.peerPool.localID, reqLen, setSize)

	// limit TransactionSet size by setSize before re-broadcast reqs
	for reqLen > 0 {
		if reqLen <= setSize {
			set := &pb.RequestSet{Requests: reqs, Local: true}
			rbft.broadcastReqSet(set)
			reqLen = 0
		} else {
			bTxs := reqs[0:setSize]
			set := &pb.RequestSet{Requests: bTxs, Local: true}
			rbft.broadcastReqSet(set)
			reqs = reqs[setSize:]
			reqLen -= setSize
		}
	}
}

// recvRequestBatch handle logic after receive request batch
func (rbft *rbftImpl) recvRequestBatch(reqBatch *txpool.RequestHashBatch) error {

	rbft.logger.Debugf("Replica %d received request batch %s", rbft.peerPool.localID, reqBatch.BatchHash)

	batch := &pb.RequestBatch{
		RequestHashList: reqBatch.TxHashList,
		RequestList:     reqBatch.TxList,
		Timestamp:       reqBatch.Timestamp,
		LocalList:       reqBatch.LocalList,
		BatchHash:       reqBatch.BatchHash,
	}

	if rbft.isPrimary(rbft.peerPool.localID) && rbft.isNormal() {
		rbft.restartBatchTimer()
		rbft.timerMgr.stopTimer(nullRequestTimer)
		if len(rbft.batchMgr.cacheBatch) > 0 {
			rbft.batchMgr.cacheBatch = append(rbft.batchMgr.cacheBatch, batch)
			rbft.maybeSendPrePrepare(nil, true)
			return nil
		}
		rbft.maybeSendPrePrepare(batch, false)
	} else {
		rbft.logger.Debugf("Replica %d is backup, not try to send prePrepare for request batch %s", rbft.peerPool.localID, reqBatch.BatchHash)
		_ = rbft.batchMgr.requestPool.RestoreOneBatch(reqBatch.BatchHash)
	}

	return nil
}

//=============================================================================
// normal case: pre-prepare, prepare, commit methods
//=============================================================================
// sendPrePrepare send prePrepare message.
func (rbft *rbftImpl) sendPrePrepare(seqNo uint64, digest string, reqBatch *pb.RequestBatch) {

	rbft.logger.Debugf("Primary %d sending prePrepare for view=%d/seqNo=%d/digest=%s, "+
		"batch size: %d, timestamp: %d", rbft.peerPool.localID, rbft.view, seqNo, digest, len(reqBatch.RequestHashList), reqBatch.Timestamp)

	hashBatch := &pb.HashBatch{
		RequestHashList: reqBatch.RequestHashList,
		Timestamp:       reqBatch.Timestamp,
	}

	preprepare := &pb.PrePrepare{
		View:           rbft.view,
		SequenceNumber: seqNo,
		BatchDigest:    digest,
		HashBatch:      hashBatch,
		ReplicaId:      rbft.peerPool.localID,
	}

	cert := rbft.storeMgr.getCert(rbft.view, seqNo, digest)
	cert.prePrepare = preprepare
	rbft.persistQSet(preprepare)

	payload, err := proto.Marshal(preprepare)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_PRE_PREPARE Marshal Error: %s", err)
		return
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_PRE_PREPARE,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)

	// set primary's seqNo to current batch seqNo
	rbft.batchMgr.setSeqNo(seqNo)

	// exit sync state as primary is ready to process requests.
	rbft.exitSyncState()
}

// recvPrePrepare process logic for PrePrepare msg.
func (rbft *rbftImpl) recvPrePrepare(preprep *pb.PrePrepare) error {
	rbft.logger.Debugf("Replica %d received prePrepare from replica %d for view=%d/seqNo=%d, digest=%s ",
		rbft.peerPool.localID, preprep.ReplicaId, preprep.View, preprep.SequenceNumber, preprep.BatchDigest)

	if !rbft.isPrePrepareLegal(preprep) {
		return nil
	}

	if preprep.BatchDigest == "" {
		if len(preprep.HashBatch.RequestHashList) != 0 {
			rbft.logger.Warningf("Replica %d received a prePrepare with an empty digest but batch is "+
				"not empty", rbft.peerPool.localID)
			rbft.sendViewChange()
			return nil
		}
	} else {
		if len(preprep.HashBatch.DeDuplicateRequestHashList) != 0 {
			rbft.logger.Noticef("Replica %d finds %d duplicate txs with digest %s, detailed: %+v",
				rbft.peerPool.localID, len(preprep.HashBatch.DeDuplicateRequestHashList), preprep.HashBatch.DeDuplicateRequestHashList)
		}
		// check if the digest sent from primary is really the hash of txHashList, if not, don't
		// send prepare for this prePrepare
		digest := calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)
		if digest != preprep.BatchDigest {
			rbft.logger.Warningf("Replica %d received a prePrepare with a wrong batch digest, calculated: %s "+
				"primary calculated: %s, send viewChange", rbft.peerPool.localID, digest, preprep.BatchDigest)
			rbft.sendViewChange()
			return nil
		}
	}

	// in recovery, we would fetch recovery PQC, and receive these PQC again,
	// and we cannot stop timer in this situation, so we check seqNo here.
	if preprep.SequenceNumber > rbft.exec.lastExec {
		rbft.timerMgr.stopTimer(nullRequestTimer)
		rbft.stopFirstRequestTimer()
	}

	cert := rbft.storeMgr.getCert(preprep.View, preprep.SequenceNumber, preprep.BatchDigest)
	if cert.prePrepare != nil && cert.prePrepare.BatchDigest != "" && cert.prePrepare.BatchDigest != preprep.BatchDigest {
		rbft.logger.Warningf("Replica %d found same view/seqNo but different digest, received: %s, stored: %s",
			rbft.peerPool.localID, preprep.BatchDigest, cert.prePrepare.BatchDigest)
		rbft.sendViewChange()
		return nil
	}
	cert.prePrepare = preprep

	if !rbft.in(SkipInProgress) && preprep.SequenceNumber > rbft.exec.lastExec {
		rbft.softStartNewViewTimer(rbft.timerMgr.getTimeoutValue(requestTimer),
			fmt.Sprintf("new prePrepare for request batch view=%d/seqNo=%d, hash=%s",
				preprep.View, preprep.SequenceNumber, preprep.BatchDigest), false)

		// exit sync state as we start process requests now.
		rbft.exitSyncState()
	}

	rbft.persistQSet(preprep)

	if !rbft.isPrimary(rbft.peerPool.localID) && !cert.sentPrepare &&
		rbft.prePrepared(preprep.BatchDigest, preprep.View, preprep.SequenceNumber) {
		cert.sentPrepare = true
		return rbft.sendPrepare(preprep)
	}

	return nil
}

// sendPrepare send prepare message.
func (rbft *rbftImpl) sendPrepare(preprep *pb.PrePrepare) error {
	rbft.logger.Debugf("Replica %d sending prepare for view=%d/seqNo=%d", rbft.peerPool.localID, preprep.View, preprep.SequenceNumber)
	prep := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      rbft.peerPool.localID,
	}
	payload, err := proto.Marshal(prep)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_PREPARE Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_PREPARE,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)

	// send to itself
	return rbft.recvPrepare(prep)
}

// recvPrepare process logic after receive prepare message
func (rbft *rbftImpl) recvPrepare(prep *pb.Prepare) error {
	rbft.logger.Debugf("Replica %d received prepare from replica %d for view=%d/seqNo=%d",
		rbft.peerPool.localID, prep.ReplicaId, prep.View, prep.SequenceNumber)

	if !rbft.isPrepareLegal(prep) {
		return nil
	}

	cert := rbft.storeMgr.getCert(prep.View, prep.SequenceNumber, prep.BatchDigest)
	ok := cert.prepare[*prep]

	if ok {
		if prep.SequenceNumber <= rbft.exec.lastExec {
			rbft.logger.Debugf("Replica %d received duplicate prepare from replica %d, view=%d/seqNo=%d, self lastExec=%d",
				rbft.peerPool.localID, prep.ReplicaId, prep.View, prep.SequenceNumber, rbft.exec.lastExec)
			return nil
		}
		// this is abnormal in common case
		rbft.logger.Infof("Replica %d ignore duplicate prepare from replica %d, view=%d/seqNo=%d",
			rbft.peerPool.localID, prep.ReplicaId, prep.View, prep.SequenceNumber)
		return nil
	}

	cert.prepare[*prep] = true

	return rbft.maybeSendCommit(prep.BatchDigest, prep.View, prep.SequenceNumber)
}

// maybeSendCommit check if we could send commit. if no problem,
// primary and replica would send commit.
func (rbft *rbftImpl) maybeSendCommit(digest string, v uint64, n uint64) error {

	if rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d do not try to send commit because it's in stateUpdate", rbft.peerPool.localID)
		return nil
	}

	cert := rbft.storeMgr.getCert(v, n, digest)
	if cert == nil {
		rbft.logger.Errorf("Replica %d can't get the cert for the view=%d/seqNo=%d/digest=%s", rbft.peerPool.localID, v, n, digest)
		return nil
	}

	if !rbft.prepared(digest, v, n) {
		return nil
	}

	if cert.sentCommit {
		return nil
	}

	if rbft.isPrimary(rbft.peerPool.localID) {
		return rbft.sendCommit(digest, v, n)
	}

	return rbft.findNextCommitBatch(digest, v, n)
}

// sendCommit send commit message.
func (rbft *rbftImpl) sendCommit(digest string, v uint64, n uint64) error {
	cert := rbft.storeMgr.getCert(v, n, digest)

	rbft.logger.Debugf("Replica %d sending commit for view=%d/seqNo=%d", rbft.peerPool.localID, v, n)
	commit := &pb.Commit{
		View:           v,
		SequenceNumber: n,
		BatchDigest:    digest,
		ReplicaId:      rbft.peerPool.localID,
		Epoch:          rbft.epoch,
	}
	cert.sentCommit = true

	rbft.persistPSet(v, n, digest)

	payload, err := proto.Marshal(commit)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_COMMIT Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_COMMIT,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	return rbft.recvCommit(commit)
}

// recvCommit process logic after receive commit message.
func (rbft *rbftImpl) recvCommit(commit *pb.Commit) error {
	rbft.logger.Debugf("Replica %d received commit from replica %d for view=%d/seqNo=%d",
		rbft.peerPool.localID, commit.ReplicaId, commit.View, commit.SequenceNumber)

	if !rbft.isCommitLegal(commit) {
		return nil
	}

	cert := rbft.storeMgr.getCert(commit.View, commit.SequenceNumber, commit.BatchDigest)

	ok := cert.commit[*commit]

	if ok {
		if commit.SequenceNumber <= rbft.exec.lastExec {
			// ignore duplicate commit with seqNo <= lastExec as this commit is not useful forever.
			rbft.logger.Debugf("Replica %d received duplicate commit from replica %d, view=%d/seqNo=%d "+
				"but current lastExec is %d, ignore it...", rbft.peerPool.localID, commit.ReplicaId, commit.View, commit.SequenceNumber,
				rbft.exec.lastExec)
			return nil
		}
		// we can simply accept all commit messages whose seqNo is larger than our lastExec as we
		// haven't execute this batch and we can ensure that we will only execute this batch once.
		rbft.logger.Debugf("Replica %d accept duplicate commit from replica %d, view=%d/seqNo=%d "+
			"current lastExec is %d", rbft.peerPool.localID, commit.ReplicaId, commit.View, commit.SequenceNumber, rbft.exec.lastExec)
	}

	cert.commit[*commit] = true

	if rbft.committed(commit.BatchDigest, commit.View, commit.SequenceNumber) {
		idx := msgID{v: commit.View, n: commit.SequenceNumber, d: commit.BatchDigest}
		if !cert.sentExecute && cert.sentCommit {
			rbft.storeMgr.committedCert[idx] = commit.BatchDigest
			rbft.commitPendingBlocks()

			// reset last new view timeout after commit one block successfully.
			rbft.vcMgr.lastNewViewTimeout = rbft.timerMgr.getTimeoutValue(newViewTimer)
			if commit.SequenceNumber == rbft.vcMgr.viewChangeSeqNo {
				rbft.logger.Warningf("Replica %d cycling view for seqNo=%d", rbft.peerPool.localID, commit.SequenceNumber)
				rbft.sendViewChange()
			}
		} else {
			rbft.logger.Debugf("Replica %d committed for seqNo: %d, but sentExecute: %v", rbft.peerPool.localID, commit.SequenceNumber, cert.sentExecute)
		}
	}
	return nil
}

// fetchMissingTxs fetch missing txs from primary which this node didn't receive but primary received
func (rbft *rbftImpl) fetchMissingTxs(prePrep *pb.PrePrepare, missingTxHashes map[uint64]string) error {
	rbft.logger.Debugf("Replica %d try to fetch missing txs for view=%d/seqNo=%d/digest=%s from primary %d",
		rbft.peerPool.localID, prePrep.View, prePrep.SequenceNumber, prePrep.BatchDigest, prePrep.ReplicaId)

	fetch := &pb.FetchMissingRequests{
		View:                 prePrep.View,
		SequenceNumber:       prePrep.SequenceNumber,
		BatchDigest:          prePrep.BatchDigest,
		MissingRequestHashes: missingTxHashes,
		ReplicaId:            rbft.peerPool.localID,
	}

	payload, err := proto.Marshal(fetch)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_FETCH_MISSING_TXS Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_FETCH_MISSING_REQUESTS,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.unicast(consensusMsg, prePrep.ReplicaId)
	return nil
}

// recvFetchMissingTxs returns txs to a node which didn't receive some txs and ask primary for them.
func (rbft *rbftImpl) recvFetchMissingTxs(fetch *pb.FetchMissingRequests) error {
	rbft.logger.Debugf("Primary %d received fetchMissingTxs request for view=%d/seqNo=%d/digest=%s from replica %d",
		rbft.peerPool.localID, fetch.View, fetch.SequenceNumber, fetch.BatchDigest, fetch.ReplicaId)

	requests := make(map[uint64]*protos.Transaction)
	var err error

	if batch := rbft.storeMgr.batchStore[fetch.BatchDigest]; batch != nil {
		batchLen := uint64(len(batch.RequestHashList))
		for i, hash := range fetch.MissingRequestHashes {
			if i >= batchLen || batch.RequestHashList[i] != hash {
				rbft.logger.Errorf("Primary %d finds mismatch requests hash when return fetch missing requests", rbft.peerPool.localID)
				return nil
			}
			requests[i] = batch.RequestList[i]
		}
	} else {
		var missingTxs map[uint64]*protos.Transaction
		missingTxs, err = rbft.batchMgr.requestPool.SendMissingRequests(fetch.BatchDigest, fetch.MissingRequestHashes)
		if err != nil {
			rbft.logger.Warningf("Primary %d cannot find the digest %s, missing tx hashes: %+v, err: %s",
				rbft.peerPool.localID, fetch.BatchDigest, fetch.MissingRequestHashes, err)
			return nil
		}
		for i, tx := range missingTxs {
			requests[i] = tx
		}
	}

	re := &pb.SendMissingRequests{
		View:                 fetch.View,
		SequenceNumber:       fetch.SequenceNumber,
		BatchDigest:          fetch.BatchDigest,
		MissingRequestHashes: fetch.MissingRequestHashes,
		MissingRequests:      requests,
		ReplicaId:            rbft.peerPool.localID,
	}

	defer func() {
		if r := recover(); r != nil {
			rbft.logger.Warningf("Primary %d finds marshal error: %v in marshaling SendMissingTxs, retry.", rbft.peerPool.localID, r)
			localEvent := &LocalEvent{
				Service:   CoreRbftService,
				EventType: CoreResendMissingTxsEvent,
				Event:     fetch,
			}

			go rbft.postMsg(localEvent)
		}
	}()

	payload, err := proto.Marshal(re)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_SEND_MISSING_TXS Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_SEND_MISSING_REQUESTS,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.unicast(consensusMsg, fetch.ReplicaId)

	return nil
}

// recvSendMissingTxs processes SendMissingTxs from primary.
// Add these transaction txs to requestPool and see if it has correct transaction txs.
func (rbft *rbftImpl) recvSendMissingTxs(re *pb.SendMissingRequests) consensusEvent {
	rbft.logger.Debugf("Replica %d received sendMissingTxs for view=%d/seqNo=%d/digest=%s from replica %d",
		rbft.peerPool.localID, re.View, re.SequenceNumber, re.BatchDigest, re.ReplicaId)

	if re.SequenceNumber < rbft.exec.lastExec {
		rbft.logger.Debugf("Replica %d ignore return missing tx with lower seqNo %d than "+
			"lastExec %d", rbft.peerPool.localID, re.SequenceNumber, rbft.exec.lastExec)
		return nil
	}

	if len(re.MissingRequests) != len(re.MissingRequestHashes) {
		rbft.logger.Warningf("Replica %d received mismatch length return %v", rbft.peerPool.localID, re)
		return nil
	}

	if !rbft.inV(re.View) {
		rbft.logger.Debugf("Replica %d received return missing transactions which has a different view=%d,"+
			"expected.View %d, ignore it", rbft.peerPool.localID, re.View, rbft.view)
		return nil
	}

	cert := rbft.storeMgr.getCert(re.View, re.SequenceNumber, re.BatchDigest)
	if cert.sentCommit {
		rbft.logger.Debugf("Replica %d received return missing transactions which has been committed with "+
			"batch seqNo=%d, ignore it", rbft.peerPool.localID, re.SequenceNumber)
		return nil
	}
	if cert.prePrepare == nil {
		rbft.logger.Warningf("Replica %d had not received a prePrepare before for view=%d/seqNo=%d",
			rbft.peerPool.localID, re.View, re.SequenceNumber)
		return nil
	}

	err := rbft.batchMgr.requestPool.ReceiveMissingRequests(re.BatchDigest, re.MissingRequests)
	if err != nil {
		rbft.logger.Warningf("Replica %d find something wrong with the return of missing txs, error: %v",
			rbft.peerPool.localID, err)
		return nil
	}

	_ = rbft.findNextCommitBatch(re.BatchDigest, re.View, re.SequenceNumber)
	return nil
}

//=============================================================================
// execute transactions
//=============================================================================

// commitPendingBlocks commit all available transactions by order
func (rbft *rbftImpl) commitPendingBlocks() {

	if rbft.exec.currentExec != nil {
		rbft.logger.Debugf("Replica %d not attempting to commitTransactions because it is currently executing %d",
			rbft.peerPool.localID, rbft.exec.currentExec)
	}
	rbft.logger.Debugf("Replica %d attempting to commitTransactions", rbft.peerPool.localID)

	for hasTxToExec := true; hasTxToExec; {
		if find, idx, cert := rbft.findNextCommitTx(); find {
			rbft.logger.Noticef("======== Replica %d Call execute, view=%d/seqNo=%d", rbft.peerPool.localID, idx.v, idx.n)
			rbft.persistCSet(idx.v, idx.n, idx.d)
			// stop new view timer after one batch has been call executed
			rbft.stopNewViewTimer()
			if ok := rbft.isPrimary(rbft.peerPool.localID); ok {
				rbft.softRestartBatchTimer()
			}

			if idx.d == "" {
				rbft.logger.Noticef("Replica %d try to execute a no-op", rbft.peerPool.localID)
				txList := make([]*protos.Transaction, 0)
				localList := make([]bool, 0)
				rbft.external.Execute(txList, localList, idx.n, 0)
			} else {
				// find batch in batchStore rather than outstandingBatch as after viewChange
				// we may clear outstandingBatch and save all batches in batchStore.
				// kick out de-duplicate txs if needed.
				if rbft.batchMgr.requestPool.IsConfigBatch(idx.d) {
					if rbft.readConfigTransactionToExecute() != uint64(0) {
						rbft.logger.Errorf("Replica %d is processing a config transaction, cannot process another config transaction",
							rbft.peerPool.localID)
						return
					}
					rbft.logger.Debugf("Replica %d found a config batch, set epoch start seq no %d",
						rbft.peerPool.localID, idx.n)
					rbft.setConfigTransactionToExecute(idx.n)
				}
				txList, localList := rbft.filterExecutableTxs(idx.d, cert.prePrepare.HashBatch.DeDuplicateRequestHashList)
				rbft.external.Execute(txList, localList, idx.n, cert.prePrepare.HashBatch.Timestamp)
			}
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			cert.sentExecute = true

			// if it is a config batch, start to wait for reload after the batch committed
			rbft.afterCommitBlock(idx)
		} else {
			hasTxToExec = false
		}
	}
	rbft.startTimerIfOutstandingRequests()
}

// flattenArray flatten txs into txs and kick out duplicate txs with hash included in deDuplicateTxHashes.
func (rbft *rbftImpl) filterExecutableTxs(digest string, deDuplicateRequestHashes []string) ([]*protos.Transaction, []bool) {
	var (
		txList, executableTxs          []*protos.Transaction
		localList, executableLocalList []bool
	)
	txList = rbft.storeMgr.batchStore[digest].RequestList
	localList = rbft.storeMgr.batchStore[digest].LocalList
	dupHashes := make(map[string]bool)
	for _, dupHash := range deDuplicateRequestHashes {
		dupHashes[dupHash] = true
	}
	for i, request := range txList {
		reqHash := requestHash(request)
		if dupHashes[reqHash] {
			rbft.logger.Noticef("Replica %d kick out de-duplicate request %s before execute batch %s", rbft.peerPool.localID, reqHash, digest)
			continue
		}
		executableTxs = append(executableTxs, request)
		executableLocalList = append(executableLocalList, localList[i])
	}
	return executableTxs, executableLocalList
}

// findNextCommitTx find next msgID which is able to commit.
func (rbft *rbftImpl) findNextCommitTx() (find bool, idx msgID, cert *msgCert) {

	for idx = range rbft.storeMgr.committedCert {
		cert = rbft.storeMgr.certStore[idx]

		if cert == nil || cert.prePrepare == nil {
			rbft.logger.Debugf("Replica %d already checkpoint for view=%d/seqNo=%d", rbft.peerPool.localID, idx.v, idx.n)
			continue
		}

		// check if already executed
		if cert.sentExecute == true {
			rbft.logger.Debugf("Replica %d already execute for view=%d/seqNo=%d", rbft.peerPool.localID, idx.v, idx.n)
			continue
		}

		if idx.n != rbft.exec.lastExec+1 {
			rbft.logger.Debugf("Replica %d expects to execute seq=%d, but get seq=%d, ignore it", rbft.peerPool.localID, rbft.exec.lastExec+1, idx.n)
			continue
		}

		// skipInProgress == true, then this replica is in viewchange, not reply or execute
		if rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d currently picking a starting point to resume, will not execute", rbft.peerPool.localID)
			continue
		}

		// check if committed
		if !rbft.committed(idx.d, idx.v, idx.n) {
			continue
		}

		if idx.d != "" {
			_, ok := rbft.storeMgr.batchStore[idx.d]
			if !ok {
				rbft.logger.Warningf("Replica %d cannot find corresponding batch %s in batchStore", rbft.peerPool.localID, idx.d)
				continue
			}
		}

		currentExec := idx.n
		rbft.exec.setCurrentExec(&currentExec)

		find = true
		break
	}

	return
}

// afterCommitTx processes logic after commit transaction, update lastExec,
// and generate checkpoint when lastExec % K == 0
func (rbft *rbftImpl) afterCommitBlock(idx msgID) {
	if rbft.exec.currentExec != nil {
		rbft.logger.Debugf("Replica %d finished execution %d, trying next", rbft.peerPool.localID, *rbft.exec.currentExec)
		rbft.exec.setLastExec(*rbft.exec.currentExec)
		delete(rbft.storeMgr.committedCert, idx)

		if rbft.exec.lastExec%rbft.K == 0 {
			state := <-rbft.cpChan
			if rbft.readConfigTransactionToExecute() == state.Applied {
				rbft.processReload(idx, true)
			}
			if state.Applied == rbft.exec.lastExec {
				rbft.logger.Debugf("Call the checkpoint, seqNo=%d", rbft.exec.lastExec)
				rbft.checkpoint(rbft.exec.lastExec, state)
			} else {
				// reqBatch call execute but have not done with execute
				rbft.logger.Errorf("Fail to call the checkpoint, seqNo=%d", rbft.exec.lastExec)
			}
		} else {
			if rbft.readConfigTransactionToExecute() != uint64(0) {
				rbft.processReload(idx, false)
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d had execDoneSync called, flagging ourselves as out of data", rbft.peerPool.localID)
		rbft.on(SkipInProgress)
	}

	rbft.exec.currentExec = nil
}

// processReload will wait for the config batch executed
func (rbft *rbftImpl) processReload(idx msgID, isCheckpoint bool) {
	rbft.logger.Debugf("Replica %d is trying to update epoch %d, waiting for config block executed", rbft.peerPool.localID, idx.n)
	if !isCheckpoint {
		<-rbft.confChan
	}

	// two types of reload
	// 1. validator set changed, start a new epoch and start epoch check. when epoch check succeed,
	//    we will start new epoch and restart consensus.
	// 2. validator set not changed, directly finish config change and do not start new epoch.
	router := rbft.node.readReloadRouter()
	if router != nil {
		// validator set has been changed, start a new epoch and check new epoch
		info, err := proto.Marshal(router)
		if err != nil {
			rbft.logger.Errorf("ConsensusMessage_SYNC_STATE marshal error: %v", err)
			return
		}
		rbft.peerPool.updateRouter(info)
		rbft.updateEpochStartState()
		rbft.turnIntoEpoch(idx.n)
		rbft.initEpochCheck()
		rbft.node.setReloadRouter(nil)
	} else {
		// validator set has not been changed, do not start new epoch and directly finish config change
		rbft.off(InConfChange)
		rbft.maybeSetNormal()
		rbft.startTimerIfOutstandingRequests()
	}

	// reset config transaction to execute
	rbft.setConfigTransactionToExecute(uint64(0))
}

//=============================================================================
// gc: checkpoint issues
//=============================================================================

// checkpoint generate a checkpoint and broadcast it to outer.
func (rbft *rbftImpl) checkpoint(n uint64, state *pb.ServiceState) {

	if n%rbft.K != 0 {
		rbft.logger.Errorf("Attempted to checkpoint a sequence number (%d) which is not a multiple of the checkpoint interval (%d)", n, rbft.K)
		return
	}

	digest := state.Digest
	seqNo := n

	rbft.logger.Infof("Replica %d sending checkpoint for view=%d/seqNo=%d and b64Id=%s",
		rbft.peerPool.localID, rbft.view, seqNo, digest)

	chkpt := &pb.Checkpoint{
		SequenceNumber: seqNo,
		Digest:         digest,
		ReplicaId:      rbft.peerPool.localID,
	}
	rbft.storeMgr.saveCheckpoint(seqNo, digest)
	rbft.persistCheckpoint(seqNo, []byte(digest))

	payload, err := proto.Marshal(chkpt)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_CHECKPOINT Marshal Error: %s", err)
		return
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_CHECKPOINT,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	rbft.recvCheckpoint(chkpt)
}

// recvCheckpoint processes logic after receive checkpoint.
func (rbft *rbftImpl) recvCheckpoint(chkpt *pb.Checkpoint) consensusEvent {
	rbft.logger.Debugf("Replica %d received checkpoint from replica %d, seqNo %d, digest %s",
		rbft.peerPool.localID, chkpt.ReplicaId, chkpt.SequenceNumber, chkpt.Digest)

	if rbft.in(InEpochSync) {
		rbft.logger.Debugf("Replica %d is in epoch sync, reject checkpoint message", rbft.peerPool.localID)
		return nil
	}

	// weakCheckpointSetOutOfRange checks if this node is fell behind or not. If we receive f+1 checkpoints whose seqNo > H (for example 150),
	// move watermark to the smallest seqNo (150) among these checkpoints, because this node is fell behind at least 50 blocks.
	// Then when this node receives f+1 checkpoints whose seqNo (160) is larger than 150,
	// enter witnessCheckpointWeakCert and set highStateTarget to 160, then this node would find itself fell behind and trigger state update
	if rbft.weakCheckpointSetOutOfRange(chkpt) {
		return nil
	}

	legal, matching := rbft.compareCheckpointWithWeakSet(chkpt)
	if !legal {
		rbft.logger.Debugf("Replica %d ignore illegal checkpoint from replica %d, seqNo=%d", rbft.peerPool.localID, chkpt.ReplicaId, chkpt.SequenceNumber)
		return nil
	}

	rbft.logger.Debugf("Replica %d found %d matching checkpoints for seqNo %d, digest %s",
		rbft.peerPool.localID, matching, chkpt.SequenceNumber, chkpt.Digest)

	if matching == rbft.oneCorrectQuorum() {
		// update state update target and state transfer to it if this node already fell behind
		rbft.witnessCheckpointWeakCert(chkpt)
	}

	if matching < rbft.commonCaseQuorum() {
		// We do not have a quorum yet
		return nil
	}

	// It is actually just fine if we do not have this checkpoint
	// and should not trigger a state transfer
	// Imagine we are executing sequence number k-1 and we are slow for some reason
	// then everyone else finishes executing k, and we receive a checkpoint quorum
	// which we will agree with very shortly, but do not move our watermarks until
	// we have reached this checkpoint
	// Note, this is not divergent from the paper, as the paper requires that
	// the quorum certificate must contain 2f+1 messages, including its own
	_, ok := rbft.storeMgr.chkpts[chkpt.SequenceNumber]
	if !ok {
		rbft.logger.Debugf("Replica %d found checkpoint quorum for seqNo %d, digest %s, but it has not reached this checkpoint itself yet",
			rbft.peerPool.localID, chkpt.SequenceNumber, chkpt.Digest)
		if rbft.in(SkipInProgress) {
			// When this node started state update, it would set h to the target, and finally it would receive a StateUpdatedEvent whose seqNo is this h.
			if rbft.in(InRecovery) {
				// If this node is in recovery, it wants to state update to a latest checkpoint so it would not fall behind more than 10 block.
				// So if move watermarks here, this node would receive StateUpdatedEvent whose seqNo is smaller than h,
				// and it would tryStateTransfer.
				// If not move watermarks here, this node would fall behind more than ten block,
				// and this is different from what we want to do using recovery.
				rbft.logger.Debugf("Replica %d in recovery finds a quorum checkpoint(%d) we don't have, directly move watermark.", rbft.peerPool.localID, chkpt.SequenceNumber)
				rbft.moveWatermarks(chkpt.SequenceNumber)
			} else {
				// If this node is not in recovery, and this node just fell behind in 20 blocks, this node could just commit and execute.
				// If larger than 20 blocks, just state update.
				logSafetyBound := rbft.h + rbft.L/2
				// As an optimization, if we are more than half way out of our log and in state transfer, move our watermarks so we don't lose track of the network
				// if needed, state transfer will restart on completion to a more recent point in time
				if chkpt.SequenceNumber >= logSafetyBound {
					rbft.logger.Debugf("Replica %d is in tryStateTransfer, but, the network seems to be moving on past logSafetyBound %d, moving our watermarks to stay with it", rbft.peerPool.localID, logSafetyBound)
					rbft.moveWatermarks(chkpt.SequenceNumber)
				}
			}
		}
		return nil
	}

	rbft.logger.Infof("Replica %d found checkpoint quorum for seqNo %d, digest %s",
		rbft.peerPool.localID, chkpt.SequenceNumber, chkpt.Digest)

	rbft.moveWatermarks(chkpt.SequenceNumber)
	rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d after "+
		"executed to the height with the same digest", rbft.peerPool.localID, rbft.h)
	rbft.external.SendFilterEvent(pb.InformType_FilterStableCheckpoint, rbft.h)

	return nil
}

// weakCheckpointSetOutOfRange checks if this node is fell behind or not. If we receive f+1 checkpoints whose seqNo > H (for example 150),
// move watermark to the smallest seqNo (150) among these checkpoints, because this node is fell behind 5 blocks at least.
func (rbft *rbftImpl) weakCheckpointSetOutOfRange(chkpt *pb.Checkpoint) bool {
	H := rbft.h + rbft.L

	// Track the last observed checkpoint sequence number if it exceeds our high watermark, keyed by replica to prevent unbounded growth
	if chkpt.SequenceNumber < H {
		// For non-byzantine nodes, the checkpoint sequence number increases monotonically
		delete(rbft.storeMgr.hChkpts, chkpt.ReplicaId)
	} else {
		// We do not track the highest one, as a byzantine node could pick an arbitrarily high sequence number
		// and even if it recovered to be non-byzantine, we would still believe it to be far ahead
		rbft.storeMgr.hChkpts[chkpt.ReplicaId] = chkpt.SequenceNumber

		// If f+1 other replicas have reported checkpoints that were (at one time) outside our watermarks
		// we need to check to see if we have fallen behind.
		if len(rbft.storeMgr.hChkpts) >= rbft.oneCorrectQuorum() {
			chkptSeqNumArray := make([]uint64, len(rbft.storeMgr.hChkpts))
			index := 0
			for replicaID, hChkpt := range rbft.storeMgr.hChkpts {
				chkptSeqNumArray[index] = hChkpt
				index++
				if hChkpt < H {
					delete(rbft.storeMgr.hChkpts, replicaID)
				}
			}
			sort.Sort(sortableUint64List(chkptSeqNumArray))

			// If f+1 nodes have issued checkpoints above our high water mark, then
			// we will never record 2f+1 checkpoints for that sequence number, we are out of date
			// (This is because all_replicas - missed - me = 3f+1 - f - 1 = 2f)
			if m := chkptSeqNumArray[len(chkptSeqNumArray)-rbft.oneCorrectQuorum()]; m > H {
				if rbft.exec.lastExec >= chkpt.SequenceNumber {
					rbft.logger.Warningf("Replica %d is ahead of others, waiting others catch up", rbft.peerPool.localID)
					return true
				}
				rbft.logger.Warningf("Replica %d is out of date, f+1 nodes agree checkpoint with seqNo %d exists but our high water mark is %d", rbft.peerPool.localID, chkpt.SequenceNumber, H)
				rbft.storeMgr.batchStore = make(map[string]*pb.RequestBatch)
				rbft.persistDelAllBatches()
				rbft.cleanOutstandingAndCert()
				rbft.moveWatermarks(m)
				rbft.on(SkipInProgress)
				// If we are in viewChange/recovery, but received f+1 checkpoint higher than our H, don't
				// stop new view timer here, else we may in infinite viewChange status.
				if !rbft.in(InViewChange) && !rbft.in(InRecovery) {
					rbft.stopNewViewTimer()
				}
				return true
			}
		}
	}

	return false
}

// witnessCheckpointWeakCert updates state update target and state transfer to it if this node already fell behind
func (rbft *rbftImpl) witnessCheckpointWeakCert(chkpt *pb.Checkpoint) {

	// Only ever invoked for the first weak cert, so guaranteed to be f+1
	checkpointMembers := make([]replicaInfo, rbft.oneCorrectQuorum())
	i := 0
	for checkpoint := range rbft.storeMgr.checkpointStore {
		if checkpoint.SequenceNumber == chkpt.SequenceNumber && checkpoint.Digest == chkpt.Digest {
			// we shouldn't put self into target
			if checkpoint.ReplicaId != rbft.peerPool.localID {
				checkpointMembers[i] = replicaInfo{replicaID: checkpoint.ReplicaId}
			}
			rbft.logger.Debugf("Replica %d adding replica %d with height %d to weak cert",
				rbft.peerPool.localID, checkpoint.ReplicaId, checkpoint.SequenceNumber)
			i++
		}
	}

	target := &stateUpdateTarget{
		targetMessage: targetMessage{
			height: chkpt.SequenceNumber,
			digest: chkpt.Digest,
		},
		replicas: checkpointMembers,
	}
	rbft.updateHighStateTarget(target)

	if rbft.in(SkipInProgress) {
		rbft.logger.Infof("Replica %d is catching up and witnessed a weak certificate for checkpoint %d, "+
			"weak cert attested to by %d replicas of %d, and checkpointMembers are (%+v)",
			rbft.peerPool.localID, chkpt.SequenceNumber, i, rbft.N, checkpointMembers)
		rbft.tryStateTransfer(target)
	}
}

// moveWatermarks move low watermark h to n, and clear all message whose seqNo is smaller than h.
func (rbft *rbftImpl) moveWatermarks(n uint64) {

	// round down n to previous low watermark
	h := n / rbft.K * rbft.K

	if rbft.h > n {
		rbft.logger.Criticalf("Replica %d moveWaterMarks but rbft.h(h=%d)>n(n=%d)", rbft.peerPool.localID, rbft.h, n)
		return
	}

	for idx := range rbft.storeMgr.certStore {
		if idx.n <= h {
			rbft.logger.Debugf("Replica %d cleaning quorum certificate for view=%d/seqNo=%d",
				rbft.peerPool.localID, idx.v, idx.n)
			delete(rbft.storeMgr.certStore, idx)
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
		}
	}

	// retain most recent 10 block info in txBatchStore cache as non-primary
	// replicas may need to fetch those batches if they are lack of some txs
	// in those batches.
	var target uint64
	if h <= 10 {
		target = 0
	} else {
		target = h - uint64(10)
	}

	var digestList []string
	for digest, batch := range rbft.storeMgr.batchStore {
		if batch.SeqNo <= target {
			delete(rbft.storeMgr.batchStore, digest)
			rbft.persistDelBatch(digest)
			digestList = append(digestList, digest)
		}
	}
	rbft.batchMgr.requestPool.RemoveBatches(digestList)

	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	for idx := range rbft.storeMgr.committedCert {
		if idx.n <= h {
			delete(rbft.storeMgr.committedCert, idx)
		}
	}

	for testChkpt := range rbft.storeMgr.checkpointStore {
		if testChkpt.SequenceNumber <= h {
			rbft.logger.Debugf("Replica %d cleaning checkpoint message from replica %d, seqNo %d, b64 snapshot no %s",
				rbft.peerPool.localID, testChkpt.ReplicaId, testChkpt.SequenceNumber, testChkpt.Digest)
			delete(rbft.storeMgr.checkpointStore, testChkpt)
		}
	}

	rbft.storeMgr.moveWatermarks(rbft, h)

	rbft.hLock.RLock()
	rbft.h = h
	rbft.persistH(h)
	rbft.hLock.RUnlock()

	rbft.logger.Infof("Replica %d updated low water mark to %d", rbft.peerPool.localID, rbft.h)
}

// updateHighStateTarget updates high state target
func (rbft *rbftImpl) updateHighStateTarget(target *stateUpdateTarget) {
	if rbft.storeMgr.highStateTarget != nil && rbft.storeMgr.highStateTarget.height >= target.height {
		rbft.logger.Infof("Replica %d not updating state target to seqNo %d, has target for seqNo %d",
			rbft.peerPool.localID, target.height, rbft.storeMgr.highStateTarget.height)
		return
	}

	rbft.logger.Debugf("Replica %d updating state target to seqNo %d", rbft.peerPool.localID, target.height)
	rbft.storeMgr.highStateTarget = target
}

// tryStateTransfer sets system abnormal and stateTransferring, then skips to target
func (rbft *rbftImpl) tryStateTransfer(target *stateUpdateTarget) {
	if !rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d is out of sync, pending tryStateTransfer", rbft.peerPool.localID)
		rbft.on(SkipInProgress)
	}

	rbft.setAbNormal()

	if rbft.in(StateTransferring) {
		rbft.logger.Debugf("Replica %d is currently mid tryStateTransfer, it must wait for this tryStateTransfer to complete before initiating a new one", rbft.peerPool.localID)
		return
	}

	// if target is nil, set target to highStateTarget
	if target == nil {
		if rbft.storeMgr.highStateTarget == nil {
			rbft.logger.Debugf("Replica %d has no targets to attempt tryStateTransfer to, delaying", rbft.peerPool.localID)
			return
		}
		target = rbft.storeMgr.highStateTarget
	}
	rbft.on(StateTransferring)
	rbft.batchMgr.requestPool.Reset()
	// reset the status of PoolFull
	rbft.setNotFull()

	// clean cert with seqNo > lastExec before stateUpdate to avoid those cert with 'validated' flag influencing the
	// following progress
	for idx := range rbft.storeMgr.certStore {
		if idx.n > rbft.exec.lastExec && idx.n <= target.height {
			rbft.logger.Debugf("Replica %d clean cert with seqNo %d > lastExec %d before state update", rbft.peerPool.localID, idx.n, rbft.exec.lastExec)
			delete(rbft.storeMgr.certStore, idx)
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			delete(rbft.storeMgr.committedCert, idx)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
		}
	}

	rbft.logger.Infof("Replica %d try state update to %d", rbft.peerPool.localID, target.height)

	// attempts to synchronize state to a particular target, implicitly calls rollback if needed
	// TODO(DH): integrate extra state into peers
	var peers []uint64
	for _, info := range target.replicas {
		peers = append(peers, info.replicaID)
	}
	rbft.external.StateUpdate(target.height, target.digest, peers)
}

// recvStateUpdatedEvent processes StateUpdatedMessage.
func (rbft *rbftImpl) recvStateUpdatedEvent(ss *pb.ServiceState) consensusEvent {
	seqNo := ss.Applied

	// when epoch sync, check the digest of state with epoch start state
	// if equal, it's a correct state the epoch start, finish epoch sync
	// if not equal, something went wrong with state update, restart state update
	if rbft.in(InEpochSync) {
		if ss.Applied == rbft.epochMgr.epochStartState.Applied && ss.Digest == rbft.epochMgr.epochStartState.Digest {
			rbft.logger.Infof("======== Replica %d finished stateUpdate, height: %d", rbft.peerPool.localID, seqNo)
			finishMsg := fmt.Sprintf("======== Replica %d finished stateUpdate, height: %d", rbft.peerPool.localID, seqNo)

			rbft.external.SendFilterEvent(pb.InformType_FilterFinishStateUpdate, finishMsg)
			rbft.exec.setLastExec(seqNo)
			rbft.batchMgr.setSeqNo(seqNo)
			rbft.off(SkipInProgress)
			rbft.off(StateTransferring)
			rbft.node.setReloadRouter(nil)

			return &LocalEvent{
				Service:   EpochMgrService,
				EventType: EpochSyncFinishedEvent,
			}
		}
		rbft.logger.Warningf("Replica %d restarts to state update, expect state {%d, %s}, get state {%d, %s}",
			rbft.peerPool.localID, rbft.epochMgr.epochStartState.Applied, rbft.epochMgr.epochStartState.Digest, ss.Applied, ss.Digest)
		rbft.external.StateUpdate(rbft.epochMgr.epochStartState.Applied, rbft.epochMgr.epochStartState.Digest, nil)
		return nil
	}

	rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d after "+
		"state update to the height", rbft.peerPool.localID, seqNo)
	rbft.external.SendFilterEvent(pb.InformType_FilterStableCheckpoint, seqNo)

	// If state transfer did not complete successfully, or if it did not reach our low watermark, do it again
	// When this node moves watermark before this node receives StateUpdatedMessage, this would happen.
	if seqNo < rbft.h {
		rbft.logger.Warningf("Replica %d recovered to seqNo %d but our low watermark has moved to %d", rbft.peerPool.localID, seqNo, rbft.h)
		if rbft.storeMgr.highStateTarget == nil {
			rbft.logger.Debugf("Replica %d has no state targets, cannot resume tryStateTransfer yet", rbft.peerPool.localID)
		} else if seqNo < rbft.storeMgr.highStateTarget.height {
			rbft.logger.Debugf("Replica %d has state target for %d, transferring", rbft.peerPool.localID, rbft.storeMgr.highStateTarget.height)
			rbft.off(StateTransferring)
			rbft.exec.setLastExec(seqNo)
			rbft.tryStateTransfer(nil)
		} else if seqNo == rbft.storeMgr.highStateTarget.height {
			rbft.logger.Debugf("Replica %d recovered to seqNo %d and highest is %d, turn off stateTransferring", rbft.peerPool.localID, seqNo, rbft.storeMgr.highStateTarget.height)
			rbft.exec.setLastExec(seqNo)
			rbft.off(StateTransferring)
		} else {
			rbft.logger.Debugf("Replica %d has no state target above %d, highest is %d", rbft.peerPool.localID, seqNo, rbft.storeMgr.highStateTarget.height)
		}
		return nil
	}

	rbft.logger.Infof("======== Replica %d finished stateUpdate, height: %d", rbft.peerPool.localID, seqNo)
	finishMsg := fmt.Sprintf("======== Replica %d finished stateUpdate, height: %d", rbft.peerPool.localID, seqNo)

	rbft.external.SendFilterEvent(pb.InformType_FilterFinishStateUpdate, finishMsg)
	rbft.exec.setLastExec(seqNo)
	rbft.batchMgr.setSeqNo(seqNo)
	rbft.off(SkipInProgress)
	rbft.off(StateTransferring)
	rbft.maybeSetNormal()

	if seqNo%rbft.K == 0 {
		state := rbft.node.getCurrentState()
		rbft.checkpoint(state.Applied, state)
	}

	if rbft.in(InRecovery) {
		if rbft.isPrimary(rbft.peerPool.localID) {
			rbft.logger.Debugf("Primary %d init recovery after state update", rbft.peerPool.localID)
			return rbft.initRecovery()
		}
		finishMsg := fmt.Sprintf("======== Replica %d finished recovery after stateUpdate, height: %d", rbft.peerPool.localID, seqNo)
		rbft.external.SendFilterEvent(pb.InformType_FilterFinishRecovery, finishMsg)

		return &LocalEvent{
			Service:   RecoveryService,
			EventType: RecoveryDoneEvent,
		}

	}
	rbft.executeAfterStateUpdate()

	return nil
}

// executeAfterStateUpdate processes logic after state update
func (rbft *rbftImpl) executeAfterStateUpdate() {

	if rbft.isPrimary(rbft.peerPool.localID) {
		rbft.logger.Debugf("Replica %d is primary, not execute after stateUpdate", rbft.peerPool.localID)
		return
	}

	rbft.logger.Debugf("Replica %d try to execute after stateUpdate", rbft.peerPool.localID)

	for idx, cert := range rbft.storeMgr.certStore {
		// If this node is not primary, it would commit pending transactions.
		if rbft.prepared(idx.d, idx.v, idx.n) && !cert.sentCommit {
			rbft.logger.Debugf("Replica %d try to commit batch %s", rbft.peerPool.localID, idx.d)
			_ = rbft.findNextCommitBatch(idx.d, idx.v, idx.n)
		}
	}
}
