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

	"github.com/samber/lo"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
)

/**
This file provides a mechanism to manage the memory storage in RBFT
*/

// storeManager manages consensus store data structures for RBFT.
type storeManager[T any, Constraint consensus.TXConstraint[T]] struct {
	logger common.Logger

	// track quorum certificates for messages
	certStore map[msgID]*msgCert

	// track the committed cert to help execute
	committedCert map[msgID]string

	// track whether we are waiting for transaction batches to execute
	outstandingReqBatches map[string]*RequestBatch[T, Constraint]

	// track L cached transaction batches produced from requestPool
	batchStore map[string]*RequestBatch[T, Constraint]

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
	higherCheckpoints map[uint64]*consensus.SignedCheckpoint

	// track all non-repeating checkpoints including self and others
	checkpointStore map[chkptID]*consensus.SignedCheckpoint

	// higher view -> cache msg
	wrfHighViewMsgCache map[uint64]*wrfHighViewCacheMsg

	beforeCheckpointEventCache []consensusEvent
}

type wrfHighViewCacheMsg struct {
	prePrepares []*consensus.PrePrepare
	prepares    []*consensus.Prepare
	commits     []*consensus.Commit
}

type stateUpdateTarget struct {
	// target height and digest
	metaState *types.MetaState

	// signed checkpoints that prove the above target
	checkpointSet []*consensus.SignedCheckpoint

	// path of epoch changes from epoch-change-proof
	epochChanges []*consensus.EpochChange
}

// newStoreMgr news an instance of storeManager
func newStoreMgr[T any, Constraint consensus.TXConstraint[T]](c Config) *storeManager[T, Constraint] {
	sm := &storeManager[T, Constraint]{
		localCheckpoints:         make(map[uint64]*consensus.SignedCheckpoint),
		higherCheckpoints:        make(map[uint64]*consensus.SignedCheckpoint),
		checkpointStore:          make(map[chkptID]*consensus.SignedCheckpoint),
		wrfHighViewMsgCache:      make(map[uint64]*wrfHighViewCacheMsg),
		certStore:                make(map[msgID]*msgCert),
		committedCert:            make(map[msgID]string),
		seqMap:                   make(map[uint64]string),
		outstandingReqBatches:    make(map[string]*RequestBatch[T, Constraint]),
		batchStore:               make(map[string]*RequestBatch[T, Constraint]),
		missingReqBatches:        make(map[string]bool),
		missingBatchesInFetching: make(map[string]msgID),
		logger:                   c.Logger,
	}
	return sm
}

// saveCheckpoint saves checkpoint information to localCheckpoints
func (sm *storeManager[T, Constraint]) saveCheckpoint(height uint64, signedCheckpoint *consensus.SignedCheckpoint) {
	sm.localCheckpoints[height] = signedCheckpoint
}

// deleteCheckpoint delete checkpoint information to localCheckpoints
func (sm *storeManager[T, Constraint]) deleteCheckpoint(height uint64) {
	delete(sm.localCheckpoints, height)
}

// Given a digest/view/seq, is there an entry in the certStore?
// If so, return it else, create a new entry
func (sm *storeManager[T, Constraint]) getCert(v uint64, n uint64, d string) *msgCert {
	idx := msgID{v: v, n: n, d: d}
	cert, ok := sm.certStore[idx]

	if ok {
		return cert
	}

	prepare := make(map[string]*consensus.Prepare)
	commit := make(map[string]*consensus.Commit)
	cert = &msgCert{
		prepare: prepare,
		commit:  commit,
	}
	sm.certStore[idx] = cert
	return cert
}

// existedDigest checks if there exists another PRE-PREPARE message in certStore which has the same digest, same view,
// but different seqNo with the given one
func (sm *storeManager[T, Constraint]) existedDigest(v uint64, n uint64, d string) bool {
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
		rbft.logger.Debugf("Replica %d try to receive prePrepare, but it's in viewChange", rbft.chainConfig.SelfID)
		return false
	}

	// backup node reject all pre-prepares with seqNo lower than current ordering config batch.
	if rbft.atomicIn(InConfChange) && preprep.SequenceNumber <= rbft.epochMgr.configBatchInOrder {
		rbft.logger.Debugf("Replica %d try to receive prePrepare for %d, but it's in confChange, "+
			"current config batch in order is %d", rbft.chainConfig.SelfID, preprep.SequenceNumber,
			rbft.epochMgr.configBatchInOrder)
		return false
	}

	if !rbft.inWV(preprep.View, preprep.SequenceNumber) {
		if preprep.SequenceNumber != rbft.chainConfig.H && !rbft.in(SkipInProgress) {
			if !rbft.chainConfig.isWRF() {
				rbft.logger.Warningf("Replica %d received prePrepare with a different view or sequence "+
					"number outside watermarks: prePrep.View %d, expected.View %d, seqNo %d, low water mark %d",
					rbft.chainConfig.SelfID, preprep.View, rbft.chainConfig.View, preprep.SequenceNumber, rbft.chainConfig.H)
			}
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d received prePrepare with a different view or sequence "+
				"number outside watermarks: preprep.View %d, expected.View %d, seqNo %d, low water mark %d",
				rbft.chainConfig.SelfID, preprep.View, rbft.chainConfig.View, preprep.SequenceNumber, rbft.chainConfig.H)
		}
		return false
	}

	// replica rejects prePrepare sent from non-primary.
	if !rbft.isPrimary(preprep.ReplicaId) {
		rbft.logger.Warningf("Replica %d received prePrepare from non-primary: got %d, should be %d",
			rbft.chainConfig.SelfID, preprep.ReplicaId, rbft.chainConfig.PrimaryID)
		return false
	}

	// primary reject prePrepare sent from itself.
	if rbft.isPrimary(rbft.chainConfig.SelfID) {
		rbft.logger.Warningf("Primary %d reject prePrepare sent from itself", rbft.chainConfig.SelfID)
		return false
	}

	if preprep.SequenceNumber <= rbft.exec.lastExec &&
		rbft.prePrepared(preprep.View, preprep.SequenceNumber, preprep.BatchDigest) {
		rbft.logger.Debugf("Replica %d received a prePrepare with seqNo %d lower than lastExec %d and "+
			"we have pre-prepare cert for it, ignore", rbft.chainConfig.SelfID, preprep.SequenceNumber, rbft.exec.lastExec)
		return false
	}

	return true
}

