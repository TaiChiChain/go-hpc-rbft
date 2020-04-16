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

package external

import (
	"github.com/ultramesh/flato-event/inner/protos"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
)

// Storage is an interface that should be implemented by the application using non-volatile
// DB to store and restore consensus log.
type Storage interface {
	// StoreState stores key-value to non-volatile memory.
	StoreState(key string, value []byte) error
	// DelState deletes data with specified key from non-volatile memory.
	DelState(key string) error
	// ReadState retrieves data with specified key from non-volatile memory.
	ReadState(key string) ([]byte, error)
	// ReadState retrieves data with specified key prefix from non-volatile memory.
	ReadStateSet(key string) (map[string][]byte, error)
	// Destroy clears the non-volatile memory.
	Destroy() error
}

// Network is used to send p2p messages between nodes.
type Network interface {
	// Broadcast delivers messages to all other nodes.
	Broadcast(msg *pb.ConsensusMessage) error
	// Unicast delivers messages to given node with specified id.
	Unicast(msg *pb.ConsensusMessage, to uint64) error
	UnicastByHash(msg *pb.ConsensusMessage, to string) error
	// UpdateTable updates routing table according to given confChangeType, which includes:
	// 1. ConfChangeType_ConfChangeAddNode, append one node to routing table
	// 2. ConfChangeType_ConfChangeRemoveNode, remove one node with given id from routing table
	// 3. ConfChangeType_ConfChangeUpdateNode, update routing table with given meta info
	// It's application's responsibility to ensure update routing table synchronously.
	UpdateTable(change *pb.ConfChange)
}

// Crypto is used to access the sign/verify methods from the crypto package
type Crypto interface {
	// Sign signs messages, returns error if any.
	Sign(msg []byte) ([]byte, error)
	// Verify verifies signature signed with msg from given peerID, return nil if verify successfully
	Verify(peerID uint64, signature []byte, msg []byte) error
}

// ServiceOutbound is the application service invoked by RBFT library which includes two core events:
// 1. Execute is invoked when RBFT core has achieved consensus on txs with batch number seqNo,
//    which will be submitted to application service. After application submitted the given batch,
//    application should call ServiceInbound.ReportExecuted to inform RBFT library the latest
//    service state.
// 2. StateUpdate is invoked when RBFT core finds current node out-of-date from other nodes by too may
//    seqNos, it's applications responsibility to implement a fast sync algorithm to ensure node
//    can catch up as soon as possible. Applications should call ServiceInbound.ReportStateUpdated
//    to inform RBFT library the latest service state after StateUpdate.
type ServiceOutbound interface {
	// Execute informs application layer to apply one batch with given request list and batch seqNo.
	// Users can apply different batches asynchronously but ensure the order by seqNo.
	Execute(txs []*protos.Transaction, localList []bool, seqNo uint64, timestamp int64)
	// StateUpdate informs application layer to catch up to given seqNo with specified state digest.
	StateUpdate(seqNo uint64, digest string, peers []uint64)
	// SendFilterEvent posts some impotent events to application layer.
	// Users can decide to post filer event synchronously or asynchronously.
	SendFilterEvent(informType pb.InformType, message ...interface{})
}

// ExternalStack integrates all external interfaces which must be implemented by application users.
type ExternalStack interface {
	Storage
	Network
	Crypto
	ServiceOutbound
}
