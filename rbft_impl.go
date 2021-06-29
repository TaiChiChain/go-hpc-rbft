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
	"strconv"
	"sync"
	"time"

	"github.com/ultramesh/flato-common/metrics"
	"github.com/ultramesh/flato-common/types"
	"github.com/ultramesh/flato-common/types/protos"
	"github.com/ultramesh/flato-rbft/external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpool "github.com/ultramesh/flato-txpool"

	"github.com/gogo/protobuf/proto"
)

// Config contains the parameters to start a RAFT instance.
type Config struct {
	// ID is the num-identity of the local node. ID cannot be 0.
	ID uint64

	// Hash is the hash of the local node. Hash cannot be null.
	Hash string

	// Hostname is the hostname of the local node. Hostname cannot be nil.
	Hostname string

	// EpochInit is the epoch of such node init
	EpochInit uint64

	// LatestConfig is the state of the latest config batch
	// we are not sure if it was a stable checkpoint
	LatestConfig *pb.MetaState

	// peers contains the IDs and hashes of all nodes (including self) in the RBFT cluster. It
	// should be set when starting/restarting a RBFT cluster or after application layer has
	// finished add/delete node.
	// It's application's responsibility to ensure a consistent peer list among cluster.
	Peers []*pb.Peer

	// IsNew indicates if current node is a new joint node or not.
	IsNew bool

	// Applied is latest applied index of application service which should be assigned when node restart.
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

	// FetchCheckpointTimeout
	FetchCheckpointTimeout time.Duration

	// CheckPoolTimeout is the time duration one check for out-of-date requests in request pool cyclically.
	CheckPoolTimeout time.Duration

	// External is the application helper interfaces which must be implemented by application before
	// initializing RBFT service.
	External external.ExternalStack

	// RequestPool helps managing the requests, detect and discover duplicate requests.
	RequestPool txpool.TxPool

	// FlowControl indicates whether flow control has been opened.
	FlowControl bool

	// FlowControlMaxMem indicates the max memory size of txs in request set
	FlowControlMaxMem int

	// MetricsProv is the metrics Provider used to generate metrics instance.
	MetricsProv metrics.Provider

	// DelFlag is a channel to stop namespace when there is a non-recoverable error
	DelFlag chan bool

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

	recvChan chan interface{}        // channel to receive ordered consensus messages and local events
	cpChan   chan *pb.ServiceState   // channel to wait for local checkpoint event
	confChan chan *pb.ReloadFinished // channel to track config transaction execute
	delFlag  chan bool               // channel to stop namespace when there is a non-recoverable error
	close    chan bool               // channel to close this event process

	flowControl       bool // whether limit flow or not
	flowControlMaxMem int  // the max memory size of txs in request set

	reusableRequestBatch *pb.SendRequestBatch // special struct to reuse the biggest message in rbft.

	viewLock  sync.RWMutex // mutex to set value of view
	hLock     sync.RWMutex // mutex to set value of h
	epochLock sync.RWMutex // mutex to set value of view

	wg sync.WaitGroup // make sure the listener has been closed

	metrics *rbftMetrics // collect all metrics in rbft
	config  Config       // get configuration info
	logger  Logger       // write logger to record some info
}

var once sync.Once

// newRBFT init the RBFT instance
func newRBFT(cpChan chan *pb.ServiceState, confC chan *pb.ReloadFinished, c Config) (*rbftImpl, error) {
	var err error

	// init message event converter
	once.Do(initMsgEventMap)

	recvC := make(chan interface{}, 1)
	rbft := &rbftImpl{
		K:                    c.K,
		epoch:                c.EpochInit,
		config:               c,
		logger:               c.Logger,
		external:             c.External,
		storage:              c.External,
		recvChan:             recvC,
		cpChan:               cpChan,
		confChan:             confC,
		close:                make(chan bool),
		delFlag:              c.DelFlag,
		flowControl:          c.FlowControl,
		flowControlMaxMem:    c.FlowControlMaxMem,
		reusableRequestBatch: &pb.SendRequestBatch{},
	}

	if rbft.K == 0 {
		rbft.K = uint64(DefaultK)
	}

	N := len(c.Peers)
	// new node turn on isNewNode flag to start add node process after recovery.
	if c.IsNew {
		// new node excludes itself when start node until finish entering the cluster.
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
			return nil, fmt.Errorf("peers: %+v doesn't contain self Hash: %s", c.Peers, c.Hash)
		}
	}

	// new metrics instance
	rbft.metrics, err = newRBFTMetrics(c.MetricsProv)
	if err != nil {
		rbft.metrics.unregisterMetrics()
		rbft.metrics = nil
		return nil, err
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
	rbft.batchMgr = newBatchManager(c)

	// new recovery manager
	rbft.recoveryMgr = newRecoveryMgr(c)

	// new viewChange manager
	rbft.vcMgr = newVcManager(c)

	// new peer pool
	rbft.peerPool = newPeerPool(c)

	// restore state from consensus database
	// TODO(move to node interface?)
	rbft.exec.setLastExec(c.Applied)

	// new epoch manager
	rbft.epochMgr = newEpochManager(c)

	// update viewChange seqNo after restore state which may update seqNo
	rbft.updateViewChangeSeqNo(rbft.exec.lastExec, rbft.K)

	rbft.metrics.idGauge.Set(float64(rbft.peerPool.ID))
	rbft.metrics.epochGauge.Set(float64(rbft.epoch))
	rbft.metrics.clusterSizeGauge.Set(float64(rbft.N))
	rbft.metrics.quorumSizeGauge.Set(float64(rbft.commonCaseQuorum()))

	rbft.logger.Infof("RBFT Max number of validating peers (N) = %v", rbft.N)
	rbft.logger.Infof("RBFT Max number of failing peers (f) = %v", rbft.f)
	rbft.logger.Infof("RBFT byzantine flag = %v", rbft.in(byzantine))
	rbft.logger.Infof("RBFT Checkpoint period (K) = %v", rbft.K)
	rbft.logger.Infof("RBFT Log multiplier = %v", rbft.logMultiplier)
	rbft.logger.Infof("RBFT log size (L) = %v", rbft.L)
	rbft.logger.Infof("RBFT ID: %d", rbft.peerPool.ID)

	return rbft, nil
}

// start initializes and starts the consensus service
func (rbft *rbftImpl) start() error {
	var err error
	err = rbft.batchMgr.requestPool.Start()
	if err != nil {
		return err
	}

	// exit pending status after start rbft to avoid missing consensus messages from other nodes.
	rbft.atomicOff(Pending)
	rbft.metrics.statusGaugePending.Set(0)

	rbft.recoveryMgr.syncReceiver.Range(rbft.readMap)

	rbft.logger.Noticef("--------RBFT starting, nodeID: %d--------", rbft.peerPool.ID)

	if err := rbft.restoreState(); err != nil {
		rbft.logger.Errorf("Replica restore state failed: %s", err)
		return err
	}

	// if the stable-checkpoint recovered from consensus-database is equal to the config-batch-to-check,
	// current state has already been checked to be stable, and we need not to check it again
	metaS := &pb.MetaState{
		Applied: rbft.h,
		Digest:  rbft.storeMgr.chkpts[rbft.h],
	}
	if rbft.equalMetaState(rbft.epochMgr.configBatchToCheck, metaS) {
		rbft.logger.Info("Config batch to check has already been stable, reset it")
		rbft.epochMgr.configBatchToCheck = nil
	}

	// start listen consensus event
	go rbft.listenEvent()

	// NOTE!!! must use goroutine to post the event to avoid blocking the rbft service.
	// trigger recovery
	initRecoveryEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryInitEvent,
	}
	rbft.postMsg(initRecoveryEvent)

	return nil
}

// stop stops the consensus service
func (rbft *rbftImpl) stop() {
	rbft.logger.Notice("RBFT stopping...")

	rbft.atomicOn(Pending)
	rbft.atomicOn(Stopped)
	rbft.metrics.statusGaugePending.Set(Pending)
	rbft.setAbNormal()
	drainChannel(rbft.recvChan)

	// stop listen consensus event
	select {
	case <-rbft.close:
	default:
		rbft.logger.Notice("close RBFT event listener")
		close(rbft.close)
	}

	// stop all timer event
	rbft.timerMgr.Stop()
	rbft.logger.Notice("close RBFT timer manager")

	// stop txPool
	if rbft.batchMgr.requestPool != nil {
		rbft.batchMgr.requestPool.Stop()
	}
	rbft.logger.Notice("close TxPool")

	// unregister metrics
	if rbft.metrics != nil {
		rbft.metrics.unregisterMetrics()
	}

	rbft.logger.Notice("Waiting...")
	rbft.wg.Wait()
	rbft.logger.Noticef("RBFT stopped!")
}

