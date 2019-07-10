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
This file defines the structs uesd in RBFT
*/

import (
	pb "github.com/ultramesh/flato-rbft/rbftpb"
)

// record consensus version in database
const (
	currentVersion = "1.8"
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
	updateTimer           = "updateTimer"           // timer track how long a add/delete node process will take
	checkPoolTimer        = "checkPoolTimer"        // timer track timeout for check pool interval
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
	CoreRetrieveStatusEvent

	// 2.view change
	ViewChangeTimerEvent
	ViewChangeResendTimerEvent
	ViewChangeQuorumEvent
	ViewChangedEvent

	// 3.recovery
	RecoveryRestartTimerEvent
	RecoveryDoneEvent
	RecoverySyncStateRspTimerEvent
	RecoverySyncStateRestartTimerEvent
	NotificationQuorumEvent

	// 4.node mgr service
	NodeMgrDelNodeEvent
	NodeMgrAgreeUpdateQuorumEvent
	NodeMgrUpdatedEvent
	NodeMgrUpdateTimerEvent
)

// service type
const (
	CoreRbftService = iota
	ViewChangeService
	RecoveryService
	NodeMgrService
	NotSupportService
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
	v        uint64
	nodeHash string
}

// nodeState records every node's consensus status(n, view, routers) and
// ledger status(block number, hash and genesis seqNo)
type nodeState struct {
	n            uint64
	view         uint64
	routerInfo   string
	appliedIndex uint64
	digest       string
	genesis      uint64
}

// nodeStateWithoutGenesis tracks every node's consensus status and ledger
// status without genesis to help compare whole status among nodes.
type nodeStateWithoutGenesis struct {
	n               uint64
	view            uint64
	routersAndMaxID string
	appliedIndex    uint64
	digest          string
}

// wholeStates maps node ID to nodeState
type wholeStates map[uint64]nodeState

// -----------viewchange related structs-----------------
// viewchange index
type vcIdx struct {
	v  uint64 // view
	id uint64 // replica id
}

type xset map[uint64]string

type nextDemandNewView uint64

// -----------node add/deletion related structs-----------------

// agreeUpdateStore index
type aidx struct {
	v    uint64
	n    int64
	flag bool   // add or delete
	id   uint64 // replica id
}

// track the new view after update
type uidx struct {
	v    uint64
	n    int64
	flag bool   // add or delete
	hash string // target node's hash
}

// -----------state update related structs-----------------
type targetMessage struct {
	height uint64
	digest string
}

type replicaInfo struct {
	replicaID uint64
}

type stateUpdateTarget struct {
	targetMessage
	replicas []replicaInfo
}
