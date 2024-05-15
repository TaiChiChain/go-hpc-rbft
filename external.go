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

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
	kittypes "github.com/axiomesh/axiom-kit/types"
)

// Storage is an interface that should be implemented by the application using non-volatile
// DB to store and restore consensus log.
type Storage interface {
	// StoreState stores key-value to non-volatile memory.
	StoreState(key string, value []byte) error

	// DelState deletes data with a specified key from non-volatile memory.
	DelState(key string) error

	// ReadState retrieves data with a specified key from non-volatile memory.
	ReadState(key string) ([]byte, error)

	// ReadStateSet retrieves data with specified key prefix from non-volatile memory.
	ReadStateSet(key string) (map[string][]byte, error)
}

// Network is used to send p2p messages between nodes.
type Network interface {
	// Broadcast delivers messages to all other nodes.
	Broadcast(ctx context.Context, msg *consensus.ConsensusMessage) error

	// Unicast delivers messages to given node with specified id.
	Unicast(ctx context.Context, msg *consensus.ConsensusMessage, to string) error
}

// Crypto is used to access the sign/verify methods from the crypto package
type Crypto interface {
	// Sign signs messages, returns error if any.
	Sign(msg []byte) ([]byte, error)

	// Verify verifies signature signed with msg from given peerHash, return nil if verify successfully
	Verify(nodeID uint64, signature []byte, msg []byte) error
}

// ServiceOutbound is the application service invoked by RBFT library which includes two core events:
//  1. Execute is invoked when RBFT core has achieved consensus on txs with batch number seqNo,
//     which will be submitted to application service. After application submitted the given batch,
//     application should call ServiceInbound.ReportExecuted to inform RBFT library the latest
//     service state.
//  2. StateUpdate is invoked when RBFT core finds current node out-of-date from other nodes by too many
//     seqNos, it's applications responsibility to implement a fast sync algorithm to ensure node
//     can catch up as soon as possible. Applications should call ServiceInbound.ReportStateUpdated
//     to inform RBFT library the latest service state after StateUpdate.
type ServiceOutbound[T any, Constraint kittypes.TXConstraint[T]] interface {
	// Execute informs application layer to apply one batch with given request list and batch seqNo.
	// Users can apply different batches asynchronously but ensure the order by seqNo.
	Execute(txs []*T, localList []bool, seqNo uint64, timestamp int64, proposerNodeID uint64)

	// StateUpdate informs application layer to catch up to given seqNo with specified state digest.
	// epochChanges should be provided when the sync request has a backwardness of epoch changes
	StateUpdate(localLowWatermark, seqNo uint64, digest string, checkpoints []*consensus.SignedCheckpoint, epochChanges ...*consensus.EpochChange)

	// SendFilterEvent posts some impotent events to application layer.
	// Users can decide to post filer event synchronously or asynchronously.
	SendFilterEvent(informType types.InformType, message ...any)
}

// EpochService provides service for epoch management.
type EpochService interface {
	GetCurrentEpochInfo() (*kittypes.EpochInfo, error)
	GetEpochInfo(epoch uint64) (*kittypes.EpochInfo, error)
	StoreEpochState(key string, value []byte) error
	ReadEpochState(key string) ([]byte, error)
}

// NodeService provides service for node management.
type NodeService interface {
	GetNodeInfo(nodeID uint64) (*NodeInfo, error)
	GetNodeIDByP2PID(p2pID string) (uint64, error)
	GetValidatorSet() (map[uint64]int64, error)
}

type Ledger interface {
	GetBlockMeta(num uint64) (*types.BlockMeta, error)
}

// ExternalStack integrates all external interfaces which must be implemented by application users.
//
//go:generate mockgen -destination ./mock_external.go -package rbft -source ./external.go -typed
type ExternalStack[T any, Constraint kittypes.TXConstraint[T]] interface {
	Storage
	Network
	Crypto
	ServiceOutbound[T, Constraint]
	EpochService
	NodeService
	Ledger
}
