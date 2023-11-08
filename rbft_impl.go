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
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/samber/lo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics"
	"github.com/axiomesh/axiom-bft/txpool"
	"github.com/axiomesh/axiom-bft/types"
)

// Config contains the parameters to start a RAFT instance.
type Config struct {
	// Genesis for test
	GenesisEpochInfo *EpochInfo

	GenesisBlockDigest string

	LastCheckpointBlockDigest string

	// self staking account address
	SelfAccountAddress string

	// Applied is the latest height index of application service which should be assigned when node restart.
	Applied uint64

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

	// VcResendTimeout is the time duration one wait for a viewChange quorum before resending
	// the same viewChange.
	VcResendTimeout time.Duration

	// CleanVCTimeout is the time duration ro clear out-of-date viewChange messages.
	CleanVCTimeout time.Duration

	// NewViewTimeout is the time duration one waits for newView messages in viewChange.
	NewViewTimeout time.Duration

	// SyncStateTimeout is the time duration one wait for same syncStateResponse.
	SyncStateTimeout time.Duration

	// SyncStateRestartTimeout is the time duration one resend syncState request after last
	// successful sync.
	SyncStateRestartTimeout time.Duration

	// FetchCheckpointTimeout
	FetchCheckpointTimeout time.Duration

	// FetchViewTimeout
	FetchViewTimeout time.Duration

	// CheckPoolTimeout is the time duration one check for out-of-date requests in request pool cyclically.
	CheckPoolTimeout time.Duration

	// FlowControl indicates whether flow control has been opened.
	FlowControl bool

	// FlowControlMaxMem indicates the max memory size of txs in request set
	FlowControlMaxMem int

	// MetricsProv is the metrics Provider used to generate metrics instance.
	MetricsProv metrics.Provider

	// Tracer is the tracing Provider used to record tracing info
	Tracer trace.Tracer

	// DelFlag is a channel to stop namespace when there is a non-recoverable error
	DelFlag chan bool

	// Logger is the logger used to record logger in RBFT.
	Logger common.Logger

	// NoTxBatchTimeout is the max time duration before one generating block which packing no tx.
	NoTxBatchTimeout time.Duration

	// CheckPoolRemoveTimeout is the max time duration before one removing tx from pool.
	CheckPoolRemoveTimeout time.Duration

	// CommittedBlockCacheNumber is committed block cache number after checkpoint
	CommittedBlockCacheNumber uint64
}

// rbftImpl is the core struct of RBFT service, which handles all functions about consensus.
type rbftImpl[T any, Constraint consensus.TXConstraint[T]] struct {
	node        *node[T, Constraint]
	chainConfig *ChainConfig
	external    ExternalStack[T, Constraint] // manage interaction with application layer

	status      *statusManager               // keep all basic status of rbft in this object
	timerMgr    *timerManager                // manage rbft event timers
	exec        *executor                    // manage transaction execution
	storeMgr    *storeManager[T, Constraint] // manage memory log storage
	batchMgr    *batchManager[T, Constraint] // manage request batch related issues
	recoveryMgr *recoveryManager             // manage recovery issues
	vcMgr       *vcManager                   // manage viewchange issues
	peerMgr     *peerManager                 // manage node status including route table, the connected peers and so on
	epochMgr    *epochManager                // manage epoch issues
	storage     Storage                      // manage non-volatile storage of consensus log

	recvChan chan consensusEvent // channel to receive ordered consensus messages and local events
	delFlag  chan bool           // channel to stop namespace when there is a non-recoverable error
	close    chan bool           // channel to close this event process

	flowControl       bool // whether limit flow or not
	flowControlMaxMem int  // the max memory size of txs in request set

	reusableRequestBatch     *consensus.FetchBatchResponse // special struct to reuse the biggest message in rbft.
	highWatermarkTimerReason string                        // reason to trigger high watermark timer

	viewLock  sync.RWMutex // mutex to set value of view
	hLock     sync.RWMutex // mutex to set value of h
	epochLock sync.RWMutex // mutex to set value of view

	wg sync.WaitGroup // make sure the listener has been closed

	config  Config        // get configuration info
	metrics *rbftMetrics  // collect all metrics in rbft
	tracer  trace.Tracer  // record tracing info
	logger  common.Logger // write logger to record some info

	isInited bool
	isTest   bool
}

var once sync.Once

// newRBFT init the RBFT instance
func newRBFT[T any, Constraint consensus.TXConstraint[T]](c Config, external ExternalStack[T, Constraint], requestPool txpool.TxPool[T, Constraint], isTest bool) (*rbftImpl[T, Constraint], error) {
	err := c.GenesisEpochInfo.Check()
	if err != nil {
		return nil, err
	}
	if !isTest {
		if c.GenesisEpochInfo.Epoch != 1 || c.GenesisEpochInfo.StartBlock != 1 {
			return nil, fmt.Errorf("epoch info error: genesis epoch and start_block must be 1, but get epoch: %d, start_block: %d", c.GenesisEpochInfo.Epoch, c.GenesisEpochInfo.StartBlock)
		}
	}
	if c.CommittedBlockCacheNumber == 0 {
		if !isTest {
			c.CommittedBlockCacheNumber = 10
		} else {
			c.CommittedBlockCacheNumber = c.GenesisEpochInfo.ConsensusParams.CheckpointPeriod
		}
	}

	// init message event converter
	once.Do(initMsgEventMap)

	recvC := make(chan consensusEvent)
	if isTest {
		recvC = make(chan consensusEvent, 1024)
	}
	chainConfig := &ChainConfig{
		EpochInfo:        nil,
		EpochDerivedData: EpochDerivedData{},
		DynamicChainConfig: DynamicChainConfig{
			H: c.GenesisEpochInfo.StartBlock,
		},
		SelfAccountAddress: c.SelfAccountAddress,
	}
	rbft := &rbftImpl[T, Constraint]{
		chainConfig:          chainConfig,
		config:               c,
		logger:               c.Logger,
		external:             external,
		storage:              external,
		recvChan:             recvC,
		close:                make(chan bool),
		delFlag:              c.DelFlag,
		flowControl:          c.FlowControl,
		flowControlMaxMem:    c.FlowControlMaxMem,
		reusableRequestBatch: &consensus.FetchBatchResponse{},
		tracer:               c.Tracer,
		isTest:               isTest,
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

	// new status manager
	rbft.status = newStatusMgr()

	// new peer pool
	rbft.peerMgr = newPeerManager(chainConfig, rbft.metrics, external, c, isTest)

	// new executor
	rbft.exec = newExecutor()

	// new store manager
	rbft.storeMgr = newStoreMgr[T, Constraint](c)

	// new batch manager
	rbft.batchMgr = newBatchManager(requestPool, c)

	// new recovery manager
	rbft.recoveryMgr = newRecoveryMgr(c)

	// new viewChange manager
	rbft.vcMgr = newVcManager(c)

	// new epoch manager
	rbft.epochMgr = newEpochManager(chainConfig, c, rbft.peerMgr, external, external)

	// use GenesisEpochInfo as default
	rbft.chainConfig.EpochInfo = c.GenesisEpochInfo
	if err := rbft.chainConfig.updateDerivedData(); err != nil {
		return nil, err
	}
	return rbft, nil
}

func (rbft *rbftImpl[T, Constraint]) init() error {
	if rbft.isInited {
		return nil
	}
	// restore state from consensus database
	rbft.exec.setLastExec(rbft.config.Applied)

	// load state from storage
	if rErr := rbft.restoreState(); rErr != nil {
		rbft.logger.Errorf("Replica restore state failed: %s", rErr)
		return rErr
	}

	rbft.initTimers()
	rbft.initStatus()

	// update viewChange seqNo after restore state which may update seqNo
	rbft.updateViewChangeSeqNo(rbft.exec.lastExec, rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod)
	rbft.metrics.idGauge.Set(float64(rbft.chainConfig.SelfID))
	rbft.metrics.epochGauge.Set(float64(rbft.chainConfig.EpochInfo.Epoch))
	rbft.metrics.clusterSizeGauge.Set(float64(rbft.chainConfig.N))
	rbft.metrics.quorumSizeGauge.Set(float64(rbft.commonCaseQuorum()))

	rbft.logger.Infof("RBFT enable wrf = %v", rbft.chainConfig.isProposerElectionTypeWRF())
	rbft.logger.Infof("RBFT epoch period = %v", rbft.chainConfig.EpochInfo.EpochPeriod)
	rbft.logger.Infof("RBFT current epoch = %v", rbft.chainConfig.EpochInfo.Epoch)
	rbft.logger.Infof("RBFT current view = %v", rbft.chainConfig.View)
	rbft.logger.Infof("RBFT last exec block = %v", rbft.exec.lastExec)
	rbft.logger.Infof("RBFT Max number of validating peers (N) = %v", rbft.chainConfig.N)
	rbft.logger.Infof("RBFT Max number of failing peers (f) = %v", rbft.chainConfig.F)
	rbft.logger.Infof("RBFT byzantine flag = %v", rbft.in(byzantine))
	rbft.logger.Infof("RBFT SignedCheckpoint period (K) = %v", rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod)
	rbft.logger.Infof("RBFT Log multiplier = %v", rbft.chainConfig.EpochInfo.ConsensusParams.HighWatermarkCheckpointPeriod)
	rbft.logger.Infof("RBFT log size (L) = %v", rbft.chainConfig.L)
	rbft.logger.Infof("RBFT ID: %d", rbft.chainConfig.SelfID)
	rbft.logger.Infof("RBFT isTimed: %v", rbft.chainConfig.EpochInfo.ConsensusParams.EnableTimedGenEmptyBlock)
	rbft.logger.Infof("RBFT minimum number of batches to retain after checkpoint = %v", rbft.config.CommittedBlockCacheNumber)

	if err := rbft.batchMgr.requestPool.Init(rbft.chainConfig.SelfID); err != nil {
		return err
	}
	rbft.isInited = true
	return nil
}

// start initializes and starts the consensus service
func (rbft *rbftImpl[T, Constraint]) start() error {
	if err := rbft.init(); err != nil {
		return err
	}

	if err := rbft.batchMgr.requestPool.Start(); err != nil {
		return err
	}
	// exit pending status after start rbft to avoid missing consensus messages from other nodes.
	rbft.atomicOff(Pending)
	rbft.metrics.statusGaugePending.Set(0)

	rbft.logger.Noticef("--------RBFT starting, nodeID: %d--------", rbft.chainConfig.SelfID)

	// if the stable-checkpoint recovered from consensus-database is equal to the config-batch-to-check,
	// current state has already been checked to be stable, and we need not check it again.

	// The checkpoint is nil if the block cannot be recovered from another node for reasons such as archiving.

	localCheckpoint := rbft.storeMgr.localCheckpoints[rbft.chainConfig.H]
	if localCheckpoint != nil {
		metaS := &types.MetaState{
			Height: rbft.chainConfig.H,
			Digest: localCheckpoint.Checkpoint.Digest(),
		}
		if rbft.equalMetaState(rbft.epochMgr.configBatchToCheck, metaS) {
			rbft.logger.Info("Config batch to check has already been stable, reset it")
			rbft.epochMgr.configBatchToCheck = nil
		}
	}

	// start listen consensus event
	go rbft.listenEvent()

	// NOTE!!! must use goroutine to post the event to avoid blocking the rbft service.
	// trigger recovery
	initRecoveryEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryInitEvent,
		Event:     rbft.chainConfig.View,
	}
	rbft.postMsg(initRecoveryEvent)

	return nil
}

// stop the consensus service
func (rbft *rbftImpl[T, Constraint]) stop() []*T {
	rbft.logger.Notice("RBFT stopping...")

	// reset status to pending.
	rbft.initStatus()
	rbft.atomicOn(Stopped)

	remainTxs, err := rbft.drainChannel(rbft.recvChan)
	if err != nil {
		rbft.logger.Errorf("drain channel error: %s", err)
	}

	rbft.logger.Debugf("get %d remaining txs from recvChan", len(remainTxs))

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
	rbft.logger.Notice("RBFT stopped!")

	return remainTxs
}

// step receives and processes messages from other peers
func (rbft *rbftImpl[T, Constraint]) step(ctx context.Context, msg *consensus.ConsensusMessage) {
	if rbft.atomicIn(Pending) {
		if rbft.atomicIn(Stopped) {
			rbft.logger.Debugf("Replica %d is stopped, reject every consensus messages", rbft.chainConfig.SelfID)
			return
		}

		// cache view changes in pending status and re-process these requests after start RBFT core to help
		// recovery quickly.
		switch msg.Type {
		case consensus.Type_VIEW_CHANGE:
			// don't cache vc with different epoch
			if msg.Epoch != rbft.chainConfig.EpochInfo.Epoch {
				rbft.logger.Errorf("Replica %d received message from invalid epoch %d, ignore it in "+
					"pending status", rbft.chainConfig.SelfID, msg.Epoch)
				return
			}
			// don't cache vc from unknown author
			if _, ok := rbft.chainConfig.NodeInfoMap[msg.From]; !ok {
				rbft.logger.Errorf("Replica %d received message from unknown node %d, ignore it in "+
					"pending status", rbft.chainConfig.SelfID, msg.From)
				return
			}
			vc := &consensus.ViewChange{}
			err := vc.UnmarshalVT(msg.Payload)
			if err != nil {
				rbft.logger.Errorf("Consensus Message ViewChange Unmarshal error: %v", err)
				return
			}
			vcBasis := vc.Basis
			idx := vcIdx{v: vcBasis.GetView(), id: vcBasis.GetReplicaId()}
			rbft.logger.Debugf("Replica %d is in pending status, pre-store the view change message: "+
				"from replica %d, e:%d, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
				rbft.chainConfig.SelfID, vcBasis.GetReplicaId(), msg.Epoch, vcBasis.GetView(), vcBasis.GetH(),
				len(vcBasis.GetCset()), len(vcBasis.GetCset()), len(vcBasis.GetQset()))
			vc.Timestamp = time.Now().UnixNano()
			rbft.vcMgr.viewChangeStore[idx] = vc

		default:
			rbft.logger.Debugf("Replica %d is in pending status, reject consensus messages", rbft.chainConfig.SelfID)
		}

		return
	}

	// block consensus progress until sync to epoch change height.
	if rbft.atomicIn(inEpochSyncing) {
		rbft.logger.Debugf("Replica %d is in epoch syncing status, reject consensus messages", rbft.chainConfig.SelfID)
		return
	}

	// nolint: errcheck
	rbft.postMsg(&consensusMessageWrapper{
		ctx:              ctx,
		ConsensusMessage: msg,
	})
}

