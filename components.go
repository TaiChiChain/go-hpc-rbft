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
	"time"

	"github.com/hyperchain/go-hpc-rbft/common/consensus"
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
)

// constant default
const (
	// default timer timeout value
	DefaultRequestTimeout          = 6 * time.Second
	DefaultBatchTimeout            = 500 * time.Millisecond
	DefaultVcResendTimeout         = 10 * time.Second
	DefaultNewViewTimeout          = 8 * time.Second
	DefaultNullRequestTimeout      = 9 * time.Second
	DefaultSyncStateRspTimeout     = 1 * time.Second
	DefaultSyncStateRestartTimeout = 10 * time.Second
	DefaultCleanViewChangeTimeout  = 60 * time.Second
	DefaultCheckPoolTimeout        = 3 * time.Minute
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
	CoreHighWatermarkEvent

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

// LocalEvent represents event sent by local modules
type LocalEvent struct {
	Service   int // service type range from {CoreRbftService, ViewChangeService, RecoveryService, NodeMgrService}
	EventType int
	Event     interface{}
}

// consensusEvent is a type meant to clearly convey that the return type or parameter to a function will be supplied to/from an events.Manager
type consensusEvent interface{}

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

// cached consensus msgs related to batch
type msgCert struct {
	prePrepare      *consensus.PrePrepare      // pre-prepare msg
	prePreparedTime int64                      // pre-prepared time
	prePrepareCtx   context.Context            // prePrepareCtx can be used to continue tracing from prePrepare.
	sentPrepare     bool                       // track whether broadcast prepare for this batch before or not
	prepare         map[consensus.Prepare]bool // prepare msgs received from other nodes
	preparedTime    int64                      // prepared time
	sentCommit      bool                       // track whether broadcast commit for this batch before or not
	commit          map[consensus.Commit]bool  // commit msgs received from other nodes
	committedTime   int64                      // committed time
	sentExecute     bool                       // track whether sent execute event to executor module before or not
	isConfig        bool                       // track whether current batch is a config batch
}

// ----------checkpoint related structs------------------
type chkptID struct {
	author   string
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