// RecvMsg receives and processes messages from other peers
func (rbft *rbftImpl) step(msg *pb.ConsensusMessage) {
	if rbft.atomicIn(Pending) {
		if rbft.atomicIn(Stopped) {
			rbft.logger.Debugf("Replica %d is stopped, reject every consensus messages", rbft.peerPool.ID)
			return
		}

		switch msg.Type {
		case pb.Type_NOTIFICATION:
			n := &pb.Notification{}
			err := proto.Unmarshal(msg.Payload, n)
			if err != nil {
				rbft.logger.Errorf("Consensus Message Unmarshal error: can not unmarshal pb.Notification %v", err)
				return
			}
			id := ntfIdx{v: n.Basis.View, nodeID: n.NodeInfo.ReplicaId}
			rbft.recoveryMgr.syncReceiver.Store(id, n)
			rbft.logger.Debugf("Replica %d is in pending status, pre-store the notification message: "+
				"from replica %d, e:%d, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
				rbft.peerPool.ID, n.NodeInfo.ReplicaId, n.Epoch, n.Basis.View, n.Basis.H, len(n.Basis.Cset), len(n.Basis.Pset), len(n.Basis.Qset))
		default:
			rbft.logger.Debugf("Replica %d is in pending status, reject consensus messages", rbft.peerPool.ID)
		}

		return
	}

	// nolint: errcheck
	rbft.postMsg(msg)
}

// reportStateUpdated informs RBFT stateUpdated event.
func (rbft *rbftImpl) reportStateUpdated(state *pb.ServiceState) {
	if rbft.atomicIn(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, reject consensus messages", rbft.peerPool.ID)
		return
	}
	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event:     state,
	}

	go rbft.postMsg(event)
}

// postRequests informs RBFT tx set event which is posted from application layer.
func (rbft *rbftImpl) postRequests(requests *pb.RequestSet) {
	if rbft.atomicIn(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, reject consensus messages", rbft.peerPool.ID)
		return
	}
	rbft.postMsg(requests)
}

// postBatches informs RBFT batch event which is usually generated by request pool.
func (rbft *rbftImpl) postBatches(batches []*txpool.RequestHashBatch) {
	for _, batch := range batches {
		// if primary has generated a config batch, turn into config change mode
		// when primary is in config change, it won't generate any batches
		if rbft.batchMgr.requestPool.IsConfigBatch(batch.BatchHash) {
			rbft.logger.Noticef("Primary %d has generated a config batch, start config change", rbft.peerPool.ID)
			rbft.atomicOn(InConfChange)
			rbft.metrics.statusGaugeInConfChange.Set(InConfChange)
		}

		_ = rbft.recvRequestBatch(batch)
	}
}