// postRequests informs RBFT tx set event which is posted from application layer.
func (rbft *rbftImpl[T, Constraint]) postRequests(requests *RequestSet[T, Constraint]) {
	if rbft.atomicIn(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, reject propose request", rbft.chainConfig.SelfID)
		return
	}

	rbft.postMsg(requests)
}

// postBatches informs RBFT batch event which is usually generated by request pool.
func (rbft *rbftImpl[T, Constraint]) postBatches(batches []*txpool.RequestHashBatch[T, Constraint]) {
	for _, batch := range batches {
		_ = rbft.recvRequestBatch(batch)
	}
}

// postMsg posts messages to main loop.
func (rbft *rbftImpl[T, Constraint]) postMsg(msg any) {
	rbft.recvChan <- msg
}

// reportStateUpdated informs RBFT stateUpdated event.
func (rbft *rbftImpl[T, Constraint]) reportStateUpdated(state *types.ServiceSyncState) {
	if rbft.atomicIn(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, reject report state updated", rbft.chainConfig.SelfID)
		return
	}
	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event:     state,
	}

	go rbft.postMsg(event)
}

// reportCheckpoint informs RBFT checkpoint event.
func (rbft *rbftImpl[T, Constraint]) reportCheckpoint(state *types.ServiceState) {
	if rbft.atomicIn(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, reject report checkpoint", rbft.chainConfig.SelfID)
		return
	}

	height := state.MetaState.Height
	// report checkpoint of config block height or checkpoint block height.
	if rbft.readConfigBatchToExecute() == height || height%rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod == 0 {
		rbft.logger.Debugf("Report checkpoint: {%d, %s} to core", state.MetaState.Height, state.MetaState.Digest)
		event := &LocalEvent{
			Service:   CoreRbftService,
			EventType: CoreCheckpointBlockExecutedEvent,
			Event:     state,
		}

		// for test, will auto report checkpoint, no need to report
		if !rbft.isTest {
			go rbft.postMsg(event)
		}
	}
}

// getStatus returns the current consensus status.
// NOTE. This function may be invoked by application in another go-routine
// rather than the main event loop go-routine, so we need to protect
// safety of `rbft.peerMgr.noMap`, `rbft.chainConfig.View` and `rbft.Status`
func (rbft *rbftImpl[T, Constraint]) getStatus() (status NodeStatus) {
	rbft.viewLock.RLock()
	status.View = rbft.chainConfig.View
	rbft.viewLock.RUnlock()

	rbft.hLock.RLock()
	status.H = rbft.chainConfig.H
	rbft.hLock.RUnlock()

	rbft.epochLock.RLock()
	status.EpochInfo = rbft.chainConfig.EpochInfo.Clone()
	rbft.epochLock.RUnlock()

	status.ID = rbft.chainConfig.SelfID
	switch {
	case rbft.atomicIn(InConfChange):
		status.Status = InConfChange
	case rbft.atomicIn(inEpochSyncing):
		status.Status = InConfChange
	case rbft.atomicIn(InRecovery):
		status.Status = InRecovery
	case rbft.atomicIn(InViewChange):
		status.Status = InViewChange
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
func (rbft *rbftImpl[T, Constraint]) listenEvent() {
	rbft.wg.Add(1)
	defer rbft.wg.Done()
	for {
		select {
		case <-rbft.close:
			rbft.logger.Notice("exit RBFT event listener")
			return
		case next := <-rbft.recvChan:
			cm, isConsensusMessage := next.(*consensusMessageWrapper)
			for {
				select {
				case <-rbft.close:
					rbft.logger.Notice("exit RBFT event listener")
					return
				default:
				}
				next = rbft.processEvent(next)
				if next == nil {
					break
				}
			}
			// check view after finished process consensus messages from remote node as current view may
			// be changed because of above consensus messages.
			if isConsensusMessage {
				rbft.checkView(cm.ConsensusMessage)
			}
		}
	}
}

func (rbft *rbftImpl[T, Constraint]) checkEventCanProcessWhenWaitCheckpoint(eventType int, event any) bool {
	if !rbft.in(waitCheckpointBatchExecute) {
		return true
	}
	if _, ok := canProcessEventsWhenWaitCheckpoint[eventType]; ok {
		return true
	}
	rbft.storeMgr.beforeCheckpointEventCache = append(rbft.storeMgr.beforeCheckpointEventCache, event)
	return false
}

func (rbft *rbftImpl[T, Constraint]) checkMsgCanAccept(msgType consensus.Type, msgFrom uint64) bool {
	if rbft.chainConfig.isValidator() {
		if rbft.chainConfig.NodeRoleMap[msgFrom] != NodeRoleValidator {
			if _, ok := validatorAcceptMsgsFromNonValidator[msgType]; !ok {
				rbft.logger.Debugf("Replica %d not accept msg[%s] from non-validator %d",
					rbft.chainConfig.SelfID, msgType.String(), msgFrom)
				return false
			}
		}
	}
	return true
}

func (rbft *rbftImpl[T, Constraint]) checkMsgCanProcessWhenWaitCheckpoint(msgType consensus.Type, msg any) bool {
	if !rbft.in(waitCheckpointBatchExecute) {
		return true
	}
	if _, ok := canProcessMsgsWhenWaitCheckpoint[msgType]; ok {
		return true
	}
	rbft.storeMgr.beforeCheckpointEventCache = append(rbft.storeMgr.beforeCheckpointEventCache, msg)
	return false
}

// processEvent process consensus messages and local events cyclically.
func (rbft *rbftImpl[T, Constraint]) processEvent(ee consensusEvent) consensusEvent {
	start := time.Now()
	switch e := ee.(type) {
	case *RequestSet[T, Constraint]:
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
		rbft.metrics.processEventDuration.With("event", "request_set").Observe(time.Since(start).Seconds())
		return nil

	case *LocalEvent:
		if !rbft.checkEventCanProcessWhenWaitCheckpoint(e.EventType, e) {
			rbft.metrics.processEventDuration.With("event", "local_event").Observe(time.Since(start).Seconds())
			return nil
		}
		ev := rbft.dispatchLocalEvent(e)
		rbft.metrics.processEventDuration.With("event", "local_event").Observe(time.Since(start).Seconds())
		return ev

	case *MiscEvent:
		ev := rbft.dispatchMiscEvent(e)
		rbft.metrics.processEventDuration.With("event", "misc_event").Observe(time.Since(start).Seconds())
		return ev

	case *consensusMessageWrapper:
		if !rbft.checkMsgCanProcessWhenWaitCheckpoint(e.ConsensusMessage.Type, e) {
			rbft.metrics.processEventDuration.With("event", "consensus_message").Observe(time.Since(start).Seconds())
			return nil
		}
		ev := rbft.consensusMessageFilter(e.ctx, e, e.ConsensusMessage)
		rbft.metrics.processEventDuration.With("event", "consensus_message").Observe(time.Since(start).Seconds())
		return ev

	default:
		rbft.logger.Errorf("Can't recognize event type of %v.", e)
		return nil
	}
}

func (rbft *rbftImpl[T, Constraint]) consensusMessageFilter(ctx context.Context, originEvent consensusEvent, msg *consensus.ConsensusMessage) consensusEvent {
	// A node in different epoch or in epoch sync will reject normal consensus messages, except:
	// EpochChangeRequest and EpochChangeProof.
	if msg.Epoch != rbft.chainConfig.EpochInfo.Epoch {
		if msg.Epoch > rbft.epochMgr.epoch {
			if !rbft.checkEventCanProcessWhenWaitCheckpoint(cannotProcessEventWhenWaitCheckpoint, originEvent) {
				return nil
			}
		}

		return rbft.epochMgr.checkEpoch(msg)
	}

	if !rbft.checkMsgCanAccept(msg.Type, msg.From) {
		return nil
	}
	msgEvent, err := rbft.msgToEvent(msg)
	if err != nil {
		return nil
	}
	start := time.Now()
	next := rbft.dispatchConsensusMsg(ctx, originEvent, msgEvent)
	rbft.metrics.processEventDuration.With("event", "consensus_message_"+msg.Type.String()).Observe(time.Since(start).Seconds())
	return next
}

// dispatchCoreRbftMsg dispatch core RBFT consensus messages.
func (rbft *rbftImpl[T, Constraint]) dispatchCoreRbftMsg(ctx context.Context, e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *consensus.NullRequest:
		return rbft.recvNullRequest(et)
	case *consensus.PrePrepare:
		return rbft.recvPrePrepare(ctx, et)
	case *consensus.Prepare:
		return rbft.recvPrepare(ctx, et)
	case *consensus.Commit:
		return rbft.recvCommit(ctx, et)
	case *consensus.FetchMissingRequest:
		return rbft.recvFetchMissingRequest(ctx, et)
	case *consensus.FetchMissingResponse:
		return rbft.recvFetchMissingResponse(ctx, et)
	case *consensus.SignedCheckpoint:
		return rbft.recvCheckpoint(et, false)
	case *consensus.ReBroadcastRequestSet:
		return rbft.recvReBroadcastRequestSet(et)
	}
	return nil
}

// =============================================================================
// null request methods
// =============================================================================

// handleNullRequestEvent triggered by null request timer, primary needs to send a null request
// and replica needs to send view change
func (rbft *rbftImpl[T, Constraint]) handleNullRequestTimerEvent() {
	if rbft.atomicIn(InViewChange) {
		rbft.logger.Debugf("Replica %d try to handle null request timer, but it's in viewChange", rbft.chainConfig.SelfID)
		return
	}

	if !rbft.isPrimary(rbft.chainConfig.SelfID) {
		// replica expects a null request, but primary never sent one
		rbft.logger.Warningf("Replica %d null request timer expired, sending viewChange", rbft.chainConfig.SelfID)
		rbft.sendViewChange()
	} else {
		rbft.logger.Infof("Primary %d null request timer expired, sending null request", rbft.chainConfig.SelfID)
		rbft.sendNullRequest()

		rbft.trySyncState()
	}
}

// sendNullRequest is for primary peer to send null when nullRequestTimer booms
func (rbft *rbftImpl[T, Constraint]) sendNullRequest() {
	// primary node reject send null request in conf change status.
	if rbft.atomicIn(InConfChange) {
		rbft.logger.Infof("Replica %d not send null request in conf change status", rbft.chainConfig.SelfID)
		return
	}

	nullRequest := &consensus.NullRequest{
		ReplicaId: rbft.chainConfig.SelfID,
	}
	payload, err := nullRequest.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_NULL_REQUEST Marshal Error: %s", err)
		return
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_NULL_REQUEST,
		Payload: payload,
	}
	rbft.peerMgr.broadcast(context.TODO(), consensusMsg)
	rbft.nullReqTimerReset()
}

// recvNullRequest process null request when it come
func (rbft *rbftImpl[T, Constraint]) recvNullRequest(msg *consensus.NullRequest) consensusEvent {
	if rbft.atomicIn(InViewChange) {
		rbft.logger.Infof("Replica %d is in viewChange, reject null request from replica %d", rbft.chainConfig.SelfID, msg.ReplicaId)
		return nil
	}

	// backup node reject process null request in conf change status.
	if rbft.atomicIn(InConfChange) {
		rbft.logger.Infof("Replica %d is in conf change, reject null request from replica %d", rbft.chainConfig.SelfID, msg.ReplicaId)
		return nil
	}

	// only primary could send a null request
	if !rbft.isPrimary(msg.ReplicaId) {
		rbft.logger.Warningf("Replica %d received null request from replica %d who is not primary", rbft.chainConfig.SelfID, msg.ReplicaId)
		return nil
	}

	rbft.logger.Infof("Replica %d received null request from primary %d", rbft.chainConfig.SelfID, msg.ReplicaId)

	rbft.trySyncState()
	rbft.nullReqTimerReset()

	return nil
}

// =============================================================================
// process request set and batch methods
// =============================================================================

