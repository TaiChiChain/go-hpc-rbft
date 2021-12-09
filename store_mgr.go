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
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	"github.com/ultramesh/flato-rbft/types"
)

/**
This file provide a mechanism to manage the memory storage in RBFT
*/

// storeManager manages common store data structures for RBFT.
type storeManager struct {
	logger Logger

	// track quorum certificates for messages
	certStore map[msgID]*msgCert

	// track the committed cert to help execute
	committedCert map[msgID]string

	// track whether we are waiting for transaction batches to execute
	outstandingReqBatches map[string]*pb.RequestBatch

	// track L cached transaction batches produced from requestPool
	batchStore map[string]*pb.RequestBatch

	// for all the assigned, non-checkpointed request batches we might miss
	// some transactions in some batches, record batch no
	missingReqBatches map[string]bool

	// used by backup node to record all missing tx batches which are in fetching
	// to avoid fetch the same batch repeatedly.
	// map missing batch digest to batch seqNo.
	missingBatchesInFetching map[string]msgID

	// a pre-prepare sequence map,
	// there need to be a one-to-one correspondence between sequence number and digest
	seqMap map[uint64]string

	// Set to the highest weak checkpoint cert we have observed
	highStateTarget *stateUpdateTarget

	// ---------------checkpoint related--------------------
	// checkpoints that we reached by ourselves after commit a block with a
	// block number == integer multiple of K;
	// map lastExec to signed checkpoint after executed certain block
	localCheckpoints map[uint64]*pb.SignedCheckpoint

	// checkpoint numbers received from others which are bigger than our
	// H(=h+L); map replicaHash to the last checkpoint number received from
	// that replica bigger than H
	higherCheckpoints map[string]*pb.SignedCheckpoint

	// track all non-repeating checkpoints including self and others
	checkpointStore map[chkptID]*pb.SignedCheckpoint
}

type stateUpdateTarget struct {
	// target height and digest
	metaState *types.MetaState
	// signed checkpoints that prove the above target
	checkpointSet []*pb.SignedCheckpoint
}

// newStoreMgr news an instance of storeManager
func newStoreMgr(c Config) *storeManager {
	sm := &storeManager{
		localCheckpoints:         make(map[uint64]*pb.SignedCheckpoint),
		higherCheckpoints:        make(map[string]*pb.SignedCheckpoint),
		checkpointStore:          make(map[chkptID]*pb.SignedCheckpoint),
		certStore:                make(map[msgID]*msgCert),
		committedCert:            make(map[msgID]string),
		seqMap:                   make(map[uint64]string),
		outstandingReqBatches:    make(map[string]*pb.RequestBatch),
		batchStore:               make(map[string]*pb.RequestBatch),
		missingReqBatches:        make(map[string]bool),
		missingBatchesInFetching: make(map[string]msgID),
		logger:                   c.Logger,
	}
	return sm
}

// saveCheckpoint saves checkpoint information to localCheckpoints, whose key is lastExec, value is the global nodeHash of current
// BlockchainInfo
func (sm *storeManager) saveCheckpoint(height uint64, signedCheckpoint *pb.SignedCheckpoint) {
	sm.localCheckpoints[height] = signedCheckpoint
}

// Given a digest/view/seq, is there an entry in the certStore?
// If so, return it else, create a new entry
func (sm *storeManager) getCert(v uint64, n uint64, d string) (cert *msgCert) {
	idx := msgID{v, n, d}
	cert, ok := sm.certStore[idx]

	if ok {
		return
	}

	prepare := make(map[pb.Prepare]bool)
	commit := make(map[pb.Commit]bool)
	cert = &msgCert{
		prepare: prepare,
		commit:  commit,
	}
	sm.certStore[idx] = cert
	return
}

// existedDigest checks if there exists another PRE-PREPARE message in certStore which has the same digest, same view,
// but different seqNo with the given one
func (sm *storeManager) existedDigest(n uint64, view uint64, digest string) bool {
	for _, cert := range sm.certStore {
		if p := cert.prePrepare; p != nil {
			if p.View == view && p.SequenceNumber != n && p.BatchDigest == digest && digest != "" {
				// This will happen if primary receive same digest result of txs
				// It may result in DDos attack
				sm.logger.Warningf("Other prePrepare found with same digest but different seqNo: %d "+
					"instead of %d", p.SequenceNumber, n)
				return true
			}
		}
	}
	return false
}
