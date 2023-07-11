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
	consensus "github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/v2/types"
)

/**
This file provides a mechanism to manage the memory storage in RBFT
*/

// storeManager manages consensus store data structures for RBFT.
type storeManager struct {
	logger Logger

	// track quorum certificates for messages
	certStore map[msgID]*msgCert

	// track the committed cert to help execute
	committedCert map[msgID]string

	// track whether we are waiting for transaction batches to execute
	outstandingReqBatches map[string]*consensus.RequestBatch

	// track L cached transaction batches produced from requestPool
	batchStore map[string]*consensus.RequestBatch

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
	localCheckpoints map[uint64]*consensus.SignedCheckpoint

	// checkpoint numbers received from others which are bigger than our
	// H(=h+L); map author to the last checkpoint number received from
	// that replica bigger than H
	higherCheckpoints map[string]*consensus.SignedCheckpoint

	// track all non-repeating checkpoints including self and others
	checkpointStore map[chkptID]*consensus.SignedCheckpoint
}

type stateUpdateTarget struct {
	// target height and digest
	metaState *types.MetaState
	// signed checkpoints that prove the above target
	checkpointSet []*consensus.SignedCheckpoint
	// path of epoch changes from epoch-change-proof
	epochChanges []*consensus.QuorumCheckpoint
}

// newStoreMgr news an instance of storeManager
func newStoreMgr[T any, Constraint consensus.TXConstraint[T]](c Config[T, Constraint]) *storeManager {
	sm := &storeManager{
		localCheckpoints:         make(map[uint64]*consensus.SignedCheckpoint),
		higherCheckpoints:        make(map[string]*consensus.SignedCheckpoint),
		checkpointStore:          make(map[chkptID]*consensus.SignedCheckpoint),
		certStore:                make(map[msgID]*msgCert),
		committedCert:            make(map[msgID]string),
		seqMap:                   make(map[uint64]string),
		outstandingReqBatches:    make(map[string]*consensus.RequestBatch),
		batchStore:               make(map[string]*consensus.RequestBatch),
		missingReqBatches:        make(map[string]bool),
		missingBatchesInFetching: make(map[string]msgID),
		logger:                   c.Logger,
	}
	return sm
}

// saveCheckpoint saves checkpoint information to localCheckpoints
func (sm *storeManager) saveCheckpoint(height uint64, signedCheckpoint *consensus.SignedCheckpoint) {
	sm.localCheckpoints[height] = signedCheckpoint
}

// Given a digest/view/seq, is there an entry in the certStore?
// If so, return it else, create a new entry
func (sm *storeManager) getCert(v uint64, n uint64, d string) *msgCert {
	idx := msgID{v, n, d}
	cert, ok := sm.certStore[idx]

	if ok {
		return cert
	}

	prepare := make(map[consensus.Prepare]bool)
	commit := make(map[consensus.Commit]bool)
	cert = &msgCert{
		prepare: prepare,
		commit:  commit,
	}
	sm.certStore[idx] = cert
	return cert
}