// processReqSetEvent process received requestSet event, reject txs in following situations:
// 1. pool is full, reject txs relayed from other nodes
// 2. node is in config change, add another config tx into txpool
// 3. node is in skipInProgress, rejects any txs from other nodes
func (rbft *rbftImpl[T, Constraint]) processReqSetEvent(req *RequestSet[T, Constraint]) consensusEvent {
	// if pool already full, rejects the tx, unless it's from RPC because of time difference or we have opened flow control
	if rbft.isPoolFull() && !req.Local && !rbft.flowControl {
		rbft.rejectRequestSet(req)
		return nil
	}

	// if current node is in skipInProgress, it should reject the transactions coming from other nodes, but has the responsibility to keep its own transactions
	// if rbft.in(SkipInProgress) && !req.Local {
	//	rbft.rejectRequestSet(req)
	//	return nil
	// }

	// if current node is in abnormal, add normal txs into txPool without generate batches.
	if !rbft.isNormal() || rbft.in(SkipInProgress) || rbft.in(InRecovery) || rbft.in(inEpochSyncing) || rbft.in(waitCheckpointBatchExecute) {
		_, completionMissingBatchHashes := rbft.batchMgr.requestPool.AddNewRequests(req.Requests, false, req.Local, false, false)
		var completionMissingBatchIdxs []msgID
		for _, batchHash := range completionMissingBatchHashes {
			idx, ok := rbft.storeMgr.missingBatchesInFetching[batchHash]
			if !ok {
				rbft.logger.Warningf("Replica %d completion batch with hash %s but not found missing record",
					rbft.chainConfig.SelfID, batchHash)
			} else {
				rbft.logger.Infof("Replica %d completion batch with hash %s",
					rbft.chainConfig.SelfID, batchHash)
				completionMissingBatchIdxs = append(completionMissingBatchIdxs, idx)
			}
			delete(rbft.storeMgr.missingBatchesInFetching, batchHash)
		}
		if rbft.in(waitCheckpointBatchExecute) {
			// findNextPrepareBatch after call checkpoint
			rbft.storeMgr.beforeCheckpointEventCache = append(rbft.storeMgr.beforeCheckpointEventCache, &LocalEvent{
				Service:   CoreRbftService,
				EventType: CoreFindNextPrepareBatchsEvent,
				Event:     completionMissingBatchIdxs,
			})
		}
	} else {
		// primary nodes would check if this transaction triggered generating a batch or not
		if rbft.isPrimary(rbft.chainConfig.SelfID) {
			// start batch timer and stop no tx batch timer when this node receives the first transaction of a batch
			if !rbft.batchMgr.isBatchTimerActive() {
				rbft.startBatchTimer()
			}

			batches, _ := rbft.batchMgr.requestPool.AddNewRequests(req.Requests, true, req.Local, false, rbft.inPrimaryTerm())
			// If these transactions trigger generating a batch, stop batch timer
			if len(batches) != 0 {
				rbft.stopBatchTimer()
				now := time.Now().UnixNano()
				if rbft.batchMgr.lastBatchTime != 0 {
					interval := time.Duration(now - rbft.batchMgr.lastBatchTime).Seconds()
					rbft.metrics.batchInterval.With("type", "maxSize").Observe(interval)
				}
				rbft.batchMgr.lastBatchTime = now
				rbft.postBatches(batches)
			}
			// if received txs but it is not match generate batch, stop no tx batch timer
			if rbft.config.GenesisEpochInfo.ConsensusParams.EnableTimedGenEmptyBlock &&
				rbft.batchMgr.requestPool.HasPendingRequestInPool() && rbft.batchMgr.noTxBatchTimerActive {
				rbft.stopNoTxBatchTimer()
			}
		} else {
			_, completionMissingBatchHashes := rbft.batchMgr.requestPool.AddNewRequests(req.Requests, false, req.Local, false, false)
			for _, batchHash := range completionMissingBatchHashes {
				idx, ok := rbft.storeMgr.missingBatchesInFetching[batchHash]
				if !ok {
					rbft.logger.Warningf("Replica %d completion batch with hash %s but not found missing record",
						rbft.chainConfig.SelfID, batchHash)
				} else {
					rbft.logger.Infof("Replica %d completion batch with hash %s, try to prepare this batch",
						rbft.chainConfig.SelfID, batchHash)

					var ctx context.Context
					if cert, ok := rbft.storeMgr.certStore[idx]; ok {
						ctx = cert.prePrepareCtx
					} else {
						ctx = context.TODO()
					}

					_ = rbft.findNextPrepareBatch(ctx, idx.v, idx.n, idx.d)
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
func (rbft *rbftImpl[T, Constraint]) rejectRequestSet(req *RequestSet[T, Constraint]) {
	if req.Local {
		rbft.metrics.rejectedLocalTxs.Add(float64(len(req.Requests)))
	} else {
		rbft.metrics.rejectedRemoteTxs.Add(float64(len(req.Requests)))
	}

	// This feature is currently not supported
	// recall promise to avoid memory leak.
	// for _, r := range req.Requests {
	//	if r.Promise != nil {
	//		r.Promise.Recall()
	//	}
	// }
}

// processOutOfDateReqs process the out-of-date requests in requestPool, get the remained txs from pool,
// then broadcast all the remained requests that generate by itself to others
func (rbft *rbftImpl[T, Constraint]) processOutOfDateReqs() {
	// if rbft is in abnormal, reject process remained reqs
	if !rbft.isNormal() {
		rbft.logger.Warningf("Replica %d is in abnormal, reject broadcast remained reqs", rbft.chainConfig.SelfID)
		return
	}

	reqs, err := rbft.batchMgr.requestPool.FilterOutOfDateRequests()
	if err != nil {
		rbft.logger.Warningf("Replica %d get the remained reqs failed, error: %v", rbft.chainConfig.SelfID, err)
	}

	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	reqLen := len(reqs)
	if reqLen == 0 {
		rbft.logger.Debugf("Replica %d in normal finds 0 remained reqs, need not broadcast to others", rbft.chainConfig.SelfID)
		return
	}

	setSize := rbft.config.SetSize
	rbft.logger.Debugf("Replica %d in normal finds %d remained reqs, broadcast to others split by setSize %d "+
		"if needed", rbft.chainConfig.SelfID, reqLen, setSize)

	// not support temporary
	// limit TransactionSet Max Mem by flowControlMaxMem before re-broadcast reqs
	// if rbft.flowControl {
	//	var txs []*consensus.FltTransaction
	//	memLen := 0
	//	for _, tx := range reqs {
	//		txMem := tx.Size()
	//		if memLen+txMem >= rbft.flowControlMaxMem && len(txs) > 0 {
	//			set := &consensus.RequestSet{Requests: txs}
	//			rbft.broadcastReqSet(set)
	//			txs = nil
	//			memLen = 0
	//		}
	//		txs = append(txs, tx)
	//		memLen += txMem
	//	}
	//	if len(txs) > 0 {
	//		set := &consensus.RequestSet{Requests: txs}
	//		rbft.broadcastReqSet(set)
	//	}
	//	return
	// }

	// limit TransactionSet size by setSize before re-broadcast reqs
	for reqLen > 0 {
		if reqLen <= setSize {
			set := &RequestSet[T, Constraint]{Requests: reqs}
			rbft.broadcastReqSet(set)
			reqLen = 0
		} else {
			bTxs := reqs[0:setSize]
			set := &RequestSet[T, Constraint]{Requests: bTxs}
			rbft.broadcastReqSet(set)
			reqs = reqs[setSize:]
			reqLen -= setSize
		}
	}
}

// processNeedRemoveReqs process the checkPoolRemove timeout requests in requestPool, get the remained reqs from pool,
// then remove these txs in local pool
func (rbft *rbftImpl[T, Constraint]) processNeedRemoveReqs() {
	rbft.logger.Infof("removeTx timer expired, Replica %d start remove tx in local txpool ", rbft.chainConfig.SelfID)
	reqLen, err := rbft.batchMgr.requestPool.RemoveTimeoutRequests()
	if err != nil {
		rbft.logger.Warningf("Replica %d get the remained reqs failed, error: %v", rbft.chainConfig.SelfID, err)
	}

	if reqLen == 0 {
		rbft.logger.Infof("Replica %d in normal finds 0 tx to remove", rbft.chainConfig.SelfID)
		return
	}

	// if requestPool is not full, set rbft state to not full
	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}
	rbft.logger.Warningf("Replica %d successful remove %d tx in local txpool ", rbft.chainConfig.SelfID, reqLen)
}

// recvRequestBatch handle logic after receive request batch
func (rbft *rbftImpl[T, Constraint]) recvRequestBatch(reqBatch *txpool.RequestHashBatch[T, Constraint]) error {
	rbft.logger.Debugf("Replica %d received request batch %s", rbft.chainConfig.SelfID, reqBatch.BatchHash)

	batch := &RequestBatch[T, Constraint]{
		RequestHashList: reqBatch.TxHashList,
		RequestList:     reqBatch.TxList,
		Timestamp:       reqBatch.Timestamp,
		SeqNo:           rbft.batchMgr.getSeqNo() + 1,
		LocalList:       reqBatch.LocalList,
		BatchHash:       reqBatch.BatchHash,
		Proposer:        rbft.chainConfig.SelfID,
	}

	// primary node should reject generate batch when there is a config batch in ordering.
	if rbft.isPrimary(rbft.chainConfig.SelfID) && rbft.isNormal() && !rbft.atomicIn(InConfChange) && rbft.inPrimaryTerm() {
		// enter config change status once we generate a config batch.
		if isConfigBatch(batch.SeqNo, rbft.chainConfig.EpochInfo) {
			rbft.logger.Noticef("Primary %d has generated a config batch, start config change", rbft.chainConfig.SelfID)
			rbft.atomicOn(InConfChange)
			rbft.metrics.statusGaugeInConfChange.Set(InConfChange)
		}
		rbft.restartBatchTimer()
		rbft.restartNoTxBatchTimer()
		rbft.timerMgr.stopTimer(nullRequestTimer)
		if len(rbft.batchMgr.cacheBatch) > 0 {
			rbft.batchMgr.cacheBatch = append(rbft.batchMgr.cacheBatch, batch)
			rbft.metrics.cacheBatchNumber.Add(float64(1))
			rbft.maybeSendPrePrepare(nil, true)
			return nil
		}
		rbft.maybeSendPrePrepare(batch, false)
	} else {
		rbft.logger.Debugf("Replica %d not try to send prePrepare for request batch %s", rbft.chainConfig.SelfID, reqBatch.BatchHash)
		_ = rbft.batchMgr.requestPool.RestoreOneBatch(reqBatch.BatchHash)
	}

	return nil
}

// =============================================================================
// normal case: pre-prepare, prepare, commit methods
// =============================================================================
// sendPrePrepare send prePrepare message.
func (rbft *rbftImpl[T, Constraint]) sendPrePrepare(seqNo uint64, digest string, reqBatch *RequestBatch[T, Constraint]) {
	rbft.logger.Debugf("Primary %d sending prePrepare for view=%d/seqNo=%d/digest=%s, "+
		"batch size: %d, timestamp: %d", rbft.chainConfig.SelfID, rbft.chainConfig.View, seqNo, digest, len(reqBatch.RequestHashList), reqBatch.Timestamp)

	ctx, span := rbft.tracer.Start(context.Background(), "sendPrePrepare")
	span.SetAttributes(attribute.Int64("seqNo", int64(seqNo)), attribute.String("digest", digest))
	defer span.End()

	hashBatch := &consensus.HashBatch{
		RequestHashList: reqBatch.RequestHashList,
		Timestamp:       reqBatch.Timestamp,
		Proposer:        reqBatch.Proposer,
	}

	preprepare := &consensus.PrePrepare{
		View:           rbft.chainConfig.View,
		SequenceNumber: seqNo,
		BatchDigest:    digest,
		HashBatch:      hashBatch,
		ReplicaId:      rbft.chainConfig.SelfID,
	}

	cert := rbft.storeMgr.getCert(rbft.chainConfig.View, seqNo, digest)
	cert.isConfig = isConfigBatch(reqBatch.SeqNo, rbft.chainConfig.EpochInfo)
	cert.prePrepare = preprepare
	cert.prePrepareCtx = ctx
	rbft.persistQSet(preprepare)
	if metrics.EnableExpensive() {
		cert.prePreparedTime = time.Now().UnixNano()
		duration := time.Duration(cert.prePreparedTime - reqBatch.Timestamp).Seconds()
		rbft.metrics.batchToPrePrepared.Observe(duration)
	}

	payload, err := preprepare.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_PRE_PREPARE Marshal Error: %s", err)
		span.RecordError(err)
		return
	}
	span.AddEvent("Marshal preprepare")
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_PRE_PREPARE,
		Payload: payload,
	}
	rbft.peerMgr.broadcast(ctx, consensusMsg)

	// set primary's seqNo to current batch seqNo
	rbft.batchMgr.setSeqNo(seqNo)

	// exit sync state as primary is ready to process requests.
	rbft.exitSyncState()
}

// recvPrePrepare process logic for PrePrepare msg.
func (rbft *rbftImpl[T, Constraint]) recvPrePrepare(ctx context.Context, preprep *consensus.PrePrepare) error {
	ctx, span := rbft.tracer.Start(ctx, "recvPrePrepare")
	defer span.End()

	rbft.logger.Debugf("Replica %d received prePrepare from replica %d for view=%d/seqNo=%d, digest=%s ",
		rbft.chainConfig.SelfID, preprep.ReplicaId, preprep.View, preprep.SequenceNumber, preprep.BatchDigest)

	if !rbft.isPrePrepareLegal(preprep) {
		if rbft.chainConfig.isProposerElectionTypeWRF() &&
			preprep.View > rbft.chainConfig.View &&
			!rbft.beyondRange(preprep.SequenceNumber) {
			// only for wrf: cache messages that are higher in the view and do not exceed the high watermark, recommit after checkpoint
			cacheMsg := rbft.getWRFHighViewMsgCache(preprep.View)
			cacheMsg.prePrepares = append(cacheMsg.prePrepares, preprep)
			rbft.logger.Debugf("Replica %d received higher view prePrepare from replica %d and cache it, "+
				"receive view %d, current view %d, seqNo %d, low water mark %d",
				rbft.chainConfig.SelfID, preprep.ReplicaId, preprep.View, rbft.chainConfig.View, preprep.SequenceNumber, rbft.chainConfig.H)
		} else {
			if preprep.View > rbft.chainConfig.View {
				rbft.logger.Warningf("Replica %d ignore too higher view prePrepare from replica %d, "+
					"receive view %d, current view %d, seqNo %d, low water mark %d",
					rbft.chainConfig.SelfID, preprep.ReplicaId, preprep.View, rbft.chainConfig.View, preprep.SequenceNumber, rbft.chainConfig.H)
			}
		}
		return nil
	}

	digest, ok := rbft.storeMgr.seqMap[preprep.SequenceNumber]
	if ok {
		if digest != preprep.BatchDigest {
			rbft.logger.Warningf("Replica %d found same view/seqNo but different digest, received: %s, stored: %s",
				rbft.chainConfig.SelfID, preprep.BatchDigest, digest)
			rbft.sendViewChange()
			return nil
		}
	}

	if rbft.beyondRange(preprep.SequenceNumber) {
		rbft.logger.Debugf("Replica %d received a pre-prepare out of high-watermark", rbft.chainConfig.SelfID)
		rbft.softStartHighWatermarkTimer("replica received a pre-prepare out of range")
	}

	if preprep.BatchDigest == "" {
		if len(preprep.HashBatch.RequestHashList) != 0 {
			rbft.logger.Warningf("Replica %d received a prePrepare with an empty digest but batch is "+
				"not empty", rbft.chainConfig.SelfID)
			rbft.sendViewChange()
			return nil
		}
	} else {
		if len(preprep.HashBatch.DeDuplicateRequestHashList) != 0 {
			rbft.logger.Noticef("Replica %d finds %d duplicate txs with digest %s, detailed: %+v",
				rbft.chainConfig.SelfID, len(preprep.HashBatch.DeDuplicateRequestHashList), preprep.HashBatch.DeDuplicateRequestHashList)
		}
		// check if the digest sent from primary is really the hash of txHashList, if not, don't
		// send prepare for this prePrepare
		batchDigest := calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)
		if batchDigest != preprep.BatchDigest {
			rbft.logger.Warningf("Replica %d received a prePrepare with a wrong batch digest, calculated: %s "+
				"primary calculated: %s, send viewChange", rbft.chainConfig.SelfID, batchDigest, preprep.BatchDigest)
			rbft.sendViewChange()
			return nil
		}
	}

	// in recovery, we would fetch recovery PQC, and receive these PQC again,
	// and we cannot stop timer in this situation, so we check seqNo here.
	if preprep.SequenceNumber > rbft.exec.lastExec {
		rbft.timerMgr.stopTimer(nullRequestTimer)
	}

	cert := rbft.storeMgr.getCert(preprep.View, preprep.SequenceNumber, preprep.BatchDigest)
	cert.prePrepare = preprep
	cert.prePrepareCtx = ctx
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

	if !rbft.isPrimary(rbft.chainConfig.SelfID) && !cert.sentPrepare {
		return rbft.findNextPrepareBatch(ctx, preprep.View, preprep.SequenceNumber, preprep.BatchDigest)
	}

	return nil
}

// sendPrepare send prepare message.
func (rbft *rbftImpl[T, Constraint]) sendPrepare(ctx context.Context, v uint64, n uint64, d string) error {
	ctx, span := rbft.tracer.Start(ctx, "sendPrepare")
	defer span.End()

	cert := rbft.storeMgr.getCert(v, n, d)
	cert.sentPrepare = true

	rbft.logger.Debugf("Replica %d sending prepare for view=%d/seqNo=%d", rbft.chainConfig.SelfID, v, n)
	prep := &consensus.Prepare{
		View:           v,
		SequenceNumber: n,
		BatchDigest:    d,
		ReplicaId:      rbft.chainConfig.SelfID,
	}

	if rbft.chainConfig.isValidator() {
		payload, err := prep.MarshalVTStrict()
		if err != nil {
			rbft.logger.Errorf("ConsensusMessage_PREPARE Marshal Error: %s", err)
			return nil
		}
		consensusMsg := &consensus.ConsensusMessage{
			Type:    consensus.Type_PREPARE,
			Payload: payload,
		}
		rbft.peerMgr.broadcast(ctx, consensusMsg)
	} else {
		rbft.logger.Debugf("Replica %d is not validator, not broadcast prepare",
			rbft.chainConfig.SelfID)
	}

	// send to itself
	return rbft.recvPrepare(ctx, prep)
}

// recvPrepare process logic after receive prepare message
func (rbft *rbftImpl[T, Constraint]) recvPrepare(ctx context.Context, prep *consensus.Prepare) error {
	ctx, span := rbft.tracer.Start(ctx, "recvPrepare")
	defer span.End()

	rbft.logger.Debugf("Replica %d received prepare from replica %d for view=%d/seqNo=%d",
		rbft.chainConfig.SelfID, prep.ReplicaId, prep.View, prep.SequenceNumber)

	if !rbft.isPrepareLegal(prep) {
		if rbft.chainConfig.isProposerElectionTypeWRF() &&
			prep.View > rbft.chainConfig.View &&
			!rbft.beyondRange(prep.SequenceNumber) {
			// only for wrf: cache messages that are higher in the view and do not exceed the high watermark, recommit after checkpoint
			cacheMsg := rbft.getWRFHighViewMsgCache(prep.View)
			cacheMsg.prepares = append(cacheMsg.prepares, prep)
			rbft.logger.Debugf("Replica %d received higher view prepare from replica %d and cache it, "+
				"receive view %d, current view %d, seqNo %d, low water mark %d",
				rbft.chainConfig.SelfID, prep.ReplicaId, prep.View, rbft.chainConfig.View, prep.SequenceNumber, rbft.chainConfig.H)
		} else {
			if prep.View > rbft.chainConfig.View {
				rbft.logger.Warningf("Replica %d ignore too higher view prepare from replica %d, "+
					"receive view %d, current view %d, seqNo %d, low water mark %d",
					rbft.chainConfig.SelfID, prep.ReplicaId, prep.View, rbft.chainConfig.View, prep.SequenceNumber, rbft.chainConfig.H)
			}
		}

		return nil
	}

	cert := rbft.storeMgr.getCert(prep.View, prep.SequenceNumber, prep.BatchDigest)
	prepID := prep.ID()
	_, ok := cert.prepare[prepID]
	if ok {
		if prep.SequenceNumber <= rbft.exec.lastExec {
			rbft.logger.Debugf("Replica %d received duplicate prepare from replica %d, view=%d/seqNo=%d, self lastExec=%d",
				rbft.chainConfig.SelfID, prep.ReplicaId, prep.View, prep.SequenceNumber, rbft.exec.lastExec)
			return nil
		}
		// this is abnormal in consensus case
		rbft.logger.Infof("Replica %d ignore duplicate prepare from replica %d, view=%d/seqNo=%d",
			rbft.chainConfig.SelfID, prep.ReplicaId, prep.View, prep.SequenceNumber)
		return nil
	}
	if !rbft.chainConfig.isValidator() && prep.ReplicaId == rbft.chainConfig.SelfID {
		rbft.logger.Debugf("Replica %d is not validator, not store self prepare msg to cert",
			rbft.chainConfig.SelfID)
	} else {
		cert.prepare[prepID] = prep
	}

	return rbft.maybeSendCommit(ctx, prep.View, prep.SequenceNumber, prep.BatchDigest)
}

// maybeSendCommit check if we could send commit. if no problem,
// primary and replica would send commit.
func (rbft *rbftImpl[T, Constraint]) maybeSendCommit(ctx context.Context, v uint64, n uint64, d string) error {
	if rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d do not try to send commit because it's in stateUpdate", rbft.chainConfig.SelfID)
		return nil
	}

	cert := rbft.storeMgr.getCert(v, n, d)
	if cert == nil {
		rbft.logger.Errorf("Replica %d can't get the cert for the view=%d/seqNo=%d/digest=%s", rbft.chainConfig.SelfID, v, n, d)
		return nil
	}

	if !rbft.prepared(v, n, d) {
		return nil
	}

	if !rbft.isPrimary(rbft.chainConfig.SelfID) && !cert.sentPrepare {
		rbft.logger.Debugf("Replica %d cert hasn't sent prepare, cancel maybeSendCommit", rbft.chainConfig.SelfID)
		return nil
	}

	if cert.sentCommit {
		rbft.logger.Debugf("Replica %d cert is committed, cancel maybeSendCommit", rbft.chainConfig.SelfID)
		return nil
	}

	if metrics.EnableExpensive() {
		cert.preparedTime = time.Now().UnixNano()
		duration := time.Duration(cert.preparedTime - cert.prePreparedTime).Seconds()
		rbft.metrics.prePreparedToPrepared.Observe(duration)
	}
	return rbft.sendCommit(ctx, v, n, d)
}