// postConfState informs RBFT conf state event which is posted from application layer.
func (rbft *rbftImpl) postConfState(cc *pb.ConfState) {
	found := false
	for _, p := range cc.QuorumRouter.Peers {
		if p.Hash == rbft.peerPool.hash {
			found = true
		}
	}
	if !found {
		rbft.logger.Criticalf("%s cannot find self id in quorum routers: %+v", rbft.peerPool.hash, cc.QuorumRouter.Peers)
		rbft.atomicOn(Pending)
		rbft.metrics.statusGaugePending.Set(Pending)
		rbft.stopNamespace()
		return
	}
	rbft.peerPool.initPeers(cc.QuorumRouter.Peers)
	rbft.metrics.idGauge.Set(float64(rbft.peerPool.ID))
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

	rbft.epochLock.RLock()
	status.Epoch = rbft.epoch
	rbft.epochLock.RUnlock()

	status.ID = rbft.peerPool.ID
	switch {
	case rbft.atomicIn(InConfChange):
		status.Status = InConfChange
	case rbft.atomicIn(InViewChange):
		status.Status = InViewChange
	case rbft.atomicIn(InRecovery):
		status.Status = InRecovery
	case rbft.atomicIn(StateTransferring):
		status.Status = StateTransferring
	case rbft.isPoolFull():
		status.Status = PoolFull
	case rbft.atomicIn(Pending):
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
	rbft.wg.Add(1)
	defer rbft.wg.Done()
	for {
		select {
		case <-rbft.close:
			rbft.logger.Notice("exit RBFT event listener")
			return
		case obj := <-rbft.recvChan:
			var next consensusEvent
			var ok bool
			if next, ok = obj.(consensusEvent); !ok {
				rbft.logger.Error("Can't recognize event type")
				rbft.stopNamespace()
				return
			}
			for {
				select {
				case <-rbft.close:
					rbft.logger.Notice("exit RBFT event listener")
					return
				default:
					break
				}
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
		// e.Local indicates whether this RequestSet was generated locally or received
		// from remote nodes.
		if e.Local {
			rbft.metrics.incomingLocalTxSets.Add(float64(1))
			rbft.metrics.incomingLocalTxs.Add(float64(len(e.Requests)))
		} else {
			rbft.metrics.incomingRemoteTxSets.Add(float64(1))
			rbft.metrics.incomingRemoteTxs.Add(float64(len(e.Requests)))
		}

		rbft.processReqSetEvent(e)

		return nil

	case *LocalEvent:
		return rbft.dispatchLocalEvent(e)

	case *pb.ConsensusMessage:
		return rbft.consensusMessageFilter(e)

	default:
		rbft.logger.Errorf("Can't recognize event type of %v.", e)
		return nil
	}
}

func (rbft *rbftImpl) consensusMessageFilter(msg *pb.ConsensusMessage) consensusEvent {

	// A node in different epoch or in epoch sync will reject normal consensus messages, except:
	// For sync state: {SyncState, SyncStateResponse},
	// For checkpoint: {Checkpoint, FetchCheckpoint},
	if msg.Epoch != rbft.epoch {
		switch msg.Type {
		case pb.Type_NOTIFICATION,
			pb.Type_NOTIFICATION_RESPONSE,
			pb.Type_SYNC_STATE,
			pb.Type_SYNC_STATE_RESPONSE,
			pb.Type_CHECKPOINT,
			pb.Type_FETCH_CHECKPOINT:
		default:
			rbft.logger.Debugf("Replica %d in epoch %d reject msg from epoch %d",
				rbft.peerPool.ID, rbft.epoch, msg.Epoch)
			// if the consensus message comes from larger epoch and current node isn't a new node,
			// track the message for that current node might be out of epoch
			if msg.Epoch > rbft.epoch {
				rbft.checkIfOutOfEpoch(msg)
			}
			return nil
		}
	}

	next, err := rbft.msgToEvent(msg)
	if err != nil {
		return nil
	}
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

	if rbft.atomicIn(InRecovery) {
		rbft.logger.Infof("Replica %d is in recovery, reject null request from replica %d", rbft.peerPool.ID, msg.ReplicaId)
		return nil
	}

	if rbft.atomicIn(InViewChange) {
		rbft.logger.Infof("Replica %d is in viewChange, reject null request from replica %d", rbft.peerPool.ID, msg.ReplicaId)
		return nil
	}

	if !rbft.isPrimary(msg.ReplicaId) { // only primary could send a null request
		rbft.logger.Warningf("Replica %d received null request from replica %d who is not primary", rbft.peerPool.ID, msg.ReplicaId)
		return nil
	}
	// if receiver is not primary, stop firstRequestTimer started after this replica finished recovery
	rbft.stopFirstRequestTimer()

	rbft.logger.Infof("Replica %d received null request from primary %d", rbft.peerPool.ID, msg.ReplicaId)

	rbft.trySyncState()
	rbft.nullReqTimerReset()

	return nil
}

// handleNullRequestEvent triggered by null request timer, primary needs to send a null request
// and replica needs to send view change
func (rbft *rbftImpl) handleNullRequestTimerEvent() {

	if rbft.atomicIn(InRecovery) {
		rbft.logger.Debugf("Replica %d try to nullRequestHandler, but it's in recovery", rbft.peerPool.ID)
		return
	}

	if rbft.atomicIn(InViewChange) {
		rbft.logger.Debugf("Replica %d try to nullRequestHandler, but it's in viewChange", rbft.peerPool.ID)
		return
	}

	if !rbft.isPrimary(rbft.peerPool.ID) {
		// replica expects a null request, but primary never sent one
		rbft.logger.Warningf("Replica %d null request timer expired, sending viewChange", rbft.peerPool.ID)
		rbft.sendViewChange()
	} else {
		rbft.logger.Infof("Primary %d null request timer expired, sending null request", rbft.peerPool.ID)
		rbft.sendNullRequest()

		rbft.trySyncState()
	}
}

// sendNullRequest is for primary peer to send null when nullRequestTimer booms
func (rbft *rbftImpl) sendNullRequest() {

	nullRequest := &pb.NullRequest{
		ReplicaId: rbft.peerPool.ID,
	}
	payload, err := proto.Marshal(nullRequest)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_NULL_REQUEST Marshal Error: %s", err)
		return
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_NULL_REQUEST,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	rbft.nullReqTimerReset()
}

//=============================================================================
// process request set and batch methods
//=============================================================================

// processReqSetEvent process received requestSet event, reject txs in following situations:
// 1. pool is full, reject txs relayed from other nodes
// 2. node is in config change, reject another config tx
// 3. node is in skipInProgress, rejects any txs from other nodes or API
func (rbft *rbftImpl) processReqSetEvent(req *pb.RequestSet) consensusEvent {
	// if pool already full, rejects the tx, unless it's from RPC because of time difference or we have opened flow control
	if rbft.isPoolFull() && !req.Local && !rbft.flowControl {
		rbft.rejectRequestSet(req)
		return nil
	}

	// if current node is in config change, reject another config tx
	if rbft.atomicIn(InConfChange) {
		for _, tx := range req.Requests {
			if types.IsConfigTx(tx) {
				rbft.logger.Debugf("Replica %d is processing a ctx, reject another one", rbft.peerPool.ID)
				rbft.rejectRequestSet(req)
				return nil
			}
		}
	}

	// if current node is in skipInProgress, it cannot handle txs anymore
	if rbft.in(SkipInProgress) {
		rbft.rejectRequestSet(req)
		return nil
	}

	// if current node is in abnormal, add normal txs into txPool without generate batches.
	if !rbft.isNormal() {
		_, completionMissingBatchHashes := rbft.batchMgr.requestPool.AddNewRequests(req.Requests, false, req.Local)
		for _, batchHash := range completionMissingBatchHashes {
			delete(rbft.storeMgr.missingBatchesInFetching, batchHash)
		}
	} else {
		// primary nodes would check if this transaction triggered generating a batch or not
		if rbft.isPrimary(rbft.peerPool.ID) {
			// start batch timer when this node receives the first transaction of a batch
			if !rbft.batchMgr.isBatchTimerActive() {
				rbft.startBatchTimer()
			}
			batches, _ := rbft.batchMgr.requestPool.AddNewRequests(req.Requests, true, req.Local)
			// If these transactions triggers generating a batch, stop batch timer
			if len(batches) != 0 {
				rbft.stopBatchTimer()
				now := time.Now().UnixNano()
				if rbft.batchMgr.lastBatchTime != 0 {
					interval := time.Duration(now - rbft.batchMgr.lastBatchTime).Seconds()
					rbft.metrics.batchInterval.Observe(interval)
				}
				rbft.batchMgr.lastBatchTime = now
				rbft.postBatches(batches)
			}
		} else {
			_, completionMissingBatchHashes := rbft.batchMgr.requestPool.AddNewRequests(req.Requests, false, req.Local)
			for _, batchHash := range completionMissingBatchHashes {
				idx, ok := rbft.storeMgr.missingBatchesInFetching[batchHash]
				if !ok {
					rbft.logger.Warningf("Replica %d completion batch with hash %s but not found missing record",
						rbft.peerPool.ID, batchHash)
				} else {
					rbft.logger.Infof("Replica %d completion batch with hash %s, try to commit this batch",
						rbft.peerPool.ID, batchHash)
					_ = rbft.findNextCommitBatch(idx.d, idx.v, idx.n)
				}
				delete(rbft.storeMgr.missingBatchesInFetching, batchHash)
			}
		}
	}

	if rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setFull()
	}

	return nil
}

// rejectRequestSet rejects tx set and update related metrics.
func (rbft *rbftImpl) rejectRequestSet(req *pb.RequestSet) {
	if req.Local {
		rbft.metrics.rejectedLocalTxs.Add(float64(len(req.Requests)))
	} else {
		rbft.metrics.rejectedRemoteTxs.Add(float64(len(req.Requests)))
	}
	// recall promise to avoid memory leak.
	for _, r := range req.Requests {
		if r.Promise != nil {
			r.Promise.Recall()
		}
	}
}

// processOutOfDateReqs process the out-of-date requests in requestPool, get the remained txs from pool,
// then broadcast all the remained requests that generate by itself to others
func (rbft *rbftImpl) processOutOfDateReqs() {

	// if rbft is in abnormal, reject process remained reqs
	if !rbft.isNormal() {
		rbft.logger.Warningf("Replica %d is in abnormal, reject broadcast remained reqs", rbft.peerPool.ID)
		return
	}

	reqs, err := rbft.batchMgr.requestPool.FilterOutOfDateRequests()
	if err != nil {
		rbft.logger.Warningf("Replica %d get the remained reqs failed, error: %v", rbft.peerPool.ID, err)
	}

	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	reqLen := len(reqs)
	if reqLen == 0 {
		rbft.logger.Debugf("Replica %d in normal finds 0 remained reqs, need not broadcast to others", rbft.peerPool.ID)
		return
	}

	setSize := rbft.config.SetSize
	rbft.logger.Debugf("Replica %d in normal finds %d remained reqs, broadcast to others split by setSize %d if needed", rbft.peerPool.ID, reqLen, setSize)

	// limit TransactionSet Max Mem by flowControlMaxMem before re-broadcast reqs
	if rbft.flowControl {
		var txs []*protos.Transaction
		memLen := 0
		for _, tx := range reqs {
			txMem := tx.Size()
			if memLen+txMem >= rbft.flowControlMaxMem && len(txs) > 0 {
				set := &pb.RequestSet{Requests: txs}
				rbft.broadcastReqSet(set)
				txs = nil
				memLen = 0
			}
			txs = append(txs, tx)
			memLen += txMem
		}
		if len(txs) > 0 {
			set := &pb.RequestSet{Requests: txs}
			rbft.broadcastReqSet(set)
		}
		return
	}

	// limit TransactionSet size by setSize before re-broadcast reqs
	for reqLen > 0 {
		if reqLen <= setSize {
			set := &pb.RequestSet{Requests: reqs}
			rbft.broadcastReqSet(set)
			reqLen = 0
		} else {
			bTxs := reqs[0:setSize]
			set := &pb.RequestSet{Requests: bTxs}
			rbft.broadcastReqSet(set)
			reqs = reqs[setSize:]
			reqLen -= setSize
		}
	}
}

// recvRequestBatch handle logic after receive request batch
func (rbft *rbftImpl) recvRequestBatch(reqBatch *txpool.RequestHashBatch) error {

	rbft.logger.Debugf("Replica %d received request batch %s", rbft.peerPool.ID, reqBatch.BatchHash)

	batch := &pb.RequestBatch{
		RequestHashList: reqBatch.TxHashList,
		RequestList:     reqBatch.TxList,
		Timestamp:       reqBatch.Timestamp,
		LocalList:       reqBatch.LocalList,
		BatchHash:       reqBatch.BatchHash,
	}

	if rbft.isPrimary(rbft.peerPool.ID) && rbft.isNormal() {
		rbft.restartBatchTimer()
		rbft.timerMgr.stopTimer(nullRequestTimer)
		if len(rbft.batchMgr.cacheBatch) > 0 {
			rbft.batchMgr.cacheBatch = append(rbft.batchMgr.cacheBatch, batch)
			rbft.metrics.cacheBatchNumber.Add(float64(1))
			rbft.maybeSendPrePrepare(nil, true)
			return nil
		}
		rbft.maybeSendPrePrepare(batch, false)
	} else {
		rbft.logger.Debugf("Replica %d is backup, not try to send prePrepare for request batch %s", rbft.peerPool.ID, reqBatch.BatchHash)
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
		"batch size: %d, timestamp: %d", rbft.peerPool.ID, rbft.view, seqNo, digest, len(reqBatch.RequestHashList), reqBatch.Timestamp)

	hashBatch := &pb.HashBatch{
		RequestHashList: reqBatch.RequestHashList,
		Timestamp:       reqBatch.Timestamp,
	}

	preprepare := &pb.PrePrepare{
		View:           rbft.view,
		SequenceNumber: seqNo,
		BatchDigest:    digest,
		HashBatch:      hashBatch,
		ReplicaId:      rbft.peerPool.ID,
	}

	cert := rbft.storeMgr.getCert(rbft.view, seqNo, digest)
	cert.prePrepare = preprepare
	rbft.persistQSet(preprepare)
	if metrics.EnableExpensive() {
		cert.prePreparedTime = time.Now().UnixNano()
		duration := time.Duration(cert.prePreparedTime - reqBatch.Timestamp).Seconds()
		rbft.metrics.batchToPrePrepared.Observe(duration)
	}

	payload, err := proto.Marshal(preprepare)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_PRE_PREPARE Marshal Error: %s", err)
		return
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_PRE_PREPARE,
		From:    rbft.peerPool.ID,
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
		rbft.peerPool.ID, preprep.ReplicaId, preprep.View, preprep.SequenceNumber, preprep.BatchDigest)

	if !rbft.isPrePrepareLegal(preprep) {
		return nil
	}

	digest, ok := rbft.storeMgr.seqMap[preprep.SequenceNumber]
	if ok {
		if digest != preprep.BatchDigest {
			rbft.logger.Warningf("Replica %d found same view/seqNo but different digest, received: %s, stored: %s",
				rbft.peerPool.ID, preprep.BatchDigest, digest)
			rbft.sendViewChange()
			return nil
		}
	}

	if rbft.beyondRange(preprep.SequenceNumber) {
		rbft.logger.Debugf("Replica %d received a pre-prepare out of high-watermark", rbft.peerPool.ID)
		rbft.softStartHighWatermarkTimer("replica received a pre-prepare out of range")
	}

	if preprep.BatchDigest == "" {
		if len(preprep.HashBatch.RequestHashList) != 0 {
			rbft.logger.Warningf("Replica %d received a prePrepare with an empty digest but batch is "+
				"not empty", rbft.peerPool.ID)
			rbft.sendViewChange()
			return nil
		}
	} else {
		if len(preprep.HashBatch.DeDuplicateRequestHashList) != 0 {
			rbft.logger.Noticef("Replica %d finds %d duplicate txs with digest %s, detailed: %+v",
				rbft.peerPool.ID, len(preprep.HashBatch.DeDuplicateRequestHashList), preprep.HashBatch.DeDuplicateRequestHashList)
		}
		// check if the digest sent from primary is really the hash of txHashList, if not, don't
		// send prepare for this prePrepare
		digest := calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)
		if digest != preprep.BatchDigest {
			rbft.logger.Warningf("Replica %d received a prePrepare with a wrong batch digest, calculated: %s "+
				"primary calculated: %s, send viewChange", rbft.peerPool.ID, digest, preprep.BatchDigest)
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
	cert.prePrepare = preprep
	rbft.storeMgr.seqMap[preprep.SequenceNumber] = preprep.BatchDigest
	if metrics.EnableExpensive() {
		cert.prePreparedTime = time.Now().UnixNano()
		duration := time.Duration(cert.prePreparedTime - preprep.HashBatch.Timestamp).Seconds()
		rbft.metrics.batchToPrePrepared.Observe(duration)
	}

	if !rbft.in(SkipInProgress) && preprep.SequenceNumber > rbft.exec.lastExec {
		rbft.softStartNewViewTimer(rbft.timerMgr.getTimeoutValue(requestTimer),
			fmt.Sprintf("new prePrepare for request batch view=%d/seqNo=%d, hash=%s",
				preprep.View, preprep.SequenceNumber, preprep.BatchDigest), false)

		// exit sync state as we start process requests now.
		rbft.exitSyncState()
	}

	rbft.persistQSet(preprep)

	if !rbft.isPrimary(rbft.peerPool.ID) && !cert.sentPrepare &&
		rbft.prePrepared(preprep.BatchDigest, preprep.View, preprep.SequenceNumber) {
		cert.sentPrepare = true
		return rbft.sendPrepare(preprep)
	}

	return nil
}

// sendPrepare send prepare message.
func (rbft *rbftImpl) sendPrepare(preprep *pb.PrePrepare) error {
	rbft.logger.Debugf("Replica %d sending prepare for view=%d/seqNo=%d", rbft.peerPool.ID, preprep.View, preprep.SequenceNumber)
	prep := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      rbft.peerPool.ID,
	}
	payload, err := proto.Marshal(prep)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_PREPARE Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_PREPARE,
		From:    rbft.peerPool.ID,
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
		rbft.peerPool.ID, prep.ReplicaId, prep.View, prep.SequenceNumber)

	if !rbft.isPrepareLegal(prep) {
		return nil
	}

	cert := rbft.storeMgr.getCert(prep.View, prep.SequenceNumber, prep.BatchDigest)
	ok := cert.prepare[*prep]

	if ok {
		if prep.SequenceNumber <= rbft.exec.lastExec {
			rbft.logger.Debugf("Replica %d received duplicate prepare from replica %d, view=%d/seqNo=%d, self lastExec=%d",
				rbft.peerPool.ID, prep.ReplicaId, prep.View, prep.SequenceNumber, rbft.exec.lastExec)
			return nil
		}
		// this is abnormal in common case
		rbft.logger.Infof("Replica %d ignore duplicate prepare from replica %d, view=%d/seqNo=%d",
			rbft.peerPool.ID, prep.ReplicaId, prep.View, prep.SequenceNumber)
		return nil
	}

	cert.prepare[*prep] = true

	return rbft.maybeSendCommit(prep.BatchDigest, prep.View, prep.SequenceNumber)
}

