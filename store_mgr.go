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

	// Set to the highest weak checkpoint cert we have observed
	highStateTarget *stateUpdateTarget

	// ---------------checkpoint related--------------------
	// checkpoints that we reached by ourselves after commit a block with a
	// block number == integer multiple of K; map lastExec to a base64
	// encoded BlockchainInfo
	chkpts map[uint64]string

	// checkpoint numbers received from others which are bigger than our
	// H(=h+L); map replicaHash to the last checkpoint number received from
	// that replica bigger than H
	hChkpts map[uint64]uint64

	// track all non-repeating checkpoints
	checkpointStore map[pb.Checkpoint]bool
}

// newStoreMgr news an instance of storeManager
func newStoreMgr(c Config) *storeManager {
	sm := &storeManager{
		chkpts:                make(map[uint64]string),
		hChkpts:               make(map[uint64]uint64),
		checkpointStore:       make(map[pb.Checkpoint]bool),
		certStore:             make(map[msgID]*msgCert),
		committedCert:         make(map[msgID]string),
		outstandingReqBatches: make(map[string]*pb.RequestBatch),
		batchStore:            make(map[string]*pb.RequestBatch),
		missingReqBatches:     make(map[string]bool),
		logger:                c.Logger,
	}
	sm.chkpts[0] = "XXX GENESIS"
	return sm
}

// moveWatermarks removes useless set in chkpts, plist, qlist whose index <= h
func (sm *storeManager) moveWatermarks(rbft *rbftImpl, h uint64) {
	for n := range sm.chkpts {
		if n < h {
			delete(sm.chkpts, n)
			rbft.persistDelCheckpoint(n)
		}
	}

	for idx := range rbft.vcMgr.qlist {
		if idx.n <= h {
			delete(rbft.vcMgr.qlist, idx)
		}
	}

	for n := range rbft.vcMgr.plist {
		if n <= h {
			delete(rbft.vcMgr.plist, n)
		}
	}
}

// saveCheckpoint saves checkpoint information to chkpts, whose key is lastExec, value is the global hash of current
// BlockchainInfo
func (sm *storeManager) saveCheckpoint(l uint64, gh string) {
	sm.chkpts[l] = gh
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