// sendCommit send commit message.
func (rbft *rbftImpl[T, Constraint]) sendCommit(ctx context.Context, v uint64, n uint64, d string) error {
	ctx, span := rbft.tracer.Start(ctx, "sendCommit")
	defer span.End()

	cert := rbft.storeMgr.getCert(v, n, d)
	cert.sentCommit = true

	rbft.logger.Debugf("Replica %d sending commit for view=%d/seqNo=%d", rbft.chainConfig.SelfID, v, n)
	commit := &consensus.Commit{
		View:           v,
		SequenceNumber: n,
		BatchDigest:    d,
		ReplicaId:      rbft.chainConfig.SelfID,
	}

	rbft.persistPSet(v, n, d)

	if rbft.chainConfig.isValidator() {
		payload, err := commit.MarshalVTStrict()
		if err != nil {
			rbft.logger.Errorf("ConsensusMessage_COMMIT Marshal Error: %s", err)
			return nil
		}
		consensusMsg := &consensus.ConsensusMessage{
			Type:    consensus.Type_COMMIT,
			Payload: payload,
		}
		rbft.peerMgr.broadcast(ctx, consensusMsg)
	} else {
		rbft.logger.Debugf("Replica %d is not validator, not broadcast commit",
			rbft.chainConfig.SelfID)
	}

	return rbft.recvCommit(ctx, commit)
}

// recvCommit process logic after receive commit message.
func (rbft *rbftImpl[T, Constraint]) recvCommit(ctx context.Context, commit *consensus.Commit) error {
	_, span := rbft.tracer.Start(ctx, "recvCommit")
	defer span.End()

	rbft.logger.Debugf("Replica %d received commit from replica %d for view=%d/seqNo=%d",
		rbft.chainConfig.SelfID, commit.ReplicaId, commit.View, commit.SequenceNumber)

	if !rbft.isCommitLegal(commit) {
		if rbft.chainConfig.isProposerElectionTypeWRF() &&
			commit.View > rbft.chainConfig.View &&
			!rbft.beyondRange(commit.SequenceNumber) {
			// only for wrf: cache messages that are higher in the view and do not exceed the high watermark, recommit after checkpoint
			cacheMsg := rbft.getWRFHighViewMsgCache(commit.View)
			cacheMsg.commits = append(cacheMsg.commits, commit)
			rbft.logger.Debugf("Replica %d received higher view commit from replica %d and cache it, "+
				"receive view %d, current view %d, seqNo %d, low water mark %d",
				rbft.chainConfig.SelfID, commit.ReplicaId, commit.View, rbft.chainConfig.View, commit.SequenceNumber, rbft.chainConfig.H)
		} else {
			if commit.View > rbft.chainConfig.View {
				rbft.logger.Warningf("Replica %d ignore too higher view commit from replica %d, "+
					"receive view %d, current view %d, seqNo %d, low water mark %d",
					rbft.chainConfig.SelfID, commit.ReplicaId, commit.View, rbft.chainConfig.View, commit.SequenceNumber, rbft.chainConfig.H)
			}
		}
		return nil
	}

	cert := rbft.storeMgr.getCert(commit.View, commit.SequenceNumber, commit.BatchDigest)
	commitID := commit.ID()
	_, ok := cert.commit[commitID]
	if ok {
		if commit.SequenceNumber <= rbft.exec.lastExec {
			// ignore duplicate commit with seqNo <= lastExec as this commit is not useful forever.
			rbft.logger.Debugf("Replica %d received duplicate commit from replica %d, view=%d/seqNo=%d "+
				"but current lastExec is %d, ignore it...", rbft.chainConfig.SelfID, commit.ReplicaId, commit.View, commit.SequenceNumber,
				rbft.exec.lastExec)
			return nil
		}
		// we can simply accept all commit messages whose seqNo is larger than our lastExec as we
		// haven't executed this batch, we can ensure that we will only execute this batch once.
		rbft.logger.Debugf("Replica %d accept duplicate commit from replica %d, view=%d/seqNo=%d "+
			"current lastExec is %d", rbft.chainConfig.SelfID, commit.ReplicaId, commit.View, commit.SequenceNumber, rbft.exec.lastExec)
	}

	if !rbft.chainConfig.isValidator() && commit.ReplicaId == rbft.chainConfig.SelfID {
		rbft.logger.Debugf("Replica %d is not validator, not store self commit msg to cert",
			rbft.chainConfig.SelfID)
	} else {
		cert.commit[commitID] = commit
	}

	if rbft.committed(commit.View, commit.SequenceNumber, commit.BatchDigest) {
		idx := msgID{v: commit.View, n: commit.SequenceNumber, d: commit.BatchDigest}
		if metrics.EnableExpensive() {
			cert.committedTime = time.Now().UnixNano()
			duration := time.Duration(cert.committedTime - cert.preparedTime).Seconds()
			rbft.metrics.preparedToCommitted.Observe(duration)
		}
		if !cert.sentExecute && cert.sentCommit {
			rbft.storeMgr.committedCert[idx] = commit.BatchDigest
			rbft.commitPendingBlocks()

			if !rbft.in(waitCheckpointBatchExecute) {
				// reset last new view timeout after commit one block successfully.
				rbft.vcMgr.lastNewViewTimeout = rbft.timerMgr.getTimeoutValue(newViewTimer)
				if commit.SequenceNumber == rbft.vcMgr.viewChangeSeqNo {
					rbft.logger.Warningf("Replica %d cycling view for seqNo=%d", rbft.chainConfig.SelfID, commit.SequenceNumber)
					rbft.sendViewChange()
				}
			}
		} else {
			rbft.logger.Debugf("Replica %d committed for seqNo: %d, but sentExecute: %v",
				rbft.chainConfig.SelfID, commit.SequenceNumber, cert.sentExecute)
		}
	}
	return nil
}

