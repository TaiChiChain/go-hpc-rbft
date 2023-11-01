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

/**
This file defines the structs used in RBFT
*/

import (
	"context"
	"fmt"
	"time"

	"github.com/axiomesh/axiom-bft/txpool"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

// constant timer names
const (
	requestTimer          = "requestTimer"
	batchTimer            = "batchTimer"            // timer for primary triggering package a batch to send pre-prepare
	vcResendTimer         = "vcResendTimer"         // timer triggering resend of a view change
	newViewTimer          = "newViewTimer"          // timer triggering view change by out of timeout of some requestBatch
	nullRequestTimer      = "nullRequestTimer"      // timer triggering send null request, used for heartbeat
	syncStateRspTimer     = "syncStateRspTimer"     // timer track timeout for quorum sync-state responses
	syncStateRestartTimer = "syncStateRestartTimer" // timer track timeout for restart sync-state
	cleanViewChangeTimer  = "cleanViewChangeTimer"  // timer track how long a viewchange msg will store in memory
	checkPoolTimer        = "checkPoolTimer"        // timer track timeout for check pool interval
	fetchCheckpointTimer  = "fetchCheckpointTimer"  // timer for nodes to trigger fetch checkpoint when we are processing config transaction
	highWatermarkTimer    = "highWatermarkTimer"    // timer for nodes to find the problem of missing too much checkpoint
	fetchViewTimer        = "fetchViewTimer"        // timer for nodes to fetch view periodically
	checkPoolRemoveTimer  = "checkPoolRemoveTimer"  // timer track timeout for check pool which need remove txs
	noTxBatchTimer        = "noTxBatchTimer"        // timer for primary triggering package a batch which no transaction to send pre-prepare
)

// constant default
const (
	// default timer timeout value
	DefaultRequestTimeout          = 6 * time.Second
	DefaultBatchTimeout            = 500 * time.Millisecond
	DefaultNoTxBatchTimeout        = 2 * time.Second
	DefaultVcResendTimeout         = 10 * time.Second
	DefaultNewViewTimeout          = 8 * time.Second
	DefaultNullRequestTimeout      = 9 * time.Second
	DefaultSyncStateRspTimeout     = 1 * time.Second
	DefaultSyncStateRestartTimeout = 10 * time.Second
	DefaultCleanViewChangeTimeout  = 60 * time.Second
	DefaultCheckPoolTimeout        = 3 * time.Minute
	DefaultCheckPoolRemoveTimeout  = 15 * time.Minute
	DefaultFetchCheckpointTimeout  = 5 * time.Second
	DefaultFetchViewTimeout        = 1 * time.Second

	// default k value
	DefaultK = 10
)

// event type
const (
	// 1.rbft core
	CoreBatchTimerEvent = iota
	CoreNullRequestTimerEvent
	CoreCheckPoolTimerEvent
	CoreStateUpdatedEvent
	CoreCheckpointBlockExecutedEvent
	CoreFindNextPrepareBatchsEvent
	CoreHighWatermarkEvent
	CoreCheckPoolRemoveTimerEvent
	CoreNoTxBatchTimerEvent

	// 2.view change
	ViewChangeTimerEvent
	ViewChangeResendTimerEvent
	ViewChangeQuorumEvent
	ViewChangeDoneEvent
	FetchViewEvent

	// 3.recovery
	RecoveryInitEvent
	RecoverySyncStateRspTimerEvent
	RecoverySyncStateRestartTimerEvent

	// 4.epoch mgr service
	FetchCheckpointEvent
	EpochSyncEvent
)

// service type
const (
	CoreRbftService = iota
	ViewChangeService
	RecoveryService
	EpochMgrService
	NotSupportService
)

var cannotProcessEventWhenWaitCheckpoint = -1

var canProcessEventsWhenWaitCheckpoint = map[int]struct{}{
	// CoreRbftService
	CoreCheckpointBlockExecutedEvent: {},
	CoreCheckPoolTimerEvent:          {},
	CoreCheckPoolRemoveTimerEvent:    {},
}

var canProcessMsgsWhenWaitCheckpoint = map[consensus.Type]struct{}{
	// CoreRbftService
	consensus.Type_FETCH_MISSING_REQUEST: {},

	// RecoveryService
	consensus.Type_SYNC_STATE:          {},
	consensus.Type_SYNC_STATE_RESPONSE: {},
	consensus.Type_FETCH_PQC_REQUEST:   {},
	consensus.Type_FETCH_PQC_RESPONSE:  {},

	// EpochMgrService
	consensus.Type_FETCH_CHECKPOINT:     {},
	consensus.Type_EPOCH_CHANGE_REQUEST: {},
	consensus.Type_EPOCH_CHANGE_PROOF:   {},
}

//consensus.Type_NULL_REQUEST:           {},
//consensus.Type_PRE_PREPARE:            {},
//consensus.Type_PREPARE:                {},
//consensus.Type_COMMIT:                 {},
//consensus.Type_REQUEST_SET:            {},
//consensus.Type_SIGNED_CHECKPOINT:      {},
//consensus.Type_FETCH_CHECKPOINT:       {},
//consensus.Type_VIEW_CHANGE:            {},
//consensus.Type_QUORUM_VIEW_CHANGE:     {},
//consensus.Type_NEW_VIEW:               {},
//consensus.Type_FETCH_VIEW:             {},
//consensus.Type_RECOVERY_RESPONSE:      {},
//consensus.Type_FETCH_BATCH_REQUEST:    {},
//consensus.Type_FETCH_BATCH_RESPONSE:   {},
//consensus.Type_FETCH_PQC_REQUEST:      {},
//consensus.Type_FETCH_PQC_RESPONSE:     {},
//consensus.Type_FETCH_MISSING_REQUEST:  {},
//consensus.Type_FETCH_MISSING_RESPONSE: {},
//consensus.Type_SYNC_STATE:             {},
//consensus.Type_SYNC_STATE_RESPONSE:    {},
//consensus.Type_EPOCH_CHANGE_REQUEST:   {},
//consensus.Type_EPOCH_CHANGE_PROOF:     {},

var validatorAcceptMsgsFromNonValidator = map[consensus.Type]struct{}{
	consensus.Type_REBROADCAST_REQUEST_SET: {},
	consensus.Type_VIEW_CHANGE:             {},
	consensus.Type_FETCH_CHECKPOINT:        {},
	consensus.Type_FETCH_VIEW:              {},
	consensus.Type_FETCH_BATCH_REQUEST:     {},
	consensus.Type_FETCH_PQC_REQUEST:       {},
	consensus.Type_FETCH_MISSING_REQUEST:   {},
	consensus.Type_SYNC_STATE:              {},
	consensus.Type_EPOCH_CHANGE_REQUEST:    {},
}

type RequestSet[T any, Constraint consensus.TXConstraint[T]] struct {
	Requests []*T
	Local    bool
}

func (s *RequestSet[T, Constraint]) ToPB(localId uint64) (*consensus.ReBroadcastRequestSet, error) {
	rawTxs, err := consensus.EncodeTxs[T, Constraint](s.Requests)
	if err != nil {
		return nil, err
	}
	return &consensus.ReBroadcastRequestSet{
		Requests:  rawTxs,
		ReplicaId: localId,
	}, nil
}

func (s *RequestSet[T, Constraint]) FromPB(pb *consensus.ReBroadcastRequestSet) error {
	txs, err := consensus.DecodeTxs[T, Constraint](pb.Requests)
	if err != nil {
		return err
	}
	s.Requests = txs
	s.Local = false
	return nil
}

func (s *RequestSet[T, Constraint]) Marshal(localId uint64) ([]byte, error) {
	pbData, err := s.ToPB(localId)
	if err != nil {
		return nil, err
	}
	return pbData.MarshalVTStrict()
}

func (s *RequestSet[T, Constraint]) Unmarshal(raw []byte) error {
	pbData := &consensus.ReBroadcastRequestSet{}
	if err := pbData.UnmarshalVT(raw); err != nil {
		return err
	}
	return s.FromPB(pbData)
}

type RequestBatch[T any, Constraint consensus.TXConstraint[T]] struct {
	RequestHashList []string
	RequestList     []*T
	Timestamp       int64
	SeqNo           uint64
	LocalList       []bool
	BatchHash       string
	Proposer        uint64
}

func (b *RequestBatch[T, Constraint]) ToPB() (*consensus.RequestBatch, error) {
	rawTxs, err := consensus.EncodeTxs[T, Constraint](b.RequestList)
	if err != nil {
		return nil, err
	}
	return &consensus.RequestBatch{
		RequestHashList: b.RequestHashList,
		RequestList:     rawTxs,
		Timestamp:       b.Timestamp,
		SeqNo:           b.SeqNo,
		LocalList:       b.LocalList,
		BatchHash:       b.BatchHash,
		Proposer:        b.Proposer,
	}, nil
}

func (b *RequestBatch[T, Constraint]) FromPB(pb *consensus.RequestBatch) error {
	txs, err := consensus.DecodeTxs[T, Constraint](pb.RequestList)
	if err != nil {
		return err
	}
	b.RequestHashList = pb.RequestHashList
	b.RequestList = txs
	b.Timestamp = pb.Timestamp
	b.SeqNo = pb.SeqNo
	b.LocalList = pb.LocalList
	b.BatchHash = pb.BatchHash
	b.Proposer = pb.Proposer
	return nil
}

func (b *RequestBatch[T, Constraint]) Marshal() ([]byte, error) {
	pbData, err := b.ToPB()
	if err != nil {
		return nil, err
	}
	return pbData.MarshalVTStrict()
}

func (b *RequestBatch[T, Constraint]) Unmarshal(raw []byte) error {
	pbData := &consensus.RequestBatch{}
	if err := pbData.UnmarshalVT(raw); err != nil {
		return err
	}
	return b.FromPB(pbData)
}

// LocalEvent represents event sent by local modules
type LocalEvent struct {
	Service   int // service type range from {CoreRbftService, ViewChangeService, RecoveryService, peerMgrService}
	EventType int
	Event     any
}

// consensusEvent is a type meant to clearly convey that the return type or parameter to a function will be supplied to/from an events.Manager
type consensusEvent any

// consensusMessageWrapper is used to wrap the *consensus.ConsensusMessage type, providing an additional context field
type consensusMessageWrapper struct {
	ctx context.Context
	*consensus.ConsensusMessage
}

// -----------certStore related structs-----------------
// Preprepare index
type qidx struct {
	d string // digest
	n uint64 // seqNo
}

// certStore index
type msgID struct {
	v uint64 // view
	n uint64 // seqNo
	d string // digest
}

func (x *msgID) ID() string {
	return fmt.Sprintf("view=%d/seq=%d/digest=%s", x.v, x.n, x.d)
}

// cached consensus msgs related to batch
type msgCert struct {
	prePrepare      *consensus.PrePrepare         // pre-prepare msg
	prePreparedTime int64                         // pre-prepared time
	prePrepareCtx   context.Context               // prePrepareCtx can be used to continue tracing from prePrepare.
	sentPrepare     bool                          // track whether broadcast prepare for this batch before or not
	prepare         map[string]*consensus.Prepare // prepare msgs received from other nodes
	preparedTime    int64                         // prepared time
	sentCommit      bool                          // track whether broadcast commit for this batch before or not
	commit          map[string]*consensus.Commit  // commit msgs received from other nodes
	committedTime   int64                         // committed time
	sentExecute     bool                          // track whether sent execute event to executor module before or not
	isConfig        bool                          // track whether current batch is a config batch
}

// ----------checkpoint related structs------------------
type chkptID struct {
	author   uint64
	sequence uint64
}

// nodeState records every node's consensus status(view) and
// ledger status(chain height, digest)
type nodeState struct {
	view   uint64
	height uint64
	digest string
}

// wholeStates maps checkpoint to nodeState
type wholeStates map[*consensus.SignedCheckpoint]nodeState

// -----------viewchange related structs-----------------
// viewchange index
type vcIdx struct {
	v  uint64 // view
	id uint64 // replica id
}

type nextDemandNewView uint64

const (
	ReqTxEvent = iota
	ReqNonceEvent
	ReqPendingTxCountEvent
	ReqGetWatermarkEvent
	ReqGetPoolMetaEvent
	ReqGetAccountMetaEvent
	ReqRemoveTxsEvent
)

// MiscEvent represents misc event sent by local modules
type MiscEvent struct {
	EventType int
	Event     any
}

type ReqTxMsg[T any, Constraint consensus.TXConstraint[T]] struct {
	hash string
	ch   chan *T
}

type ReqNonceMsg struct {
	account string
	ch      chan uint64
}

type ReqPendingTxCountMsg struct {
	ch chan uint64
}

type ReqGetWatermarkMsg struct {
	ch chan uint64
}

type ReqGetAccountPoolMetaMsg[T any, Constraint consensus.TXConstraint[T]] struct {
	account string
	full    bool
	ch      chan *txpool.AccountMeta[T, Constraint]
}

type ReqGetPoolMetaMsg[T any, Constraint consensus.TXConstraint[T]] struct {
	full bool
	ch   chan *txpool.Meta[T, Constraint]
}

type ReqRemoveTxsMsg[T any, Constraint consensus.TXConstraint[T]] struct {
	removeTxHashList []string
}