// existedDigest checks if there exists another PRE-PREPARE message in certStore which has the same digest, same view,
// but different seqNo with the given one
func (sm *storeManager) existedDigest(v uint64, n uint64, d string) bool {
	for _, cert := range sm.certStore {
		if p := cert.prePrepare; p != nil {
			if p.View == v && p.SequenceNumber != n && p.BatchDigest == d && d != "" {
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

// =============================================================================
// helper functions for check the validity of consensus messages
// =============================================================================
// isPrePrepareLegal firstly checks if current status can receive pre-prepare or not, then checks pre-prepare message
// itself is legal or not
func (rbft *rbftImpl[T, Constraint]) isPrePrepareLegal(preprep *consensus.PrePrepare) bool {

	if rbft.atomicIn(InViewChange) {
		rbft.logger.Debugf("Replica %d try to receive prePrepare, but it's in viewChange", rbft.peerPool.ID)
		return false
	}

	// backup node reject all pre-prepares with seqNo lower than current ordering config batch.
	if rbft.atomicIn(InConfChange) && preprep.SequenceNumber <= rbft.epochMgr.configBatchInOrder {
		rbft.logger.Debugf("Replica %d try to receive prePrepare for %d, but it's in confChange, "+
			"current config batch in order is %d", rbft.peerPool.ID, preprep.SequenceNumber,
			rbft.epochMgr.configBatchInOrder)
		return false
	}

	// replica rejects prePrepare sent from non-primary.
	if !rbft.isPrimary(preprep.ReplicaId) {
		primaryID := rbft.primaryID(rbft.view)
		rbft.logger.Warningf("Replica %d received prePrepare from non-primary: got %d, should be %d",
			rbft.peerPool.ID, preprep.ReplicaId, primaryID)
		return false
	}

	// primary reject prePrepare sent from itself.
	if rbft.isPrimary(rbft.peerPool.ID) {
		rbft.logger.Warningf("Primary %d reject prePrepare sent from itself", rbft.peerPool.ID)
		return false
	}

	if !rbft.inWV(preprep.View, preprep.SequenceNumber) {
		if preprep.SequenceNumber != rbft.h && !rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d received prePrepare with a different view or sequence "+
				"number outside watermarks: prePrep.View %d, expected.View %d, seqNo %d, low water mark %d",
				rbft.peerPool.ID, preprep.View, rbft.view, preprep.SequenceNumber, rbft.h)
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d received prePrepare with a different view or sequence "+
				"number outside watermarks: preprep.View %d, expected.View %d, seqNo %d, low water mark %d",
				rbft.peerPool.ID, preprep.View, rbft.view, preprep.SequenceNumber, rbft.h)
		}
		return false
	}

	if preprep.SequenceNumber <= rbft.exec.lastExec &&
		rbft.prePrepared(preprep.View, preprep.SequenceNumber, preprep.BatchDigest) {
		rbft.logger.Debugf("Replica %d received a prePrepare with seqNo %d lower than lastExec %d and "+
			"we have pre-prepare cert for it, ignore", rbft.peerPool.ID, preprep.SequenceNumber, rbft.exec.lastExec)
		return false
	}

	return true
}

// isPrepareLegal firstly checks if current status can receive prepare or not, then checks prepare message itself is
// legal or not
func (rbft *rbftImpl[T, Constraint]) isPrepareLegal(prep *consensus.Prepare) bool {

	// if we receive prepare from primary, which means primary behavior as a byzantine, we don't send view change here,
	// because in this case, replicas will eventually find primary abnormal in other cases.
	if rbft.isPrimary(prep.ReplicaId) {
		rbft.logger.Debugf("Replica %d received prepare from primary, ignore it", rbft.peerPool.ID)
		return false
	}

	if !rbft.inWV(prep.View, prep.SequenceNumber) {
		if prep.SequenceNumber != rbft.h && !rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d ignore prepare from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.peerPool.ID, prep.ReplicaId, prep.View, prep.SequenceNumber, rbft.view, rbft.h)
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d ignore prepare from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.peerPool.ID, prep.ReplicaId, prep.View, prep.SequenceNumber, rbft.view, rbft.h)
		}

		return false
	}
	return true
}

// isCommitLegal checks commit message is legal or not
func (rbft *rbftImpl[T, Constraint]) isCommitLegal(commit *consensus.Commit) bool {

	if !rbft.inWV(commit.View, commit.SequenceNumber) {
		if commit.SequenceNumber != rbft.h && !rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d ignore commit from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.peerPool.ID, commit.ReplicaId, commit.View, commit.SequenceNumber, rbft.view, rbft.h)
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d ignore commit from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.peerPool.ID, commit.ReplicaId, commit.View, commit.SequenceNumber, rbft.view, rbft.h)
		}
		return false
	}
	return true
}