// fetchMissingTxs fetch missing txs from primary which this node didn't receive but primary received
func (rbft *rbftImpl[T, Constraint]) fetchMissingTxs(ctx context.Context, prePrep *consensus.PrePrepare, missingTxHashes map[uint64]string) {
	// avoid fetch the same batch again.
	if _, ok := rbft.storeMgr.missingBatchesInFetching[prePrep.BatchDigest]; ok {
		return
	}

	rbft.logger.Debugf("Replica %d try to fetch missing txs for view=%d/seqNo=%d/digest=%s from primary %d",
		rbft.chainConfig.SelfID, prePrep.View, prePrep.SequenceNumber, prePrep.BatchDigest, prePrep.ReplicaId)

	ctx, span := rbft.tracer.Start(ctx, "fetchMissingTxs")
	defer span.End()

	fetch := &consensus.FetchMissingRequest{
		View:                 prePrep.View,
		SequenceNumber:       prePrep.SequenceNumber,
		BatchDigest:          prePrep.BatchDigest,
		MissingRequestHashes: missingTxHashes,
		ReplicaId:            rbft.chainConfig.SelfID,
	}

	payload, err := fetch.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_FetchMissingRequest Marshal Error: %s", err)
		return
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_FETCH_MISSING_REQUEST,
		Payload: payload,
	}
	rbft.metrics.fetchMissingTxsCounter.Add(float64(1))
	rbft.storeMgr.missingBatchesInFetching[prePrep.BatchDigest] = msgID{
		v: prePrep.View,
		n: prePrep.SequenceNumber,
		d: prePrep.BatchDigest,
	}
	rbft.peerMgr.unicast(ctx, consensusMsg, prePrep.ReplicaId)
}

// recvFetchMissingRequest returns txs to a node which didn't receive some txs and ask primary for them.
func (rbft *rbftImpl[T, Constraint]) recvFetchMissingRequest(ctx context.Context, fetch *consensus.FetchMissingRequest) error {
	rbft.logger.Debugf("Primary %d received fetchMissing request for view=%d/seqNo=%d/digest=%s "+
		"from replica %d", rbft.chainConfig.SelfID, fetch.View, fetch.SequenceNumber, fetch.BatchDigest, fetch.ReplicaId)

	ctx, span := rbft.tracer.Start(ctx, "recvFetchMissingRequest")
	defer span.End()

	requests := make(map[uint64][]byte)
	var err error

	if batch := rbft.storeMgr.batchStore[fetch.BatchDigest]; batch != nil {
		batchLen := uint64(len(batch.RequestHashList))
		for i, hash := range fetch.MissingRequestHashes {
			if i >= batchLen || batch.RequestHashList[i] != hash {
				rbft.logger.Errorf("Primary %d finds mismatch requests hash when return "+
					"fetch missing requests", rbft.chainConfig.SelfID)
				return nil
			}
			requests[i], err = Constraint(batch.RequestList[i]).RbftMarshal()
			if err != nil {
				rbft.logger.Errorf("Tx marshal Error: %s", err)
				return nil
			}
		}
	} else {
		var missingTxs map[uint64]*T
		missingTxs, err = rbft.batchMgr.requestPool.SendMissingRequests(fetch.BatchDigest, fetch.MissingRequestHashes)
		if err != nil {
			rbft.logger.Warningf("Primary %d cannot find the digest %s, missing tx hashes: %+v, err: %s",
				rbft.chainConfig.SelfID, fetch.BatchDigest, fetch.MissingRequestHashes, err)
			return nil
		}
		for i, tx := range missingTxs {
			requests[i], err = Constraint(tx).RbftMarshal()
			if err != nil {
				rbft.logger.Errorf("Tx marshal Error: %s", err)
				return nil
			}
		}
	}

	re := &consensus.FetchMissingResponse{
		View:                 fetch.View,
		SequenceNumber:       fetch.SequenceNumber,
		BatchDigest:          fetch.BatchDigest,
		MissingRequestHashes: fetch.MissingRequestHashes,
		MissingRequests:      requests,
		ReplicaId:            rbft.chainConfig.SelfID,
	}

	payload, err := re.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_FetchMissingResponse Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_FETCH_MISSING_RESPONSE,
		Payload: payload,
	}
	rbft.metrics.returnFetchMissingTxsCounter.With("node", strconv.Itoa(int(fetch.ReplicaId))).Add(float64(1))
	rbft.peerMgr.unicast(ctx, consensusMsg, fetch.ReplicaId)

	return nil
}

// recvFetchMissingResponse processes SendMissingTxs from primary.
// Add these transaction txs to requestPool and see if it has correct transaction txs.
func (rbft *rbftImpl[T, Constraint]) recvFetchMissingResponse(ctx context.Context, re *consensus.FetchMissingResponse) consensusEvent {
	ctx, span := rbft.tracer.Start(ctx, "recvFetchMissingResponse")
	defer span.End()

	if _, ok := rbft.storeMgr.missingBatchesInFetching[re.BatchDigest]; !ok {
		rbft.logger.Debugf("Replica %d ignore fetchMissingResponse with batch hash %s",
			rbft.chainConfig.SelfID, re.BatchDigest)
		return nil
	}

	rbft.logger.Debugf("Replica %d received fetchMissingResponse for view=%d/seqNo=%d/digest=%s from replica %d",
		rbft.chainConfig.SelfID, re.View, re.SequenceNumber, re.BatchDigest, re.ReplicaId)

	if re.SequenceNumber < rbft.exec.lastExec {
		rbft.logger.Debugf("Replica %d ignore fetchMissingResponse with lower seqNo %d than "+
			"lastExec %d", rbft.chainConfig.SelfID, re.SequenceNumber, rbft.exec.lastExec)
		return nil
	}

	if len(re.MissingRequests) != len(re.MissingRequestHashes) {
		rbft.logger.Warningf("Replica %d received mismatch length fetchMissingResponse %v", rbft.chainConfig.SelfID, re)
		return nil
	}

	if !rbft.inV(re.View) {
		rbft.logger.Debugf("Replica %d received fetchMissingResponse which has a different view=%d, "+
			"expected view=%d, ignore it", rbft.chainConfig.SelfID, re.View, rbft.chainConfig.View)
		return nil
	}

	if !rbft.isPrimary(re.ReplicaId) {
		rbft.logger.Warningf("Replica %d received fetchMissingResponse from replica %d which is not "+
			"primary, ignore it", rbft.chainConfig.SelfID, re.ReplicaId)
		return nil
	}

	cert := rbft.storeMgr.getCert(re.View, re.SequenceNumber, re.BatchDigest)
	if cert.sentCommit {
		rbft.logger.Debugf("Replica %d received fetchMissingResponse which has been committed with "+
			"cert view=%d/seqNo=%d/digest=%s, ignore it", rbft.chainConfig.SelfID, re.View, re.SequenceNumber, re.BatchDigest)
		return nil
	}
	if cert.prePrepare == nil {
		rbft.logger.Warningf("Replica %d had not received a prePrepare before for view=%d/seqNo=%d",
			rbft.chainConfig.SelfID, re.View, re.SequenceNumber)
		return nil
	}

	requests := make(map[uint64]*T)
	for i, reqRaw := range re.MissingRequests {
		var req T
		if err := Constraint(&req).RbftUnmarshal(reqRaw); err != nil {
			rbft.logger.Errorf("Tx unmarshal Error: %s", err)
			return nil
		}
		requests[i] = &req
	}

	err := rbft.batchMgr.requestPool.ReceiveMissingRequests(re.BatchDigest, requests)
	if err != nil {
		// there is something wrong with primary for it propose a transaction with mismatched hash,
		// so that we should send view-change directly to expect a new leader.
		rbft.logger.Warningf("Replica %d find something wrong with fetchMissingResponse, error: %v",
			rbft.chainConfig.SelfID, err)
		return rbft.sendViewChange()
	}

	// set pool full status if received txs fill up the txpool.
	if rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setFull()
	}

	_ = rbft.findNextPrepareBatch(ctx, re.View, re.SequenceNumber, re.BatchDigest)
	return nil
}

// =============================================================================
// execute transactions
// =============================================================================

// commitPendingBlocks commit all available transactions by order
func (rbft *rbftImpl[T, Constraint]) commitPendingBlocks() {
	rbft.logger.Debugf("Replica %d attempting to commitTransactions", rbft.chainConfig.SelfID)

	foreachIdx := 0
	for hasTxToExec := true; hasTxToExec; {
		find, idx, cert := rbft.findNextCommitBatch()
		if find {
			rbft.metrics.committedBlockNumber.Add(float64(1))
			rbft.persistCSet(idx.v, idx.n, idx.d)
			// stop new view timer after one batch has been call executed
			rbft.stopNewViewTimer()
			if ok := rbft.isPrimary(rbft.chainConfig.SelfID); ok {
				rbft.softRestartBatchTimer()
			}

			var proposerAccount string
			if cert != nil && cert.prePrepare != nil {
				proposer, ok := rbft.chainConfig.NodeInfoMap[cert.prePrepare.HashBatch.Proposer]
				if ok {
					proposerAccount = proposer.AccountAddress
				} else {
					rbft.logger.Warningf("Replica %d did not find the proposer in the epoch", rbft.chainConfig.SelfID)
				}
			}

			if idx.d == "" {
				txList := make([]*T, 0)
				localList := make([]bool, 0)
				rbft.metrics.committedEmptyBlockNumber.Add(float64(1))
				rbft.metrics.txsPerBlock.Observe(float64(0))
				rbft.logger.Noticef("======== Replica %d Call execute a no-nop, epoch=%d/view=%d/seqNo=%d",
					rbft.chainConfig.SelfID, rbft.chainConfig.EpochInfo.Epoch, idx.v, idx.n)

				rbft.external.Execute(txList, localList, idx.n, 0, proposerAccount)
			} else {
				// find batch in batchStore rather than outstandingBatch as after viewChange
				// we may clear outstandingBatch and save all batches in batchStore.
				// kick out de-duplicate txs if needed.
				if cert.isConfig {
					rbft.logger.Debugf("Replica %d found a config batch, set config batch number to %d",
						rbft.chainConfig.SelfID, idx.n)
					rbft.setConfigBatchToExecute(idx.n)
					rbft.metrics.committedConfigBlockNumber.Add(float64(1))
				}
				txList, localList := rbft.filterExecutableTxs(idx.d, cert.prePrepare.HashBatch.DeDuplicateRequestHashList)
				rbft.metrics.committedTxs.Add(float64(len(txList)))
				rbft.metrics.txsPerBlock.Observe(float64(len(txList)))
				batchToCommit := time.Duration(time.Now().UnixNano() - cert.prePrepare.HashBatch.Timestamp).Seconds()
				rbft.metrics.batchToCommitDuration.Observe(batchToCommit)
				rbft.logger.Noticef("======== Replica %d Call execute, epoch=%d/view=%d/seqNo=%d/txCount=%d/digest=%s",
					rbft.chainConfig.SelfID, rbft.chainConfig.EpochInfo.Epoch, idx.v, idx.n, len(txList), idx.d)
				rbft.external.Execute(txList, localList, idx.n, cert.prePrepare.HashBatch.Timestamp, proposerAccount)
			}
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			rbft.metrics.outstandingBatchesGauge.Set(float64(len(rbft.storeMgr.outstandingReqBatches)))
			cert.sentExecute = true

			// if it is a config batch, start to wait for stable checkpoint process after the batch committed
			rbft.afterCommitBlock(idx, cert.isConfig)
			foreachIdx++
		} else {
			hasTxToExec = false
		}
	}
	rbft.logger.Debugf("Replica %d attempting to commitTransactions finished, times=%d", rbft.chainConfig.SelfID, foreachIdx)

	if !rbft.in(waitCheckpointBatchExecute) {
		rbft.startTimerIfOutstandingRequests()
	}
}

// filterExecutableTxs flatten txs into txs and kick out duplicate txs with hash included in deDuplicateTxHashes.
func (rbft *rbftImpl[T, Constraint]) filterExecutableTxs(digest string, deDuplicateRequestHashes []string) ([]*T, []bool) {
	var (
		txList, executableTxs          []*T
		localList, executableLocalList []bool
	)
	txList = rbft.storeMgr.batchStore[digest].RequestList
	localList = rbft.storeMgr.batchStore[digest].LocalList
	dupHashes := make(map[string]bool)
	for _, dupHash := range deDuplicateRequestHashes {
		dupHashes[dupHash] = true
	}
	for i, request := range txList {
		reqHash := Constraint(request).RbftGetTxHash()
		if dupHashes[reqHash] {
			rbft.logger.Noticef("Replica %d kick out de-duplicate request %s before execute batch %s", rbft.chainConfig.SelfID, reqHash, digest)
			continue
		}
		executableTxs = append(executableTxs, request)
		executableLocalList = append(executableLocalList, localList[i])
	}
	return executableTxs, executableLocalList
}

// findNextCommitBatch find next msgID which is able to commit.
func (rbft *rbftImpl[T, Constraint]) findNextCommitBatch() (find bool, idx msgID, cert *msgCert) {
	if rbft.in(waitCheckpointBatchExecute) {
		return false, msgID{}, nil
	}
	for idx = range rbft.storeMgr.committedCert {
		cert = rbft.storeMgr.certStore[idx]

		if cert == nil || cert.prePrepare == nil {
			rbft.logger.Debugf("Replica %d already checkpoint for view=%d/seqNo=%d", rbft.chainConfig.SelfID, idx.v, idx.n)
			continue
		}

		// check if already executed
		if cert.sentExecute {
			rbft.logger.Debugf("Replica %d already execute for view=%d/seqNo=%d", rbft.chainConfig.SelfID, idx.v, idx.n)
			continue
		}

		if idx.n != rbft.exec.lastExec+1 {
			rbft.logger.Debugf("Replica %d expects to execute seq=%d, but get seq=%d, ignore it", rbft.chainConfig.SelfID, rbft.exec.lastExec+1, idx.n)
			continue
		}

		// skipInProgress == true, then this replica is in viewchange, not reply or execute
		if rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d currently picking a starting point to resume, will not execute", rbft.chainConfig.SelfID)
			continue
		}

		// check if committed
		if !rbft.committed(idx.v, idx.n, idx.d) {
			continue
		}

		if idx.d != "" {
			_, ok := rbft.storeMgr.batchStore[idx.d]
			if !ok {
				rbft.logger.Warningf("Replica %d cannot find corresponding batch %s in batchStore", rbft.chainConfig.SelfID, idx.d)
				continue
			}
		}

		find = true
		break
	}

	return
}