// isPrepareLegal firstly checks if current status can receive prepare or not, then checks prepare message itself is
// legal or not
func (rbft *rbftImpl[T, Constraint]) isPrepareLegal(prep *consensus.Prepare) bool {
	if !rbft.inWV(prep.View, prep.SequenceNumber) {
		if prep.SequenceNumber != rbft.chainConfig.H && !rbft.in(SkipInProgress) {
			if !rbft.chainConfig.isWRF() {
				rbft.logger.Warningf("Replica %d ignore prepare from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
					rbft.chainConfig.SelfID, prep.ReplicaId, prep.View, prep.SequenceNumber, rbft.chainConfig.View, rbft.chainConfig.H)
			}
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d ignore prepare from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.chainConfig.SelfID, prep.ReplicaId, prep.View, prep.SequenceNumber, rbft.chainConfig.View, rbft.chainConfig.H)
		}

		return false
	}

	// if we receive prepare from primary, which means primary behavior as a byzantine, we don't send view change here,
	// because in this case, replicas will eventually find primary abnormal in other cases.
	if rbft.isPrimary(prep.ReplicaId) {
		rbft.logger.Debugf("Replica %d received prepare from primary, ignore it", rbft.chainConfig.SelfID)
		return false
	}
	return true
}

// isCommitLegal checks commit message is legal or not
func (rbft *rbftImpl[T, Constraint]) isCommitLegal(commit *consensus.Commit) bool {
	if !rbft.inWV(commit.View, commit.SequenceNumber) {
		if commit.SequenceNumber != rbft.chainConfig.H && !rbft.in(SkipInProgress) {
			if !rbft.chainConfig.isWRF() {
				rbft.logger.Warningf("Replica %d ignore commit from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
					rbft.chainConfig.SelfID, commit.ReplicaId, commit.View, commit.SequenceNumber, rbft.chainConfig.View, rbft.chainConfig.H)
			}
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d ignore commit from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.chainConfig.SelfID, commit.ReplicaId, commit.View, commit.SequenceNumber, rbft.chainConfig.View, rbft.chainConfig.H)
		}
		return false
	}
	return true
}

func (rbft *rbftImpl[T, Constraint]) getWRFHighViewMsgCache(view uint64) *wrfHighViewCacheMsg {
	cacheMsg, ok := rbft.storeMgr.wrfHighViewMsgCache[view]
	if ok {
		return cacheMsg
	}
	cacheMsg = &wrfHighViewCacheMsg{}
	rbft.storeMgr.wrfHighViewMsgCache[view] = cacheMsg
	return cacheMsg
}

func (rbft *rbftImpl[T, Constraint]) processWRFHighViewMsgs() {
	//	clean and process current view msgs
	for view, cacheMsg := range rbft.storeMgr.wrfHighViewMsgCache {
		if view < rbft.chainConfig.View {
			rbft.logger.Debugf("Replica %d clean lower view cached msgs, for view %d, current view %d",
				rbft.chainConfig.SelfID, view, rbft.chainConfig.View)
			delete(rbft.storeMgr.wrfHighViewMsgCache, view)
		} else if view == rbft.chainConfig.View {
			ctx := context.Background()
			lo.ForEach(cacheMsg.prePrepares, func(item *consensus.PrePrepare, index int) {
				rbft.logger.Debugf("Replica %d resubmit prePrepare from replica %d, "+
					"receive view %d, current view %d, seqNo %d, low water mark %d",
					rbft.chainConfig.SelfID, item.ReplicaId, item.View, rbft.chainConfig.View, item.SequenceNumber, rbft.chainConfig.H)
				_ = rbft.recvPrePrepare(ctx, item)
			})
			lo.ForEach(cacheMsg.prepares, func(item *consensus.Prepare, index int) {
				rbft.logger.Debugf("Replica %d resubmit prepare from replica %d, "+
					"receive view %d, current view %d, seqNo %d, low water mark %d",
					rbft.chainConfig.SelfID, item.ReplicaId, item.View, rbft.chainConfig.View, item.SequenceNumber, rbft.chainConfig.H)
				_ = rbft.recvPrepare(ctx, item)
			})
			lo.ForEach(cacheMsg.commits, func(item *consensus.Commit, index int) {
				rbft.logger.Debugf("Replica %d resubmit commit from replica %d, "+
					"receive view %d, current view %d, seqNo %d, low water mark %d",
					rbft.chainConfig.SelfID, item.ReplicaId, item.View, rbft.chainConfig.View, item.SequenceNumber, rbft.chainConfig.H)
				_ = rbft.recvCommit(ctx, item)
			})
		}
	}
}