// maybeSendCommit check if we could send commit. if no problem,
// primary and replica would send commit.
func (rbft *rbftImpl) maybeSendCommit(digest string, v uint64, n uint64) error {

	if rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d do not try to send commit because it's in stateUpdate", rbft.peerPool.ID)
		return nil
	}

	cert := rbft.storeMgr.getCert(v, n, digest)
	if cert == nil {
		rbft.logger.Errorf("Replica %d can't get the cert for the view=%d/seqNo=%d/digest=%s", rbft.peerPool.ID, v, n, digest)
		return nil
	}

	if !rbft.prepared(digest, v, n) {
		return nil
	}

	if cert.sentCommit {
		return nil
	}

	if metrics.EnableExpensive() {
		cert.preparedTime = time.Now().UnixNano()
		duration := time.Duration(cert.preparedTime - cert.prePreparedTime).Seconds()
		rbft.metrics.prePreparedToPrepared.Observe(duration)
	}

	if rbft.isPrimary(rbft.peerPool.ID) {
		return rbft.sendCommit(digest, v, n)
	}

	return rbft.findNextCommitBatch(digest, v, n)
}

// sendCommit send commit message.
func (rbft *rbftImpl) sendCommit(digest string, v uint64, n uint64) error {
	cert := rbft.storeMgr.getCert(v, n, digest)

	rbft.logger.Debugf("Replica %d sending commit for view=%d/seqNo=%d", rbft.peerPool.ID, v, n)
	commit := &pb.Commit{
		View:           v,
		SequenceNumber: n,
		BatchDigest:    digest,
		ReplicaId:      rbft.peerPool.ID,
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
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	return rbft.recvCommit(commit)
}

// recvCommit process logic after receive commit message.
func (rbft *rbftImpl) recvCommit(commit *pb.Commit) error {
	rbft.logger.Debugf("Replica %d received commit from replica %d for view=%d/seqNo=%d",
		rbft.peerPool.ID, commit.ReplicaId, commit.View, commit.SequenceNumber)

	if !rbft.isCommitLegal(commit) {
		return nil
	}

	cert := rbft.storeMgr.getCert(commit.View, commit.SequenceNumber, commit.BatchDigest)

	ok := cert.commit[*commit]

	if ok {
		if commit.SequenceNumber <= rbft.exec.lastExec {
			// ignore duplicate commit with seqNo <= lastExec as this commit is not useful forever.
			rbft.logger.Debugf("Replica %d received duplicate commit from replica %d, view=%d/seqNo=%d "+
				"but current lastExec is %d, ignore it...", rbft.peerPool.ID, commit.ReplicaId, commit.View, commit.SequenceNumber,
				rbft.exec.lastExec)
			return nil
		}
		// we can simply accept all commit messages whose seqNo is larger than our lastExec as we
		// haven't execute this batch and we can ensure that we will only execute this batch once.
		rbft.logger.Debugf("Replica %d accept duplicate commit from replica %d, view=%d/seqNo=%d "+
			"current lastExec is %d", rbft.peerPool.ID, commit.ReplicaId, commit.View, commit.SequenceNumber, rbft.exec.lastExec)
	}

	cert.commit[*commit] = true

	if rbft.committed(commit.BatchDigest, commit.View, commit.SequenceNumber) {
		idx := msgID{v: commit.View, n: commit.SequenceNumber, d: commit.BatchDigest}
		if metrics.EnableExpensive() {
			cert.committedTime = time.Now().UnixNano()
			duration := time.Duration(cert.committedTime - cert.preparedTime).Seconds()
			rbft.metrics.preparedToCommitted.Observe(duration)
		}
		if !cert.sentExecute && cert.sentCommit {
			rbft.storeMgr.committedCert[idx] = commit.BatchDigest
			rbft.commitPendingBlocks()

			// reset last new view timeout after commit one block successfully.
			rbft.vcMgr.lastNewViewTimeout = rbft.timerMgr.getTimeoutValue(newViewTimer)
			if commit.SequenceNumber == rbft.vcMgr.viewChangeSeqNo {
				rbft.logger.Warningf("Replica %d cycling view for seqNo=%d", rbft.peerPool.ID, commit.SequenceNumber)
				rbft.sendViewChange()
			}
		} else {
			rbft.logger.Debugf("Replica %d committed for seqNo: %d, but sentExecute: %v", rbft.peerPool.ID, commit.SequenceNumber, cert.sentExecute)
		}
	}
	return nil
}

// fetchMissingTxs fetch missing txs from primary which this node didn't receive but primary received
func (rbft *rbftImpl) fetchMissingTxs(prePrep *pb.PrePrepare, missingTxHashes map[uint64]string) error {
	// avoid fetch the same batch again.
	if _, ok := rbft.storeMgr.missingBatchesInFetching[prePrep.BatchDigest]; ok {
		return nil
	}

	rbft.logger.Debugf("Replica %d try to fetch missing txs for view=%d/seqNo=%d/digest=%s from primary %d",
		rbft.peerPool.ID, prePrep.View, prePrep.SequenceNumber, prePrep.BatchDigest, prePrep.ReplicaId)

	fetch := &pb.FetchMissingRequests{
		View:                 prePrep.View,
		SequenceNumber:       prePrep.SequenceNumber,
		BatchDigest:          prePrep.BatchDigest,
		MissingRequestHashes: missingTxHashes,
		ReplicaId:            rbft.peerPool.ID,
	}

	payload, err := proto.Marshal(fetch)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_FETCH_MISSING_TXS Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_FETCH_MISSING_REQUESTS,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.metrics.fetchMissingTxsCounter.Add(float64(1))
	rbft.storeMgr.missingBatchesInFetching[prePrep.BatchDigest] = msgID{
		v: prePrep.View,
		n: prePrep.SequenceNumber,
		d: prePrep.BatchDigest,
	}
	rbft.peerPool.unicast(consensusMsg, prePrep.ReplicaId)
	return nil
}

// recvFetchMissingTxs returns txs to a node which didn't receive some txs and ask primary for them.
func (rbft *rbftImpl) recvFetchMissingTxs(fetch *pb.FetchMissingRequests) error {
	rbft.logger.Debugf("Primary %d received fetchMissingTxs request for view=%d/seqNo=%d/digest=%s from replica %d",
		rbft.peerPool.ID, fetch.View, fetch.SequenceNumber, fetch.BatchDigest, fetch.ReplicaId)

	requests := make(map[uint64]*protos.Transaction)
	var err error

	if batch := rbft.storeMgr.batchStore[fetch.BatchDigest]; batch != nil {
		batchLen := uint64(len(batch.RequestHashList))
		for i, hash := range fetch.MissingRequestHashes {
			if i >= batchLen || batch.RequestHashList[i] != hash {
				rbft.logger.Errorf("Primary %d finds mismatch requests hash when return fetch missing requests", rbft.peerPool.ID)
				return nil
			}
			requests[i] = batch.RequestList[i]
		}
	} else {
		var missingTxs map[uint64]*protos.Transaction
		missingTxs, err = rbft.batchMgr.requestPool.SendMissingRequests(fetch.BatchDigest, fetch.MissingRequestHashes)
		if err != nil {
			rbft.logger.Warningf("Primary %d cannot find the digest %s, missing tx hashes: %+v, err: %s",
				rbft.peerPool.ID, fetch.BatchDigest, fetch.MissingRequestHashes, err)
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
		ReplicaId:            rbft.peerPool.ID,
	}

	defer func() {
		if r := recover(); r != nil {
			rbft.logger.Warningf("Primary %d finds marshal error: %v in marshaling SendMissingTxs, retry.", rbft.peerPool.ID, r)
			localEvent := &LocalEvent{
				Service:   CoreRbftService,
				EventType: CoreResendMissingTxsEvent,
				Event:     fetch,
			}
			if rbft.atomicIn(Pending) {
				rbft.logger.Debugf("Replica %d is in pending status, reject consensus messages", rbft.peerPool.ID)
				return
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
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.metrics.returnFetchMissingTxsCounter.With("node", strconv.Itoa(int(fetch.ReplicaId))).Add(float64(1))
	rbft.peerPool.unicast(consensusMsg, fetch.ReplicaId)

	return nil
}

// recvSendMissingTxs processes SendMissingTxs from primary.
// Add these transaction txs to requestPool and see if it has correct transaction txs.
func (rbft *rbftImpl) recvSendMissingTxs(re *pb.SendMissingRequests) consensusEvent {
	if _, ok := rbft.storeMgr.missingBatchesInFetching[re.BatchDigest]; !ok {
		rbft.logger.Warningf("Replica %d ignore return missing txs with batch hash %s",
			rbft.peerPool.ID, re.BatchDigest)
		return nil
	}

	rbft.logger.Debugf("Replica %d received sendMissingTxs for view=%d/seqNo=%d/digest=%s from replica %d",
		rbft.peerPool.ID, re.View, re.SequenceNumber, re.BatchDigest, re.ReplicaId)

	if re.SequenceNumber < rbft.exec.lastExec {
		rbft.logger.Debugf("Replica %d ignore return missing tx with lower seqNo %d than "+
			"lastExec %d", rbft.peerPool.ID, re.SequenceNumber, rbft.exec.lastExec)
		return nil
	}

	if len(re.MissingRequests) != len(re.MissingRequestHashes) {
		rbft.logger.Warningf("Replica %d received mismatch length return %v", rbft.peerPool.ID, re)
		return nil
	}

	if !rbft.inV(re.View) {
		rbft.logger.Debugf("Replica %d received return missing transactions which has a different view=%d, "+
			"expected view=%d, ignore it", rbft.peerPool.ID, re.View, rbft.view)
		return nil
	}

	cert := rbft.storeMgr.getCert(re.View, re.SequenceNumber, re.BatchDigest)
	if cert.sentCommit {
		rbft.logger.Debugf("Replica %d received return missing transactions which has been committed with "+
			"batch seqNo=%d, ignore it", rbft.peerPool.ID, re.SequenceNumber)
		return nil
	}
	if cert.prePrepare == nil {
		rbft.logger.Warningf("Replica %d had not received a prePrepare before for view=%d/seqNo=%d",
			rbft.peerPool.ID, re.View, re.SequenceNumber)
		return nil
	}

	err := rbft.batchMgr.requestPool.ReceiveMissingRequests(re.BatchDigest, re.MissingRequests)
	if err != nil {
		rbft.logger.Warningf("Replica %d find something wrong with the return of missing txs, error: %v",
			rbft.peerPool.ID, err)
		return nil
	}

	// set pool full status if received txs fill up the txpool.
	if rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setFull()
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
			rbft.peerPool.ID, rbft.exec.currentExec)
	}
	rbft.logger.Debugf("Replica %d attempting to commitTransactions", rbft.peerPool.ID)

	for hasTxToExec := true; hasTxToExec; {
		if find, idx, cert := rbft.findNextCommitTx(); find {
			rbft.metrics.committedBlockNumber.Add(float64(1))
			rbft.persistCSet(idx.v, idx.n, idx.d)
			// stop new view timer after one batch has been call executed
			rbft.stopNewViewTimer()
			if ok := rbft.isPrimary(rbft.peerPool.ID); ok {
				rbft.softRestartBatchTimer()
			}

			isConfig := rbft.batchMgr.requestPool.IsConfigBatch(idx.d)
			if idx.d == "" {
				txList := make([]*protos.Transaction, 0)
				localList := make([]bool, 0)
				rbft.metrics.committedEmptyBlockNumber.Add(float64(1))
				rbft.metrics.txsPerBlock.Observe(float64(0))
				rbft.logger.Noticef("======== Replica %d Call execute a no-nop, view=%d/seqNo=%d",
					rbft.peerPool.ID, idx.v, idx.n)
				rbft.external.Execute(txList, localList, idx.n, 0)
			} else {
				// find batch in batchStore rather than outstandingBatch as after viewChange
				// we may clear outstandingBatch and save all batches in batchStore.
				// kick out de-duplicate txs if needed.
				if isConfig {
					rbft.logger.Debugf("Replica %d found a config batch, set epoch start seq no %d",
						rbft.peerPool.ID, idx.n)
					rbft.setConfigBatchToExecute(idx.n)
					rbft.atomicOn(InConfChange)
					rbft.metrics.statusGaugeInConfChange.Set(InConfChange)
					rbft.metrics.committedConfigBlockNumber.Add(float64(1))
				}
				txList, localList := rbft.filterExecutableTxs(idx.d, cert.prePrepare.HashBatch.DeDuplicateRequestHashList)
				rbft.metrics.committedTxs.Add(float64(len(txList)))
				rbft.metrics.txsPerBlock.Observe(float64(len(txList)))
				batchToCommit := time.Duration(time.Now().UnixNano() - cert.prePrepare.HashBatch.Timestamp).Seconds()
				rbft.metrics.batchToCommitDuration.Observe(batchToCommit)
				rbft.logger.Noticef("======== Replica %d Call execute, view=%d/seqNo=%d/txCount=%d/digest=%s",
					rbft.peerPool.ID, idx.v, idx.n, len(txList), idx.d)
				rbft.external.Execute(txList, localList, idx.n, cert.prePrepare.HashBatch.Timestamp)
			}
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			rbft.metrics.outstandingBatchesGauge.Set(float64(len(rbft.storeMgr.outstandingReqBatches)))
			cert.sentExecute = true

			// if it is a config batch, start to wait for reload after the batch committed
			rbft.afterCommitBlock(idx, isConfig)
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
			rbft.logger.Noticef("Replica %d kick out de-duplicate request %s before execute batch %s", rbft.peerPool.ID, reqHash, digest)
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
			rbft.logger.Debugf("Replica %d already checkpoint for view=%d/seqNo=%d", rbft.peerPool.ID, idx.v, idx.n)
			continue
		}

		// check if already executed
		if cert.sentExecute == true {
			rbft.logger.Debugf("Replica %d already execute for view=%d/seqNo=%d", rbft.peerPool.ID, idx.v, idx.n)
			continue
		}

		if idx.n != rbft.exec.lastExec+1 {
			rbft.logger.Debugf("Replica %d expects to execute seq=%d, but get seq=%d, ignore it", rbft.peerPool.ID, rbft.exec.lastExec+1, idx.n)
			continue
		}

		// skipInProgress == true, then this replica is in viewchange, not reply or execute
		if rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d currently picking a starting point to resume, will not execute", rbft.peerPool.ID)
			continue
		}

		// check if committed
		if !rbft.committed(idx.d, idx.v, idx.n) {
			continue
		}

		if idx.d != "" {
			_, ok := rbft.storeMgr.batchStore[idx.d]
			if !ok {
				rbft.logger.Warningf("Replica %d cannot find corresponding batch %s in batchStore", rbft.peerPool.ID, idx.d)
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
func (rbft *rbftImpl) afterCommitBlock(idx msgID, isConfig bool) {
	if rbft.exec.currentExec != nil {
		rbft.logger.Debugf("Replica %d finished execution %d, trying next", rbft.peerPool.ID, *rbft.exec.currentExec)
		rbft.exec.setLastExec(*rbft.exec.currentExec)
		delete(rbft.storeMgr.committedCert, idx)

		// after committed block, there are 3 cases:
		// 1. a config transaction: waiting for checkpoint channel and turn into epoch process
		// 2. a normal transaction in checkpoint: waiting for checkpoint channel and turn into checkpoint process
		// 3. a normal transaction not in checkpoint: finish directly
		if isConfig {
			state, ok := <-rbft.cpChan
			if !ok {
				rbft.logger.Info("checkpoint channel closed")
				return
			}

			// reset config transaction to execute
			rbft.resetConfigBatchToExecute()

			if state.MetaState.Applied == rbft.exec.lastExec {
				rbft.logger.Debugf("Call the checkpoint for config batch, seqNo=%d", rbft.exec.lastExec)
				rbft.epochMgr.configBatchToCheck = state.MetaState
				rbft.checkpoint(state.MetaState)
			} else {
				// reqBatch call execute but have not done with execute
				rbft.logger.Errorf("Fail to call the checkpoint, seqNo=%d", rbft.exec.lastExec)
			}

		} else if rbft.exec.lastExec%rbft.K == 0 {
			state, ok := <-rbft.cpChan
			if !ok {
				rbft.logger.Info("checkpoint channel closed")
				return
			}

			if state.MetaState.Applied == rbft.exec.lastExec {
				rbft.logger.Debugf("Call the checkpoint for normal, seqNo=%d", rbft.exec.lastExec)
				rbft.checkpoint(state.MetaState)
			} else {
				// reqBatch call execute but have not done with execute
				rbft.logger.Errorf("Fail to call the checkpoint, seqNo=%d", rbft.exec.lastExec)
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d had execDoneSync called, flagging ourselves as out of data", rbft.peerPool.ID)
		rbft.on(SkipInProgress)
	}

	rbft.exec.currentExec = nil
}

//=============================================================================
// gc: checkpoint issues
//=============================================================================

// checkpoint generate a checkpoint and broadcast it to outer.
func (rbft *rbftImpl) checkpoint(state *pb.MetaState) {

	digest := state.Digest
	seqNo := state.Applied

	rbft.logger.Infof("Replica %d sending checkpoint for view=%d/seqNo=%d and digest=%s",
		rbft.peerPool.ID, rbft.view, seqNo, digest)

	chkpt := &pb.Checkpoint{
		SequenceNumber: seqNo,
		Digest:         digest,
		NodeInfo:       rbft.getNodeInfo(),
	}
	rbft.storeMgr.saveCheckpoint(seqNo, digest)
	rbft.persistCheckpoint(seqNo, []byte(digest))

	if rbft.epochMgr.configBatchToCheck != nil {
		// use fetchCheckpointTimer to fetch the missing checkpoint
		rbft.startFetchCheckpointTimer()
	} else {
		// if our lastExec is equal to high watermark, it means there is something wrong with checkpoint procedure, so that
		// we need to start a high-watermark timer for checkpoint, and trigger view-change when high-watermark timer expired
		if rbft.exec.lastExec == rbft.h+rbft.L {
			rbft.logger.Warningf("Replica %d try to send checkpoint equal to high watermark, "+
				"there may be something wrong with checkpoint", rbft.peerPool.ID)
			rbft.softStartHighWatermarkTimer("replica send checkpoint equal to high-watermark")
		}
	}
	payload, err := proto.Marshal(chkpt)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_CHECKPOINT Marshal Error: %s", err)
		return
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_CHECKPOINT,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	rbft.recvCheckpoint(chkpt)
}

// recvCheckpoint processes logic after receive checkpoint.
func (rbft *rbftImpl) recvCheckpoint(chkpt *pb.Checkpoint) consensusEvent {
	rbft.logger.Debugf("Replica %d received checkpoint from replica %d, seqNo %d, digest %s",
		rbft.peerPool.ID, chkpt.NodeInfo.ReplicaId, chkpt.SequenceNumber, chkpt.Digest)

	if rbft.in(isNewNode) {
		rbft.logger.Debug("A new node reject checkpoint messages")
		return nil
	}

	if rbft.weakCheckpointSetOutOfRange(chkpt) {
		return rbft.restartRecovery()
	}

	legal, matching := rbft.compareCheckpointWithWeakSet(chkpt)
	if !legal {
		rbft.logger.Debugf("Replica %d ignore illegal checkpoint from replica %d, seqNo=%d", rbft.peerPool.ID, chkpt.NodeInfo.ReplicaId, chkpt.SequenceNumber)
		return nil
	}

	rbft.logger.Debugf("Replica %d found %d matching checkpoints for seqNo %d, digest %s",
		rbft.peerPool.ID, matching, chkpt.SequenceNumber, chkpt.Digest)

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
			rbft.peerPool.ID, chkpt.SequenceNumber, chkpt.Digest)

		// update transferring target for state update, in order to trigger another state-update instance at the
		// moment the previous one has finished.
		target := &pb.MetaState{Applied: chkpt.SequenceNumber, Digest: chkpt.Digest}
		rbft.updateHighStateTarget(target)
		return nil
	}

	// the checkpoint is trigger by config batch
	if rbft.epochMgr.configBatchToCheck != nil {
		return rbft.finishConfigCheckpoint(chkpt)
	}

	return rbft.finishNormalCheckpoint(chkpt)
}

func (rbft *rbftImpl) finishConfigCheckpoint(chkpt *pb.Checkpoint) consensusEvent {
	if chkpt.SequenceNumber != rbft.epochMgr.configBatchToCheck.Applied {
		rbft.logger.Warningf("Replica %d received a non-expected checkpoint for config batch, "+
			"received %d, expected %d", chkpt.SequenceNumber, rbft.epochMgr.configBatchToCheck.Applied)
		return nil
	}
	rbft.stopFetchCheckpointTimer()

	rbft.logger.Infof("Replica %d found config checkpoint quorum for seqNo %d, digest %s",
		rbft.peerPool.ID, chkpt.SequenceNumber, chkpt.Digest)

	rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d after "+
		"executed to the height with the same digest", rbft.peerPool.ID, chkpt.SequenceNumber)
	rbft.external.SendFilterEvent(pb.InformType_FilterStableCheckpoint, chkpt.SequenceNumber, chkpt.Digest)
	rbft.epochMgr.configBatchToCheck = nil

	// waiting for commit db finish the reload
	if rbft.atomicIn(InConfChange) {
		rbft.logger.Noticef("Replica %d is waiting for commit-db finished...", rbft.peerPool.ID)
		ev, ok := <-rbft.confChan
		if !ok {
			rbft.logger.Info("Config Channel Closed")
			return nil
		}
		rbft.logger.Debugf("Replica %d received a commit-db finished target at height %d", rbft.peerPool.ID, ev.Height)
		if ev.Height != chkpt.SequenceNumber {
			rbft.logger.Errorf("Wrong commit-db height: %d", ev.Height)
			rbft.stopNamespace()
			return nil
		}
	} else {
		rbft.logger.Warningf("Replica %d isn't in config-change", rbft.peerPool.ID)
	}

	// two types of config transaction:
	// 1. validator set changed: try to start a new epoch
	// 2. validator set not changed: do not start a new epoch
	routerInfo := rbft.node.readReloadRouter()
	if routerInfo != nil {
		rbft.turnIntoEpoch(routerInfo, chkpt.SequenceNumber)
	}

	// move watermark for config batch
	rbft.moveWatermarks(chkpt.SequenceNumber)

	// finish config change and restart consensus
	rbft.atomicOff(InConfChange)
	rbft.metrics.statusGaugeInConfChange.Set(0)
	rbft.maybeSetNormal()
	rbft.startTimerIfOutstandingRequests()
	finishMsg := fmt.Sprintf("======== Replica %d finished config change, primary=%d, epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.peerPool.ID, rbft.primaryID(rbft.view), rbft.epoch, rbft.N, rbft.f, rbft.view, rbft.h, rbft.exec.lastExec)
	rbft.external.SendFilterEvent(pb.InformType_FilterFinishConfigChange, finishMsg)

	// set primary sequence log
	if rbft.isPrimary(rbft.peerPool.ID) {
		// set seqNo to lastExec for new primary to sort following batches from correct seqNo.
		rbft.batchMgr.setSeqNo(rbft.exec.lastExec)

		// resubmit transactions
		rbft.primaryResubmitTransactions()
	} else {
		rbft.fetchRecoveryPQC()
	}

	return nil
}

func (rbft *rbftImpl) finishNormalCheckpoint(chkpt *pb.Checkpoint) consensusEvent {
	rbft.stopFetchCheckpointTimer()
	rbft.stopHighWatermarkTimer()

	rbft.logger.Infof("Replica %d found normal checkpoint quorum for seqNo %d, digest %s",
		rbft.peerPool.ID, chkpt.SequenceNumber, chkpt.Digest)

	rbft.moveWatermarks(chkpt.SequenceNumber)
	rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d after "+
		"executed to the height with the same digest", rbft.peerPool.ID, rbft.h)
	rbft.external.SendFilterEvent(pb.InformType_FilterStableCheckpoint, chkpt.SequenceNumber, chkpt.Digest)

	// make sure node is in normal status before try to batch, as we may reach stable
	// checkpoint in vc/recovery.
	if rbft.isNormal() {
		// for primary, we can try resubmit transactions after stable checkpoint as we
		// may block pre-prepare before because of high watermark limit.
		rbft.primaryResubmitTransactions()
	}
	return nil
}

// weakCheckpointSetOutOfRange checks if this node is fell behind or not. If we receive f+1 checkpoints whose seqNo > H (for example 150),
// it is possible that we has fell behind, and will trigger recovery, but if we has already executed blocks larger than these
// checkpoints, we would like to start high-watermark timer in order to get the latest stable checkpoint.
func (rbft *rbftImpl) weakCheckpointSetOutOfRange(chkpt *pb.Checkpoint) bool {
	H := rbft.h + rbft.L

	// Track the last observed checkpoint sequence number if it exceeds our high watermark, keyed by replica to prevent unbounded growth
	if chkpt.SequenceNumber < H {
		// For non-byzantine nodes, the checkpoint sequence number increases monotonically
		delete(rbft.storeMgr.hChkpts, chkpt.NodeInfo.ReplicaHash)
	} else {
		// We do not track the highest one, as a byzantine node could pick an arbitrarily high sequence number
		// and even if it recovered to be non-byzantine, we would still believe it to be far ahead
		rbft.storeMgr.hChkpts[chkpt.NodeInfo.ReplicaHash] = &pb.MetaState{Applied: chkpt.SequenceNumber, Digest: chkpt.Digest}
		rbft.logger.Debugf("Replica %d received a checkpoint out of range from replica %d, seq %d",
			rbft.peerPool.ID, chkpt.NodeInfo.ReplicaId, chkpt.SequenceNumber)

		// If f+1 other replicas have reported checkpoints that were (at one time) outside our watermarks
		// we need to check to see if we have fallen behind.
		if len(rbft.storeMgr.hChkpts) >= rbft.oneCorrectQuorum() {
			var chkptSeqNumArray []*pb.MetaState
			for replicaHash, hChkpt := range rbft.storeMgr.hChkpts {
				if hChkpt.Applied <= H {
					delete(rbft.storeMgr.hChkpts, replicaHash)
					continue
				}
				chkptSeqNumArray = append(chkptSeqNumArray, hChkpt)
			}
			sort.Sort(sortableUint64Slice(chkptSeqNumArray))

			if chkptSeqNumArray == nil {
				return false
			}

			if len(chkptSeqNumArray) < rbft.oneCorrectQuorum() {
				return false
			}

			// If there are f+1 nodes have issued checkpoints above our high water mark, then current
			// node probably cannot record 2f+1 checkpoints for that sequence number, it is perhaps that
			// current node has been out of date
			if m := chkptSeqNumArray[len(chkptSeqNumArray)-rbft.oneCorrectQuorum()]; m.Applied > H {
				rbft.logger.Warningf("Replica %d is out of date, f+1 nodes agree checkpoints out of our high water mark %d", rbft.peerPool.ID, H)

				if rbft.exec.lastExec >= chkpt.SequenceNumber {
					rbft.logger.Infof("Replica %d has already executed block %d larger than checkpoint's seqNo %d", rbft.peerPool.ID, rbft.exec.lastExec, chkpt.SequenceNumber)
					rbft.softStartHighWatermarkTimer("replica received f+1 checkpoints out of range but we have already executed")
					return false
				}

				// update state update target here for an efficient initiation for a new state-update instance.
				target := m
				rbft.updateHighStateTarget(target)

				return true
			}
		}
	}

	return false
}

// moveWatermarks move low watermark h to n, and clear all message whose seqNo is smaller than h.
func (rbft *rbftImpl) moveWatermarks(n uint64) {

	h := n

	if rbft.h > n {
		rbft.logger.Criticalf("Replica %d moveWaterMarks but rbft.h(h=%d)>n(n=%d)", rbft.peerPool.ID, rbft.h, n)
		return
	}

	for idx := range rbft.storeMgr.certStore {
		if idx.n <= h {
			rbft.logger.Debugf("Replica %d cleaning quorum certificate for view=%d/seqNo=%d",
				rbft.peerPool.ID, idx.v, idx.n)
			delete(rbft.storeMgr.certStore, idx)
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			delete(rbft.storeMgr.seqMap, idx.n)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
		}
	}
	rbft.metrics.outstandingBatchesGauge.Set(float64(len(rbft.storeMgr.outstandingReqBatches)))

	// retain most recent 10 block info in txBatchStore cache as non-primary
	// replicas may need to fetch those batches if they are lack of some txs
	// in those batches.
	var target uint64
	pos := n / rbft.K * rbft.K
	if pos <= rbft.K {
		target = 0
	} else {
		target = pos - rbft.K
	}

	// clean batches every K interval
	var digestList []string
	for digest, batch := range rbft.storeMgr.batchStore {
		if batch.SeqNo <= target {
			delete(rbft.storeMgr.batchStore, digest)
			rbft.persistDelBatch(digest)
			digestList = append(digestList, digest)
		}
	}
	rbft.metrics.batchesGauge.Set(float64(len(rbft.storeMgr.batchStore)))
	rbft.batchMgr.requestPool.RemoveBatches(digestList)

	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	for idx := range rbft.storeMgr.committedCert {
		if idx.n <= h {
			delete(rbft.storeMgr.committedCert, idx)
		}
	}

	for cID, digest := range rbft.storeMgr.checkpointStore {
		if cID.sequence <= h {
			rbft.logger.Debugf("Replica %d cleaning checkpoint message from replica %s, seqNo %d, digest %s",
				rbft.peerPool.ID, cID.nodeHash, cID.sequence, digest)
			delete(rbft.storeMgr.checkpointStore, cID)
		}
	}

	for digest, idx := range rbft.storeMgr.missingBatchesInFetching {
		if idx.n <= h {
			delete(rbft.storeMgr.missingBatchesInFetching, digest)
		}
	}

	rbft.storeMgr.moveWatermarks(rbft, h)

	rbft.hLock.RLock()
	rbft.h = h
	rbft.persistH(h)
	rbft.hLock.RUnlock()

	rbft.logger.Infof("Replica %d updated low water mark to %d", rbft.peerPool.ID, rbft.h)
}

// updateHighStateTarget updates high state target
func (rbft *rbftImpl) updateHighStateTarget(target *pb.MetaState) {
	if target == nil {
		rbft.logger.Warningf("Replica %d received a nil target", rbft.peerPool.ID)
		return
	}

	if rbft.storeMgr.highStateTarget != nil && rbft.storeMgr.highStateTarget.Applied >= target.Applied {
		rbft.logger.Infof("Replica %d not updating state target to seqNo %d, has target for seqNo %d",
			rbft.peerPool.ID, target.Applied, rbft.storeMgr.highStateTarget.Applied)
		return
	}

	if rbft.atomicIn(StateTransferring) {
		rbft.logger.Infof("Replica %d has found high-target expired which transferring, update target to %d", rbft.peerPool.ID, target.Applied)
	}

	rbft.logger.Debugf("Replica %d updating state target to seqNo %d digest %s", rbft.peerPool.ID, target.Applied, target.Digest)
	rbft.storeMgr.highStateTarget = target
}

// tryStateTransfer sets system abnormal and stateTransferring, then skips to target
func (rbft *rbftImpl) tryStateTransfer() {
	if !rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d is out of sync, pending tryStateTransfer", rbft.peerPool.ID)
		rbft.on(SkipInProgress)
	}

	rbft.setAbNormal()

	if rbft.atomicIn(StateTransferring) {
		rbft.logger.Debugf("Replica %d is currently mid tryStateTransfer, it must wait for this tryStateTransfer to complete before initiating a new one", rbft.peerPool.ID)
		return
	}

	// if high state targe is nil, we could not state update
	if rbft.storeMgr.highStateTarget == nil {
		rbft.logger.Debugf("Replica %d has no targets to attempt tryStateTransfer to, delaying", rbft.peerPool.ID)
		return
	}
	target := rbft.storeMgr.highStateTarget

	// when we start to state update, it means we will find a correct checkpoint eventually,
	// so that we need to stop fetchCheckpointTimer here
	rbft.stopFetchCheckpointTimer()

	// besides, a node trying to state update will find a correct epoch at last,
	// so that, we need to reset the storage for config change and close config-change state here
	rbft.epochMgr.configBatchToCheck = nil
	rbft.atomicOff(InConfChange)
	rbft.metrics.statusGaugeInConfChange.Set(0)

	// just stop high-watermark timer:
	// a primary who has started a high-watermark timer because of missing of checkpoint may find
	// quorum checkpoint with different digest and trigger state-update
	rbft.stopHighWatermarkTimer()

	rbft.atomicOn(StateTransferring)
	rbft.metrics.statusGaugeStateTransferring.Set(StateTransferring)

	// clean cert with seqNo <= target before stateUpdate to avoid influencing the
	// following progress
	for idx := range rbft.storeMgr.certStore {
		if idx.n <= target.Applied {
			rbft.logger.Debugf("Replica %d clean cert with seqNo %d <= target %d, "+
				"digest=%s, before state update", rbft.peerPool.ID, idx.n, target.Applied, idx.d)
			delete(rbft.storeMgr.certStore, idx)
			delete(rbft.storeMgr.committedCert, idx)
			delete(rbft.storeMgr.seqMap, idx.n)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
		}
	}
	for d, batch := range rbft.storeMgr.outstandingReqBatches {
		if batch.SeqNo <= target.Applied {
			rbft.logger.Debugf("Replica %d clean outstanding batch with seqNo %d <= target %d, "+
				"digest=%s, before state update", rbft.peerPool.ID, batch.SeqNo, target.Applied, d)
			delete(rbft.storeMgr.outstandingReqBatches, d)
		}
	}
	rbft.metrics.outstandingBatchesGauge.Set(float64(len(rbft.storeMgr.outstandingReqBatches)))
	for d, batch := range rbft.storeMgr.batchStore {
		if batch.SeqNo <= target.Applied {
			rbft.logger.Debugf("Replica %d clean batch with seqNo %d <= target %d, "+
				"digest=%s, before state update", rbft.peerPool.ID, batch.SeqNo, target.Applied, d)
			delete(rbft.storeMgr.batchStore, d)
			rbft.persistDelBatch(d)
		}
	}
	rbft.metrics.batchesGauge.Set(float64(len(rbft.storeMgr.batchStore)))
	// NOTE!!! save batches with seqNo larger than target height because those
	// batches may be useful in PQList.
	var saveBatches []string
	for digest := range rbft.storeMgr.batchStore {
		saveBatches = append(saveBatches, digest)
	}

	rbft.batchMgr.requestPool.Reset(saveBatches)
	// reset the status of PoolFull
	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	// clear cacheBatch as they are useless and all related batches have been reset in requestPool.
	rbft.batchMgr.cacheBatch = nil
	rbft.metrics.cacheBatchNumber.Set(float64(0))

	rbft.logger.Noticef("Replica %d try state update to %d", rbft.peerPool.ID, target.Applied)

	// attempts to synchronize state to a particular target, implicitly calls rollback if needed
	rbft.metrics.stateUpdateCounter.Add(float64(1))
	rbft.external.StateUpdate(target.Applied, target.Digest, nil)
}

// recvStateUpdatedEvent processes StateUpdatedMessage.
// functions:
// 1) succeed or not
// 2) update information about latest stable checkpoint
// 3) update epoch info if it has been changed
//
// we need to check if the state update process is successful at first
// as for that state update target is the latest stable checkpoint,
// we need to move watermark and update our checkpoint storage for it
// at last if the epoch info has been changed, we also need to update
// self epoch-info and trigger another recovery process
func (rbft *rbftImpl) recvStateUpdatedEvent(ss *pb.ServiceState) consensusEvent {
	seqNo := ss.MetaState.Applied
	digest := ss.MetaState.Digest

	// high state target nil warning
	if rbft.storeMgr.highStateTarget == nil {
		rbft.logger.Warningf("Replica %d has no state targets, cannot resume tryStateTransfer yet", rbft.peerPool.ID)
	} else if seqNo < rbft.storeMgr.highStateTarget.Applied {
		// If state transfer did not complete successfully, or if it did not reach our low watermark, do it again
		// When this node moves watermark before this node receives StateUpdatedMessage, this would happen.
		rbft.logger.Warningf("Replica %d recovered to seqNo %d but our high-target has moved to %d, transferring", rbft.peerPool.ID, seqNo, rbft.storeMgr.highStateTarget.Applied)
		rbft.atomicOff(StateTransferring)
		rbft.metrics.statusGaugeStateTransferring.Set(0)
		rbft.exec.setLastExec(seqNo)
		rbft.tryStateTransfer()
	} else if seqNo == rbft.storeMgr.highStateTarget.Applied {
		rbft.logger.Debugf("Replica %d recovered to seqNo %d and highest is %d, turn off stateTransferring", rbft.peerPool.ID, seqNo, rbft.storeMgr.highStateTarget.Applied)
		rbft.exec.setLastExec(seqNo)
		rbft.atomicOff(StateTransferring)
		rbft.metrics.statusGaugeStateTransferring.Set(0)
	} else {
		rbft.logger.Warningf("Replica %d recovered to seqNo %d but highest is %d, too high for seq", rbft.peerPool.ID, seqNo, rbft.storeMgr.highStateTarget.Applied)
	}

	// 1. finished state update
	rbft.logger.Noticef("======== Replica %d finished stateUpdate, height: %d", rbft.peerPool.ID, seqNo)
	finishMsg := fmt.Sprintf("======== Replica %d finished stateUpdate, height: %d", rbft.peerPool.ID, seqNo)
	rbft.external.SendFilterEvent(pb.InformType_FilterFinishStateUpdate, finishMsg)
	rbft.exec.setLastExec(seqNo)
	rbft.batchMgr.setSeqNo(seqNo)
	rbft.storeMgr.missingBatchesInFetching = make(map[string]msgID)
	rbft.off(SkipInProgress)
	rbft.atomicOff(StateTransferring)
	rbft.metrics.statusGaugeStateTransferring.Set(0)
	rbft.maybeSetNormal()

	// 2. process information about stable checkpoint
	rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d after state update to the height",
		rbft.peerPool.ID, seqNo)
	rbft.external.SendFilterEvent(pb.InformType_FilterStableCheckpoint, seqNo, digest)
	rbft.storeMgr.saveCheckpoint(seqNo, digest)
	rbft.persistCheckpoint(seqNo, []byte(digest))
	rbft.moveWatermarks(seqNo)

	// 3. process epoch-info
	changed := false
	if ss.EpochInfo != nil {
		changed = ss.EpochInfo.Epoch != rbft.epoch
	}
	if changed {
		var peers []*pb.Peer
		for index, hostname := range ss.EpochInfo.VSet {
			peer := &pb.Peer{
				Id:       uint64(index + 1),
				Hostname: hostname,
			}
			peers = append(peers, peer)
		}
		router := &pb.Router{
			Peers: peers,
		}
		rbft.turnIntoEpoch(router, ss.EpochInfo.Epoch)

		// trigger another round of recovery to find correct view-number
		rbft.logger.Noticef("======== Replica %d updated epoch, epoch=%d.", rbft.peerPool.ID, rbft.epoch)
		rbft.timerMgr.stopTimer(recoveryRestartTimer)
		return rbft.initRecovery()
	}

	if rbft.atomicIn(InRecovery) {
		if rbft.isPrimary(rbft.peerPool.ID) {
			// view may not be changed after state-update, current node mistakes itself for primary
			// so init recovery to recover the view and other states
			rbft.logger.Debugf("Primary %d init recovery after state update", rbft.peerPool.ID)
			return rbft.initRecovery()
		}

		return &LocalEvent{
			Service:   RecoveryService,
			EventType: RecoveryDoneEvent,
		}
	}

	if rbft.atomicIn(InViewChange) {
		if rbft.isPrimary(rbft.peerPool.ID) {
			// view may not be changed after state-update, current node mistakes itself for primary
			// so send view change to step into the new view
			rbft.logger.Debugf("Primary %d send view-change after state update", rbft.peerPool.ID)
			return rbft.sendViewChange()
		}

		return &LocalEvent{
			Service:   ViewChangeService,
			EventType: ViewChangedEvent,
		}
	}

	rbft.executeAfterStateUpdate()
	return nil
}

// executeAfterStateUpdate processes logic after state update
func (rbft *rbftImpl) executeAfterStateUpdate() {

	if rbft.isPrimary(rbft.peerPool.ID) {
		rbft.logger.Debugf("Replica %d is primary, not execute after stateUpdate", rbft.peerPool.ID)
		return
	}

	rbft.logger.Debugf("Replica %d try to execute after stateUpdate", rbft.peerPool.ID)

	for idx, cert := range rbft.storeMgr.certStore {
		// If this node is not primary, it would commit pending transactions.
		if rbft.prepared(idx.d, idx.v, idx.n) && !cert.sentCommit {
			rbft.logger.Debugf("Replica %d try to commit batch %s", rbft.peerPool.ID, idx.d)
			_ = rbft.findNextCommitBatch(idx.d, idx.v, idx.n)
		}
	}
}