// afterCommitBlock processes logic after commit block, update lastExec,
// and generate checkpoint when lastExec % K == 0
func (rbft *rbftImpl[T, Constraint]) afterCommitBlock(idx msgID, isConfig bool) {
	rbft.logger.Debugf("Replica %d finished execution %d, trying next", rbft.chainConfig.SelfID, idx.n)
	rbft.exec.setLastExec(idx.n)
	delete(rbft.storeMgr.committedCert, idx)
	// not checkpoint
	if isConfig || rbft.exec.lastExec%rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod == 0 {
		if !rbft.isTest {
			rbft.logger.Debugf("Replica %d start wait checkpoint batch=%d executed", rbft.chainConfig.SelfID, rbft.exec.lastExec)
			rbft.on(waitCheckpointBatchExecute)
		} else {
			// for test, auto report checkpoint
			rbft.recvCheckpointBlockExecutedEvent(&types.ServiceState{
				MetaState: &types.MetaState{
					Height: idx.n,
					Digest: idx.d,
				},
				Epoch: rbft.epochMgr.epoch,
			})
		}
	}
}

func (rbft *rbftImpl[T, Constraint]) reProcessCacheEvents() {
	c := rbft.storeMgr.beforeCheckpointEventCache
	// reset
	rbft.storeMgr.beforeCheckpointEventCache = []consensusEvent{}

	for _, msg := range c {
		cm, isConsensusMessage := msg.(*consensusMessageWrapper)
		for {
			select {
			case <-rbft.close:
				rbft.logger.Notice("exit RBFT event listener")
				return
			default:
			}
			msg = rbft.processEvent(msg)
			if msg == nil {
				break
			}
		}
		// check view after finished process consensus messages from remote node as current view may
		// be changed because of above consensus messages.
		if isConsensusMessage {
			rbft.checkView(cm.ConsensusMessage)
		}
	}
}

func (rbft *rbftImpl[T, Constraint]) recvCheckpointBlockExecutedEvent(state *types.ServiceState) {
	// after execute a block, there are 3 cases:
	// 1. a config transaction: waiting for checkpoint channel and turn into epoch process
	// 2. a normal transaction in checkpoint: waiting for checkpoint channel and turn into checkpoint process
	// 3. a normal transaction not in checkpoint: finish directly
	if isConfigBatch(state.MetaState.Height, rbft.chainConfig.EpochInfo) {
		// reset config transaction to execute
		rbft.resetConfigBatchToExecute()
		if state.MetaState.Height == rbft.exec.lastExec {
			rbft.logger.Debugf("Call the checkpoint for config batch, seqNo=%d", rbft.exec.lastExec)
			rbft.epochMgr.configBatchToCheck = state.MetaState
			rbft.off(waitCheckpointBatchExecute)
			rbft.checkpoint(state, true)

			rbft.startTimerIfOutstandingRequests()

			// reset last new view timeout after commit one block successfully.
			rbft.vcMgr.lastNewViewTimeout = rbft.timerMgr.getTimeoutValue(newViewTimer)
			if state.MetaState.Height == rbft.vcMgr.viewChangeSeqNo {
				rbft.logger.Warningf("Replica %d cycling view for seqNo=%d", rbft.chainConfig.SelfID, state.MetaState.Height)
				rbft.sendViewChange()
			}

			rbft.logger.Debugf("Replica %d start reprocess cache events after checkpoint batch, seqNo=%d, cache events size=%d", rbft.chainConfig.SelfID, rbft.exec.lastExec, len(rbft.storeMgr.beforeCheckpointEventCache))
			rbft.reProcessCacheEvents()
			rbft.logger.Debugf("Replica %d after reprocess cache events after checkpoint batch, seqNo=%d", rbft.chainConfig.SelfID, rbft.exec.lastExec)
		} else {
			// reqBatch call execute but have not done with execute
			rbft.logger.Errorf("Fail to call the checkpoint, seqNo=%d", rbft.exec.lastExec)
		}
	} else if state.MetaState.Height%rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod == 0 {
		if state.MetaState.Height == rbft.exec.lastExec {
			rbft.logger.Debugf("Call the checkpoint for normal, seqNo=%d", rbft.exec.lastExec)
			rbft.off(waitCheckpointBatchExecute)
			rbft.checkpoint(state, false)

			rbft.startTimerIfOutstandingRequests()

			// reset last new view timeout after commit one block successfully.
			rbft.vcMgr.lastNewViewTimeout = rbft.timerMgr.getTimeoutValue(newViewTimer)
			if state.MetaState.Height == rbft.vcMgr.viewChangeSeqNo {
				rbft.logger.Warningf("Replica %d cycling view for seqNo=%d", rbft.chainConfig.SelfID, state.MetaState.Height)
				rbft.sendViewChange()
			}

			rbft.logger.Debugf("Replica %d start reprocess cache events after checkpoint batch, seqNo=%d, cache events size=%d", rbft.chainConfig.SelfID, rbft.exec.lastExec, len(rbft.storeMgr.beforeCheckpointEventCache))
			rbft.reProcessCacheEvents()
			rbft.logger.Debugf("Replica %d after reprocess cache events after checkpoint batch, seqNo=%d", rbft.chainConfig.SelfID, rbft.exec.lastExec)
		} else {
			// reqBatch call execute but have not done with execute
			rbft.logger.Errorf("Fail to call the checkpoint, lastExec=%d, report height=%d", rbft.exec.lastExec, state.MetaState.Height)
		}
	}
}

// =============================================================================
// gc: checkpoint issues
// =============================================================================

// checkpoint generate a checkpoint and broadcast it to outer.
func (rbft *rbftImpl[T, Constraint]) checkpoint(state *types.ServiceState, isConfig bool) {
	digest := state.MetaState.Digest
	seqNo := state.MetaState.Height

	rbft.logger.Infof("Replica %d sending checkpoint for view=%d/seqNo=%d and digest=%s",
		rbft.chainConfig.SelfID, rbft.chainConfig.View, seqNo, digest)

	signedCheckpoint, err := rbft.generateSignedCheckpoint(state, isConfig)
	if err != nil {
		rbft.logger.Errorf("Replica %d generate signed checkpoint error: %s", rbft.chainConfig.SelfID, err)
		rbft.stopNamespace()
		return
	}

	rbft.storeMgr.saveCheckpoint(seqNo, signedCheckpoint)
	rbft.persistCheckpoint(seqNo, []byte(digest))

	if isConfig {
		// use fetchCheckpointTimer to fetch the missing config checkpoint
		rbft.startFetchCheckpointTimer()
	} else {
		// if our lastExec is equal to high watermark, it means there is something wrong with checkpoint procedure, so that
		// we need to start a high-watermark timer for checkpoint, and trigger view-change when high-watermark timer expired
		if rbft.exec.lastExec == rbft.chainConfig.H+rbft.chainConfig.L && !rbft.chainConfig.isProposerElectionTypeWRF() {
			rbft.logger.Warningf("Replica %d try to send checkpoint equal to high watermark, "+
				"there may be something wrong with checkpoint", rbft.chainConfig.SelfID)
			rbft.softStartHighWatermarkTimer("replica send checkpoint equal to high-watermark")
		}
	}

	if rbft.chainConfig.isValidator() {
		payload, err := signedCheckpoint.MarshalVTStrict()
		if err != nil {
			rbft.logger.Errorf("ConsensusMessage_CHECKPOINT Marshal Error: %s", err)
			return
		}
		consensusMsg := &consensus.ConsensusMessage{
			Type:    consensus.Type_SIGNED_CHECKPOINT,
			Payload: payload,
		}
		rbft.peerMgr.broadcast(context.TODO(), consensusMsg)
	} else {
		rbft.logger.Debugf("Replica %d is not validator, not broadcast signedCheckpoint",
			rbft.chainConfig.SelfID)
	}

	rbft.logger.Trace(consensus.TagNameCheckpoint, consensus.TagStageStart, consensus.TagContentCheckpoint{
		Node:   rbft.chainConfig.SelfID,
		Height: seqNo,
		Config: isConfig,
	})

	rbft.recvCheckpoint(signedCheckpoint, true)
}

// recvCheckpoint processes logic after receive checkpoint.
func (rbft *rbftImpl[T, Constraint]) recvCheckpoint(signedCheckpoint *consensus.SignedCheckpoint, local bool) consensusEvent {
	// avoid repeated signature verification
	isCheckSignedCheckpoint := false

	if local && !rbft.chainConfig.isValidator() {
		rbft.logger.Debugf("Replica %d is not validator, not process self signedCheckpoint",
			rbft.chainConfig.SelfID)

		// Check whether qurom checkpoints have been received before
		for cp, signedChkpt := range rbft.storeMgr.checkpointStore {
			if cp.sequence == signedCheckpoint.Height() {
				isCheckSignedCheckpoint = true
				signedCheckpoint = signedChkpt
			}
		}
		if !isCheckSignedCheckpoint {
			return nil
		}
	}

	if signedCheckpoint.Checkpoint.Epoch < rbft.chainConfig.EpochInfo.Epoch {
		rbft.logger.Debugf("Replica %d received checkpoint from expired epoch %d, current epoch %d, ignore it.",
			rbft.chainConfig.SelfID, signedCheckpoint.Checkpoint.Epoch, rbft.chainConfig.EpochInfo.Epoch)
		return nil
	}

	checkpointHeight := signedCheckpoint.Checkpoint.Height()
	checkpointDigest := signedCheckpoint.Checkpoint.Digest()
	rbft.logger.Debugf("Replica %d received checkpoint from replica %d, seqNo %d, digest %s",
		rbft.chainConfig.SelfID, signedCheckpoint.GetAuthor(), checkpointHeight, checkpointDigest)

	// verify signature of remote checkpoint.
	if !local && !isCheckSignedCheckpoint {
		vErr := rbft.verifySignedCheckpoint(signedCheckpoint)
		if vErr != nil {
			rbft.logger.Errorf("Replica %d verify signature of checkpoint from %d error: %s",
				rbft.chainConfig.SelfID, signedCheckpoint.GetAuthor(), vErr)
			return nil
		}
	}

	rbft.logger.Trace(consensus.TagNameCheckpoint, consensus.TagStageReceive, consensus.TagContentCheckpoint{
		Node:   signedCheckpoint.GetAuthor(),
		Height: signedCheckpoint.GetCheckpoint().Height(),
		Config: signedCheckpoint.GetCheckpoint().NeedUpdateEpoch,
	})

	if rbft.weakCheckpointSetOutOfRange(signedCheckpoint) {
		if rbft.atomicIn(StateTransferring) {
			rbft.logger.Debugf("Replica %d keep trying state transfer", rbft.chainConfig.SelfID)
			return nil
		}
		// TODO(DH): do we need ?
		rbft.initRecovery()
		// try state transfer immediately when found lagging for the first time.
		rbft.logger.Debugf("Replica %d try state transfer after found high target", rbft.chainConfig.SelfID)
		rbft.tryStateTransfer()
		return nil
	}

	legal, matchingCheckpoints := rbft.compareCheckpointWithWeakSet(signedCheckpoint)
	if !legal {
		rbft.logger.Debugf("Replica %d ignore illegal checkpoint from replica %d, seqNo=%d",
			rbft.chainConfig.SelfID, signedCheckpoint.GetAuthor(), checkpointHeight)
		return nil
	}

	rbft.logger.Debugf("Replica %d found %d matching checkpoints for seqNo %d, digest %s",
		rbft.chainConfig.SelfID, len(matchingCheckpoints), checkpointHeight, checkpointDigest)

	if len(matchingCheckpoints) < rbft.commonCaseQuorum() {
		// We do not have a quorum yet
		return nil
	}

	// only update state target if we don't have the quorum checkpoint height which is not out of high
	// watermark range. After some time, we may catch to this height by ourselves.
	_, ok := rbft.storeMgr.localCheckpoints[checkpointHeight]
	if !ok {
		rbft.logger.Debugf("Replica %d found checkpoint quorum for seqNo %d, digest %s, but it has not "+
			"reached this checkpoint itself yet", rbft.chainConfig.SelfID, checkpointHeight, checkpointDigest)

		// update transferring target for state update, in order to trigger another state-update instance at the
		// moment the previous one has finished.
		target := &types.MetaState{Height: checkpointHeight, Digest: checkpointDigest}
		rbft.updateHighStateTarget(target, matchingCheckpoints) // for backwardness
		return nil
	}

	rbft.chainConfig.LastCheckpointExecBlockHash = checkpointDigest
	// the checkpoint is trigger by config batch
	if signedCheckpoint.Checkpoint.NeedUpdateEpoch {
		return rbft.finishConfigCheckpoint(checkpointHeight, checkpointDigest, matchingCheckpoints)
	}

	return rbft.finishNormalCheckpoint(checkpointHeight, checkpointDigest, matchingCheckpoints)
}

