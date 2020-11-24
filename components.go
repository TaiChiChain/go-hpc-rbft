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
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"
)

// constant timer names
const (
	requestTimer          = "requestTimer"
	batchTimer            = "batchTimer"            // timer for primary triggering package a batch to send pre-prepare
	vcResendTimer         = "vcResendTimer"         // timer triggering resend of a view change
	newViewTimer          = "newViewTimer"          // timer triggering view change by out of timeout of some requestBatch
	nullRequestTimer      = "nullRequestTimer"      // timer triggering send null request, used for heartbeat
	firstRequestTimer     = "firstRequestTimer"     // timer set for replicas in case of primary start and shut down immediately
	syncStateRspTimer     = "syncStateRspTimer"     // timer track timeout for quorum sync-state responses
	syncStateRestartTimer = "syncStateRestartTimer" // timer track timeout for restart sync-state
	recoveryRestartTimer  = "recoveryRestartTimer"  // timer track how long a recovery is finished and fires if needed
	cleanViewChangeTimer  = "cleanViewChangeTimer"  // timer track how long a viewchange msg will store in memory
	checkPoolTimer        = "checkPoolTimer"        // timer track timeout for check pool interval
	fetchCheckpointTimer  = "fetchCheckpointTimer"  // timer for nodes to trigger fetch checkpoint when we are processing config transaction
	highWatermarkTimer    = "highWatermarkTimer"    // timer for nodes to find the problem of missing too much checkpoint
)

// constant default
const (
	// default timer timeout value
	DefaultRequestTimeout          = 6 * time.Second
	DefaultBatchTimeout            = 500 * time.Millisecond
	DefaultVcResendTimeout         = 10 * time.Second
	DefaultNewViewTimeout          = 8 * time.Second
	DefaultNullRequestTimeout      = 9 * time.Second
	DefaultFirstRequestTimeout     = 30 * time.Second
	DefaultSyncStateRspTimeout     = 1 * time.Second
	DefaultSyncStateRestartTimeout = 10 * time.Second
	DefaultRecoveryRestartTimeout  = 10 * time.Second
	DefaultCleanViewChangeTimeout  = 60 * time.Second
	DefaultCheckPoolTimeout        = 3 * time.Minute
	DefaultFetchCheckpointTimeout  = 5 * time.Second

	// default k value
	DefaultK = 10
)

// event type
const (
	// 1.rbft core
	CoreBatchTimerEvent = iota
	CoreNullRequestTimerEvent
	CoreFirstRequestTimerEvent
	CoreCheckPoolTimerEvent
	CoreStateUpdatedEvent
	CoreResendMissingTxsEvent
	CoreResendFetchMissingEvent
	CoreHighWatermarkEvent

	// 2.view change
	ViewChangeTimerEvent
	ViewChangeResendTimerEvent
	ViewChangeQuorumEvent
	ViewChangedEvent

	// 3.recovery
	RecoveryInitEvent
	RecoveryRestartTimerEvent
	RecoveryDoneEvent
	RecoverySyncStateRspTimerEvent
	RecoverySyncStateRestartTimerEvent
	NotificationQuorumEvent

	// 4.epoch mgr service
	FetchCheckpointEvent
)

// service type
const (
	CoreRbftService = iota
	ViewChangeService
	RecoveryService
	EpochMgrService
	NotSupportService
)

const (
	// DefaultRequestSetMaxMem is the default memory size of request set
	DefaultRequestSetMaxMem = 45 * 1024 * 1024 // 45MB
)

// LocalEvent represents event sent by local modules
type LocalEvent struct {
	Service   int // service type range from {CoreRbftService, ViewChangeService, RecoveryService, NodeMgrService}
	EventType int
	Event     interface{}
}

// Event is a type meant to clearly convey that the return type or parameter to a function will be supplied to/from an events.Manager
type consensusEvent interface{}

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
	prePrepare  *pb.PrePrepare      // pre-prepare msg
	sentPrepare bool                // track whether broadcast prepare for this batch before or not
	prepare     map[pb.Prepare]bool // prepare msgs received from other nodes
	sentCommit  bool                // track whether broadcast commit for this batch before or not
	commit      map[pb.Commit]bool  // commit msgs received from other nodes
	sentExecute bool                // track whether sent execute event to executor module before or not
}

// -----------recovery related structs-----------------
type ntfIdx struct {
	v      uint64
	nodeID uint64
}

type ntfIde struct {
	e      uint64
	nodeID uint64
}

// -----------router struct-----------------
type routerMap struct {
	//IDMap   map[uint64]string
	HashMap map[string]uint64
}

// nodeState records every node's consensus status(n, view, routerMap) and
// ledger status(block number, hash and seqNo)
type nodeState struct {
	n       uint64
	epoch   uint64
	view    uint64
	applied uint64
	digest  string
}

// wholeStates maps node ID to nodeState
type wholeStates map[*pb.NodeInfo]nodeState

// -----------viewchange related structs-----------------
// viewchange index
type vcIdx struct {
	v  uint64 // view
	id uint64 // replica id
}

type xset map[uint64]string

type nextDemandNewView uint64