func (rbft *rbftImpl[T, Constraint]) finishConfigCheckpoint(checkpointHeight uint64, checkpointDigest string,
	matchingCheckpoints []*consensus.SignedCheckpoint) consensusEvent {
	// only process config checkpoint in ConfChange status.
	if !rbft.atomicIn(InConfChange) {
		rbft.logger.Warningf("Replica %d isn't in config-change when finishConfigCheckpoint", rbft.chainConfig.SelfID)
		return nil
	}

	// stop the fetch config checkpoint timer.
	rbft.stopFetchCheckpointTimer()

	rbft.logger.Infof("Replica %d found config checkpoint quorum for seqNo %d, digest %s",
		rbft.chainConfig.SelfID, checkpointHeight, checkpointDigest)

	// sync config checkpoint with ledger.
	rbft.syncConfigCheckpoint(checkpointHeight, matchingCheckpoints)

	// sync epoch with ledger.
	rbft.syncEpoch()

	// finish config change and restart consensus
	rbft.atomicOff(InConfChange)
	rbft.epochMgr.configBatchInOrder = 0
	rbft.metrics.statusGaugeInConfChange.Set(0)
	rbft.maybeSetNormal()
	finishMsg := fmt.Sprintf("======== Replica %d finished config change, "+
		"primary=%d, epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d",
		rbft.chainConfig.SelfID, rbft.chainConfig.PrimaryID, rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.N, rbft.chainConfig.F, rbft.chainConfig.View, rbft.chainConfig.H, rbft.exec.lastExec)
	rbft.external.SendFilterEvent(types.InformTypeFilterFinishConfigChange, finishMsg)
	rbft.logger.Trace(consensus.TagNameCheckpoint, consensus.TagStageFinish, consensus.TagContentCheckpoint{
		Node:   rbft.chainConfig.SelfID,
		Height: checkpointHeight,
		Config: true,
	})

	rbft.logger.Debugf("Replica %d sending view change again because of epoch change", rbft.chainConfig.SelfID)
	return rbft.initRecovery()
}

func (rbft *rbftImpl[T, Constraint]) finishNormalCheckpoint(checkpointHeight uint64, checkpointDigest string,
	matchingCheckpoints []*consensus.SignedCheckpoint) consensusEvent {
	rbft.stopFetchCheckpointTimer()
	rbft.stopHighWatermarkTimer()

	rbft.logger.Infof("Replica %d found normal checkpoint quorum for seqNo %d, digest %s",
		rbft.chainConfig.SelfID, checkpointHeight, checkpointDigest)

	rbft.moveWatermarks(checkpointHeight, false)

	if rbft.chainConfig.isProposerElectionTypeWRF() {
		localCheckpoint := rbft.storeMgr.localCheckpoints[checkpointHeight]

		newView := rbft.chainConfig.View + 1
		qvc := &consensus.QuorumViewChange{
			ViewChanges: make([]*consensus.ViewChange, 0, len(rbft.vcMgr.viewChangeStore)),
		}
		for _, ckp := range matchingCheckpoints {
			if ckp.Checkpoint.ViewChange != nil {
				if ckp.Checkpoint.ViewChange.Basis.View == newView {
					qvc.ViewChanges = append(qvc.ViewChanges, ckp.Checkpoint.ViewChange)
				}
			}
		}

		// update view after checkpoint
		// persist new view
		nv := &consensus.NewView{
			ReplicaId:     rbft.chainConfig.PrimaryID,
			View:          newView,
			ViewChangeSet: qvc,
			QuorumCheckpoint: &consensus.QuorumCheckpoint{
				Checkpoint: localCheckpoint.Checkpoint,
				Signatures: lo.SliceToMap(matchingCheckpoints, func(item *consensus.SignedCheckpoint) (uint64, []byte) {
					return item.Author, item.Signature
				}),
			},
			FromId:         rbft.chainConfig.SelfID,
			AutoTermUpdate: true,
		}
		sig, sErr := rbft.signNewView(nv)
		if sErr != nil {
			rbft.logger.Warningf("Replica %d sign new view failed: %s", rbft.chainConfig.SelfID, sErr)
			return nil
		}
		nv.Signature = sig
		rbft.persistNewView(nv)

		// Slave -> Primary： need update self seqNo(because only primary will update)
		rbft.batchMgr.setSeqNo(checkpointHeight)
		rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d, update to new view: %d, new primary ID: %d,  epoch:%d, last checkpoint digest: %s", rbft.chainConfig.SelfID, rbft.chainConfig.H, rbft.chainConfig.View, rbft.chainConfig.PrimaryID, rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.LastCheckpointExecBlockHash)
	} else {
		rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d", rbft.chainConfig.SelfID, rbft.chainConfig.H)
	}

	rbft.nullReqTimerReset()
	if !rbft.isPrimary(rbft.chainConfig.SelfID) || !rbft.isNormal() {
		if rbft.batchMgr.batchTimerActive {
			rbft.stopBatchTimer()
		}
	}
	if rbft.batchMgr.noTxBatchTimerActive {
		rbft.stopNoTxBatchTimer()
	}

	rbft.external.SendFilterEvent(types.InformTypeFilterStableCheckpoint, matchingCheckpoints)
	rbft.logger.Trace(consensus.TagNameCheckpoint, consensus.TagStageFinish, consensus.TagContentCheckpoint{
		Node:   rbft.chainConfig.SelfID,
		Height: checkpointHeight,
		Config: false,
	})

	if rbft.chainConfig.isProposerElectionTypeWRF() {
		rbft.processWRFHighViewMsgs()
	}

	// make sure node is in normal status before try to batch, as we may reach stable
	// checkpoint in vc.
	if rbft.isNormal() && rbft.isPrimary(rbft.chainConfig.SelfID) {
		// for primary, we can try to resubmit transactions after stable checkpoint as we
		// may block pre-prepare before because of high watermark limit.
		rbft.primaryResubmitTransactions()

		rbft.restartBatchTimer()

		// start noTx batch timer if there is no pending tx in pool
		rbft.restartNoTxBatchTimer()
	}
	return nil
}

// weakCheckpointSetOutOfRange checks if this node is fell behind or not. If we receive f+1
// checkpoints whose seqNo > H (for example 150), it is possible that we have fell behind, and
// will trigger recovery, but if we have already executed blocks larger than these checkpoints,
// we would like to start high-watermark timer in order to get the latest stable checkpoint.
func (rbft *rbftImpl[T, Constraint]) weakCheckpointSetOutOfRange(signedCheckpoint *consensus.SignedCheckpoint) bool {
	H := rbft.chainConfig.H + rbft.chainConfig.L
	checkpointHeight := signedCheckpoint.Checkpoint.Height()

	// Track the last observed checkpoint sequence number if it exceeds our high watermark,
	// keyed by replica to prevent unbounded growth
	if checkpointHeight < H {
		// For non-byzantine nodes, the checkpoint sequence number increases monotonically
		delete(rbft.storeMgr.higherCheckpoints, signedCheckpoint.GetAuthor())
	} else {
		// We do not track the highest one, as a byzantine node could pick an arbitrarily high sequence number
		// and even if it recovered to be non-byzantine, we would still believe it to be far ahead
		rbft.storeMgr.higherCheckpoints[signedCheckpoint.GetAuthor()] = signedCheckpoint
		rbft.logger.Debugf("Replica %d received a checkpoint out of range from replica %d, seq %d",
			rbft.chainConfig.SelfID, signedCheckpoint.GetAuthor(), checkpointHeight)

		// If f+1 other replicas have reported checkpoints that were (at one time) outside our watermarks
		// we need to check to see if we have fallen behind.
		if len(rbft.storeMgr.higherCheckpoints) >= rbft.oneCorrectQuorum() {
			highestWeakCertMeta := types.MetaState{}
			weakCertRecord := make(map[types.MetaState][]*consensus.SignedCheckpoint)
			for replicaID, remoteCheckpoint := range rbft.storeMgr.higherCheckpoints {
				if remoteCheckpoint.Checkpoint.Height() <= H {
					delete(rbft.storeMgr.higherCheckpoints, replicaID)
					continue
				}
				meta := types.MetaState{
					Height: remoteCheckpoint.Checkpoint.Height(),
					Digest: remoteCheckpoint.Checkpoint.Digest(),
				}
				if _, exist := weakCertRecord[meta]; !exist {
					weakCertRecord[meta] = []*consensus.SignedCheckpoint{remoteCheckpoint}
				} else {
					weakCertRecord[meta] = append(weakCertRecord[meta], remoteCheckpoint)
				}

				// found a weak cert, compare and cache the largest weak cert
				if len(weakCertRecord[meta]) > rbft.oneCorrectQuorum() && meta.Height > highestWeakCertMeta.Height {
					highestWeakCertMeta = meta
				}
			}

			// If there are f+1 nodes have issued same checkpoints above our high watermark, then current
			// node probably cannot record 2f+1 checkpoints for that sequence number, it is perhaps that
			// current node has been out of date
			highestWeakCert, ok := weakCertRecord[highestWeakCertMeta]
			if !ok {
				return false
			}
			rbft.logger.Debugf("Replica %d is out of date, f+1 nodes agree checkpoints "+
				"out of our high water mark, %d vs %d", rbft.chainConfig.SelfID, highestWeakCertMeta.Height, H)

			// we have executed to the target height, only need to start a high watermark timer.
			if rbft.exec.lastExec >= highestWeakCertMeta.Height {
				rbft.logger.Infof("Replica %d has already executed block %d larger than target's "+
					"seqNo %d", rbft.chainConfig.SelfID, rbft.exec.lastExec, highestWeakCertMeta.Height)
				rbft.softStartHighWatermarkTimer("replica received f+1 checkpoints out of range but " +
					"we have already executed")
				return false
			}

			// update state-update target here for an efficient initiation for a new state-update instance.
			rbft.updateHighStateTarget(&highestWeakCertMeta, highestWeakCert) // for backwardness

			return true
		}
	}

	return false
}

// moveWatermarks move low watermark h to n, and clear all message whose seqNo is smaller than h.
func (rbft *rbftImpl[T, Constraint]) moveWatermarks(n uint64, newEpoch bool) {
	h := n

	if rbft.chainConfig.H > n {
		rbft.logger.Criticalf("Replica %d moveWaterMarks but rbft.h(h=%d)>n(n=%d)", rbft.chainConfig.SelfID, rbft.chainConfig.H, n)
		return
	}

	for idx, cert := range rbft.storeMgr.certStore {
		if idx.n <= h {
			rbft.logger.Debugf("Replica %d cleaning quorum certificate for view=%d/seqNo=%d",
				rbft.chainConfig.SelfID, idx.v, idx.n)
			delete(rbft.storeMgr.certStore, idx)
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			delete(rbft.storeMgr.committedCert, idx)
			delete(rbft.storeMgr.seqMap, idx.n)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
			rbft.storeMgr.committedCertCache[idx] = cert
		}
	}
	rbft.storeMgr.cleanCommittedCertCache(h)
	rbft.metrics.outstandingBatchesGauge.Set(float64(len(rbft.storeMgr.outstandingReqBatches)))

	// retain most recent 10 block info in txBatchStore cache as non-primary
	// replicas may need to fetch those batches if they are lack of some txs
	// in those batches.
	var target uint64
	pos := n / rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod * rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod
	if pos <= rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod {
		target = 0
	} else {
		target = pos - rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod
	}
	// At least the last k batches are also kept when checkpoint is 1
	if n-target < rbft.config.CommittedBlockCacheNumber {
		if n > rbft.config.CommittedBlockCacheNumber {
			target = n - rbft.config.CommittedBlockCacheNumber
		} else {
			target = 0
		}
	}

	rbft.logger.Debugf("Replica %d clean batch, seqNo<=%d", rbft.chainConfig.SelfID, target)
	// clean batches every K interval
	var digestList []string
	for digest, batch := range rbft.storeMgr.batchStore {
		if batch.SeqNo <= target {
			rbft.logger.Debugf("Replica %d clean batch, seqNo=%d, digest=%s", rbft.chainConfig.SelfID, batch.SeqNo, digest)
			delete(rbft.storeMgr.batchStore, digest)
			rbft.persistDelBatch(digest)
		}
		if batch.SeqNo <= n {
			digestList = append(digestList, digest)
		}
	}
	rbft.metrics.batchesGauge.Set(float64(len(rbft.storeMgr.batchStore)))
	rbft.batchMgr.requestPool.RemoveBatches(digestList)

	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	for cID, signedCheckpoint := range rbft.storeMgr.checkpointStore {
		if cID.sequence <= h {
			rbft.logger.Debugf("Replica %d cleaning checkpoint message from replica %d, seqNo %d, digest %s",
				rbft.chainConfig.SelfID, cID.author, cID.sequence, signedCheckpoint.Checkpoint.Digest())
			delete(rbft.storeMgr.checkpointStore, cID)
		}
	}

	// save local checkpoint to help remote lagging nodes recover.
	for seqNo, signedCheckpoint := range rbft.storeMgr.localCheckpoints {
		if seqNo < h {
			rbft.logger.Debugf("Replica %d remove localCheckpoints, seqNo: %d",
				rbft.chainConfig.SelfID, seqNo)
			// remove local checkpoint from cache and database
			rbft.storeMgr.deleteCheckpoint(seqNo)
			rbft.persistDelCheckpoint(seqNo)
		} else {
			if newEpoch {
				rbft.logger.Debugf("Replica %d resign checkpoint, seqNo: %d",
					rbft.chainConfig.SelfID, seqNo)

				// NOTE! re-sign checkpoint in case cert has been replaced after turn into a higher epoch.
				newSig, sErr := rbft.signCheckpoint(signedCheckpoint.Checkpoint)
				if sErr != nil {
					rbft.logger.Errorf("Replica %d sign checkpoint error: %s", rbft.chainConfig.SelfID, sErr)
					rbft.stopNamespace()
					return
				}
				signedCheckpoint.Signature = newSig
			}
		}
	}
	rbft.logger.Debugf("Replica %d finished clean checkpoint, remain number: %d",
		rbft.chainConfig.SelfID, len(rbft.storeMgr.localCheckpoints))

	for idx := range rbft.vcMgr.qlist {
		if idx.n <= h {
			delete(rbft.vcMgr.qlist, idx)
		}
	}

	for seqNo := range rbft.vcMgr.plist {
		if seqNo <= h {
			delete(rbft.vcMgr.plist, seqNo)
		}
	}

	for digest, idx := range rbft.storeMgr.missingBatchesInFetching {
		if idx.n <= h {
			rbft.logger.Debugf("Clean old missingBatchesInFetching, batch=%s", digest)
			delete(rbft.storeMgr.missingBatchesInFetching, digest)
		}
	}

	rbft.hLock.RLock()
	rbft.chainConfig.H = h
	rbft.persistH(h)
	rbft.hLock.RUnlock()

	rbft.logger.Infof("Replica %d updated low water mark to %d", rbft.chainConfig.SelfID, rbft.chainConfig.H)
}

// updateHighStateTarget updates high state target
func (rbft *rbftImpl[T, Constraint]) updateHighStateTarget(target *types.MetaState, checkpointSet []*consensus.SignedCheckpoint, epochChanges ...*consensus.EpochChange) {
	if target == nil {
		rbft.logger.Warningf("Replica %d received a nil target", rbft.chainConfig.SelfID)
		return
	}

	if rbft.storeMgr.highStateTarget != nil && rbft.storeMgr.highStateTarget.metaState.Height >= target.Height {
		rbft.logger.Infof("Replica %d not updating state target to seqNo %d, has target for seqNo %d",
			rbft.chainConfig.SelfID, target.Height, rbft.storeMgr.highStateTarget.metaState.Height)
		return
	}

	if rbft.atomicIn(StateTransferring) {
		rbft.logger.Noticef("Replica %d has found high-target expired while transferring, "+
			"update target to %d", rbft.chainConfig.SelfID, target.Height)
	} else {
		rbft.logger.Noticef("Replica %d updating state target to seqNo %d digest %s", rbft.chainConfig.SelfID,
			target.Height, target.Digest)
	}

	rbft.storeMgr.highStateTarget = &stateUpdateTarget{
		metaState:     target,
		checkpointSet: checkpointSet,
		epochChanges:  epochChanges,
	}
}

// tryStateTransfer sets system abnormal and stateTransferring, then skips to target
func (rbft *rbftImpl[T, Constraint]) tryStateTransfer() {
	if !rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d is out of sync, pending tryStateTransfer", rbft.chainConfig.SelfID)
		rbft.on(SkipInProgress)
	}

	rbft.setAbNormal()

	if rbft.atomicIn(StateTransferring) {
		rbft.logger.Debugf("Replica %d is currently mid tryStateTransfer, it must wait for this "+
			"tryStateTransfer to complete before initiating a new one", rbft.chainConfig.SelfID)
		return
	}

	// if high state target is nil, we could not state update
	if rbft.storeMgr.highStateTarget == nil {
		rbft.logger.Debugf("Replica %d has no targets to attempt tryStateTransfer to, delaying", rbft.chainConfig.SelfID)
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
	rbft.epochMgr.configBatchInOrder = 0
	rbft.metrics.statusGaugeInConfChange.Set(0)

	// just stop high-watermark timer:
	// a primary who has started a high-watermark timer because of missing of checkpoint may find
	// quorum checkpoint with different digest and trigger state-update
	rbft.stopHighWatermarkTimer()

	rbft.atomicOn(StateTransferring)
	rbft.metrics.statusGaugeStateTransferring.Set(StateTransferring)

	// clean cert with seqNo <= target before stateUpdate to avoid influencing the
	// following progress
	for idx, cert := range rbft.storeMgr.certStore {
		if idx.n <= target.metaState.Height {
			rbft.logger.Debugf("Replica %d clean cert with seqNo %d <= target %d, "+
				"digest=%s, before state update", rbft.chainConfig.SelfID, idx.n, target.metaState.Height, idx.d)
			delete(rbft.storeMgr.certStore, idx)
			delete(rbft.storeMgr.outstandingReqBatches, idx.d)
			delete(rbft.storeMgr.committedCert, idx)
			delete(rbft.storeMgr.seqMap, idx.n)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
			rbft.storeMgr.committedCertCache[idx] = cert
		}
	}
	rbft.storeMgr.cleanCommittedCertCache(target.metaState.Height)
	rbft.metrics.outstandingBatchesGauge.Set(float64(len(rbft.storeMgr.outstandingReqBatches)))

	rbft.logger.Noticef("Replica %d try state update to %d", rbft.chainConfig.SelfID, target.metaState.Height)

	// attempts to synchronize state to a particular target, implicitly calls rollback if needed
	rbft.metrics.stateUpdateCounter.Add(float64(1))
	rbft.external.StateUpdate(rbft.chainConfig.H, target.metaState.Height, target.metaState.Digest, target.checkpointSet, target.epochChanges...)
}

// recvStateUpdatedEvent processes StateUpdatedMessage.
// functions:
// 1) succeed or not
// 2) update information about the latest stable checkpoint
// 3) update epoch info if it has been changed
//
// we need to check if the state update process is successful at first
// as for that state update target is the latest stable checkpoint,
// we need to move watermark and update our checkpoint storage for it
// at last if the epoch info has been changed, we also need to update
// self epoch-info and trigger another recovery process
func (rbft *rbftImpl[T, Constraint]) recvStateUpdatedEvent(ss *types.ServiceSyncState) consensusEvent {
	seqNo := ss.MetaState.Height
	digest := ss.MetaState.Digest

	// high state target nil warning
	if rbft.storeMgr.highStateTarget == nil {
		rbft.logger.Warningf("Replica %d has no state targets, cannot resume tryStateTransfer yet", rbft.chainConfig.SelfID)
	} else if seqNo < rbft.storeMgr.highStateTarget.metaState.Height {
		if !ss.EpochChanged {
			// If state transfer did not complete successfully, or if it did not reach the highest target, try again.
			rbft.logger.Warningf("Replica %d recovered to seqNo %d but our high-target has moved to %d, "+
				"keep on state transferring", rbft.chainConfig.SelfID, seqNo, rbft.storeMgr.highStateTarget.metaState.Height)
			rbft.atomicOff(StateTransferring)
			rbft.metrics.statusGaugeStateTransferring.Set(0)
			rbft.exec.setLastExec(seqNo)
			rbft.tryStateTransfer()
		} else {
			rbft.logger.Debugf("Replica %d state updated, lastExec = %d, seqNo = %d, accept epoch proof for %d", rbft.chainConfig.SelfID, rbft.exec.lastExec, seqNo, ss.Epoch)
			if ec, ok := rbft.epochMgr.epochProofCache[ss.Epoch]; ok {
				rbft.epochMgr.persistEpochQuorumCheckpoint(ec.GetCheckpoint())
			}
		}
		return nil
	} else if seqNo > rbft.storeMgr.highStateTarget.metaState.Height {
		rbft.logger.Errorf("Replica %d recovered to seqNo %d which is higher than high-target %d",
			rbft.chainConfig.SelfID, seqNo, rbft.storeMgr.highStateTarget.metaState.Height)
		rbft.stopNamespace()
		return nil
	}

	rbft.logger.Debugf("Replica %d state updated, lastExec = %d, seqNo = %d", rbft.chainConfig.SelfID, rbft.exec.lastExec, seqNo)
	if ss.EpochChanged {
		rbft.logger.Debugf("Replica %d accept epoch proof for %d", rbft.chainConfig.SelfID, ss.Epoch)
		if ec, ok := rbft.epochMgr.epochProofCache[ss.Epoch]; ok {
			rbft.epochMgr.persistEpochQuorumCheckpoint(ec.GetCheckpoint())
		}
	}

	// 1. clear useless txs in txpool after state updated and saves txs which are not committed to ensure
	// all received txs will be committed eventually.
	// var saveBatches []string
	// for batchDigest, batch := range rbft.storeMgr.batchStore {
	//	if batch.SeqNo > seqNo {
	//		saveBatches = append(saveBatches, batchDigest)
	//	}
	// }
	// rbft.batchMgr.requestPool.Reset(saveBatches)

	// 2. reset commit state after cut down block.
	if seqNo < rbft.exec.lastExec {
		rbft.logger.Debugf("Replica %d reset commit state after cut down blocks", rbft.chainConfig.SelfID)
		// rebuild committedCert cache
		rbft.storeMgr.committedCert = make(map[msgID]string)

		// after cut down block, some committed blocks may be committed again.
		for idx, cert := range rbft.storeMgr.certStore {
			if idx.n > seqNo {
				cert.sentExecute = false
				rbft.logger.Debugf("reset cert %v", idx)
				if idx.v == rbft.chainConfig.View && rbft.committed(idx.v, idx.n, idx.d) && cert.sentCommit && !cert.sentExecute {
					rbft.storeMgr.committedCert[idx] = idx.d
				}
			}
		}
	}

	// 3. finished state update
	finishMsg := fmt.Sprintf("======== Replica %d finished stateUpdate, height: %d", rbft.chainConfig.SelfID, seqNo)
	rbft.logger.Noticef(finishMsg)
	rbft.external.SendFilterEvent(types.InformTypeFilterFinishStateUpdate, finishMsg)
	rbft.exec.setLastExec(seqNo)
	rbft.batchMgr.setSeqNo(seqNo)
	rbft.storeMgr.missingBatchesInFetching = make(map[string]msgID)
	rbft.off(SkipInProgress)
	rbft.atomicOff(StateTransferring)
	rbft.metrics.statusGaugeStateTransferring.Set(0)
	rbft.maybeSetNormal()

	// 4. process epoch-info
	epochChanged := ss.Epoch != rbft.chainConfig.EpochInfo.Epoch
	if epochChanged {
		rbft.logger.Infof("epoch changed from %d to %d", rbft.chainConfig.EpochInfo.Epoch, ss.Epoch)
		rbft.turnIntoEpoch()
		rbft.logger.Noticef("======== Replica %d updated epoch, epoch=%d.", rbft.chainConfig.SelfID, rbft.chainConfig.EpochInfo.Epoch)
		rbft.atomicOff(inEpochSyncing)
	}

	// 5. sign and cache local checkpoint.
	// NOTE! generate checkpoint and move watermark when epochChanged or reach checkpoint height.
	if epochChanged || seqNo%rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod == 0 {
		checkpointSet := rbft.storeMgr.highStateTarget.checkpointSet
		if len(checkpointSet) == 0 {
			rbft.logger.Warningf("Replica %d found an empty checkpoint set", rbft.chainConfig.SelfID)
			rbft.stopNamespace()
			return nil
		}

		// NOTE! don't generate local checkpoint using current epoch, use remote consistent checkpoint
		// to generate a signed checkpoint as this consistent checkpoint may be generated in an old epoch.
		checkpoint := checkpointSet[0].Checkpoint
		signature, sErr := rbft.signCheckpoint(checkpoint)
		if sErr != nil {
			rbft.logger.Errorf("Replica %d generate signed checkpoint error: %s", rbft.chainConfig.SelfID, sErr)
			rbft.stopNamespace()
			return nil
		}
		signedCheckpoint := &consensus.SignedCheckpoint{
			Author:     rbft.chainConfig.SelfID,
			Checkpoint: checkpoint,
			Signature:  signature,
		}
		rbft.storeMgr.saveCheckpoint(seqNo, signedCheckpoint)
		rbft.chainConfig.LastCheckpointExecBlockHash = digest
		rbft.persistCheckpoint(seqNo, []byte(digest))
		rbft.moveWatermarks(seqNo, epochChanged)
	}

	// 6. process recovery.
	if epochChanged {
		rbft.logger.Debugf("Replica %d sending view change after sync chain because of epoch change", rbft.chainConfig.SelfID)
		// trigger another round of recovery after epoch change to find correct view-number
		return rbft.initRecovery()
	}

	if rbft.atomicIn(InViewChange) {
		if rbft.chainConfig.SelfID == rbft.chainConfig.calPrimaryIDByView(rbft.chainConfig.View) {
			// view may not be changed after state-update, current node mistakes itself for primary
			// so send view change to step into the new view
			rbft.logger.Debugf("Primary %d send view-change after state update", rbft.chainConfig.SelfID)
			return rbft.sendViewChange()
		}

		// check if we have new view for current view, if so(vc then sync chain), directly finish view change.
		// if not(sync chain then vc), trigger recovery to find correct view-number
		nv, ok := rbft.vcMgr.newViewStore[rbft.chainConfig.View]
		if ok {
			rbft.persistNewView(nv)
			rbft.logger.Infof("Replica %d persist view=%d after sync chain", rbft.chainConfig.SelfID, rbft.chainConfig.View)
			return &LocalEvent{
				Service:   ViewChangeService,
				EventType: ViewChangeDoneEvent,
			}
		}
		rbft.logger.Debugf("Replica %d sending view change after sync chain because of miss new view", rbft.chainConfig.SelfID)
		// trigger another round of recovery after sync chain to find correct view-number
		return rbft.initRecovery()
	}

	// here, we always fetch PQC after finish state update as we only recovery to the largest checkpoint which
	// is lower or equal to the lastExec quorum of others.
	return rbft.fetchRecoveryPQC()
}

func (rbft *rbftImpl[T, Constraint]) recvReBroadcastRequestSet(e *consensus.ReBroadcastRequestSet) consensusEvent {
	rbft.logger.Debugf("Replica %d recv ReBroadcastRequestSet from remote node: %d", rbft.chainConfig.SelfID, e.GetReplicaId())
	start := time.Now()
	// handle reBroadcast requestSet
	rbft.metrics.incomingRemoteTxSets.Add(float64(1))
	rbft.metrics.incomingRemoteTxs.Add(float64(len(e.Requests)))

	var requestSet RequestSet[T, Constraint]
	if err := requestSet.FromPB(e); err != nil {
		rbft.logger.Errorf("RequestSet unmarshal error: %v", err)
		return nil
	}
	rbft.processReqSetEvent(&requestSet)
	rbft.metrics.processEventDuration.With("event", "rebroadcast_request_set").Observe(time.Since(start).Seconds())
	return nil
}
