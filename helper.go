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
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/sha3"
)

// =============================================================================
// helper functions for sort
// =============================================================================
type sortableUint64List []uint64

func (a sortableUint64List) Len() int {
	return len(a)
}
func (a sortableUint64List) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a sortableUint64List) Less(i, j int) bool {
	return a[i] < a[j]
}

// =============================================================================
// helper functions for RBFT
// =============================================================================

// primaryID returns the expected primary id with the given view v
func (rbft *rbftImpl[T, Constraint]) primaryID(v uint64) uint64 {
	// calculate primary id by view
	primaryID := v%uint64(rbft.N) + 1
	return primaryID
}

// isPrimary returns if current node is primary or not
func (rbft *rbftImpl[T, Constraint]) isPrimary(id uint64) bool {
	return rbft.primaryID(rbft.view) == id
}

// InW returns if the given seqNo is higher than h or not
func (rbft *rbftImpl[T, Constraint]) inW(n uint64) bool {
	return n > rbft.h
}

// InV returns if the given view equals the current view or not
func (rbft *rbftImpl[T, Constraint]) inV(v uint64) bool {
	return rbft.view == v
}

// InWV firstly checks if the given view is inV then checks if the given seqNo n is inW
func (rbft *rbftImpl[T, Constraint]) inWV(v uint64, n uint64) bool {
	return rbft.inV(v) && rbft.inW(n)
}

// sendInW used in maybeSendPrePrepare checks the given seqNo is between low
// watermark and high watermark or not.
func (rbft *rbftImpl[T, Constraint]) sendInW(n uint64) bool {
	return n > rbft.h && n <= rbft.h+rbft.L
}

// beyondRange is used to check the given seqNo is out of high-watermark or not
func (rbft *rbftImpl[T, Constraint]) beyondRange(n uint64) bool {
	return n > rbft.h+rbft.L
}

// cleanAllBatchAndCert cleans all outstandingReqBatches and committedCert
func (rbft *rbftImpl[T, Constraint]) cleanOutstandingAndCert() {
	rbft.storeMgr.outstandingReqBatches = make(map[string]*consensus.RequestBatch)
	rbft.storeMgr.committedCert = make(map[msgID]string)

	rbft.metrics.outstandingBatchesGauge.Set(float64(0))
}

// When N=3F+1, this should be 2F+1 (N-F)
// More generally, we need every two consensus case quorum of size X to intersect in at least F+1
// hence 2X>=N+F+1
func (rbft *rbftImpl[T, Constraint]) commonCaseQuorum() int {
	return int(math.Ceil(float64(rbft.N+rbft.f+1) / float64(2)))
}

// oneCorrectQuorum returns the number of replicas in which correct numbers must be bigger than incorrect number
func (rbft *rbftImpl[T, Constraint]) allCorrectReplicasQuorum() int {
	return rbft.N - rbft.f
}

// oneCorrectQuorum returns the number of replicas in which there must exist at least one correct replica
func (rbft *rbftImpl[T, Constraint]) oneCorrectQuorum() int {
	return rbft.f + 1
}

// =============================================================================
// pre-prepare/prepare/commit check helper
// =============================================================================

// prePrepared returns if there existed a pre-prepare message in certStore with the given view,seqNo,digest
func (rbft *rbftImpl[T, Constraint]) prePrepared(v uint64, n uint64, d string) bool {
	cert := rbft.storeMgr.certStore[msgID{v, n, d}]

	if cert != nil {
		p := cert.prePrepare
		if p != nil && p.View == v && p.SequenceNumber == n && p.BatchDigest == d {
			return true
		}
	}

	rbft.logger.Debugf("Replica %d does not have view=%d/seqNo=%d prePrepared", rbft.peerPool.ID, v, n)

	return false
}

// prepared firstly checks if the cert with the given msgID has been prePrepared,
// then checks if this node has collected enough prepare messages for the cert with given msgID
func (rbft *rbftImpl[T, Constraint]) prepared(v uint64, n uint64, d string) bool {

	if !rbft.prePrepared(v, n, d) {
		return false
	}

	cert := rbft.storeMgr.certStore[msgID{v, n, d}]

	prepCount := len(cert.prepare)

	rbft.logger.Debugf("Replica %d prepare count for view=%d/seqNo=%d is %d",
		rbft.peerPool.ID, v, n, prepCount)

	return prepCount >= rbft.commonCaseQuorum()-1
}

// committed firstly checks if the cert with the given msgID has been prepared,
// then checks if this node has collected enough commit messages for the cert with given msgID
func (rbft *rbftImpl[T, Constraint]) committed(v uint64, n uint64, d string) bool {

	if !rbft.prepared(v, n, d) {
		return false
	}

	cert := rbft.storeMgr.certStore[msgID{v, n, d}]

	cmtCount := len(cert.commit)

	rbft.logger.Debugf("Replica %d commit count for view=%d/seqNo=%d is %d",
		rbft.peerPool.ID, v, n, cmtCount)

	return cmtCount >= rbft.commonCaseQuorum()
}

// =============================================================================
// helper functions for transfer message
// =============================================================================

// broadcastReqSet helps broadcast requestSet to others.
func (rbft *rbftImpl[T, Constraint]) broadcastReqSet(set *consensus.RequestSet) {
	payload, err := proto.Marshal(set)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_TRANSACTION_SET Marshal Error: %s", err)
		return
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_REQUEST_SET,
		Payload: payload,
	}
	rbft.peerPool.broadcast(context.TODO(), consensusMsg)
}

// =============================================================================
// helper functions for timer
// =============================================================================

// startTimerIfOutstandingRequests soft starts a new view timer if there exists some outstanding request batches,
// else reset the null request timer
func (rbft *rbftImpl[T, Constraint]) startTimerIfOutstandingRequests() {
	if rbft.in(SkipInProgress) {
		// Do not start the view change timer if we are executing or state transferring, these take arbitrarily long amounts of time
		return
	}

	if len(rbft.storeMgr.outstandingReqBatches) > 0 {
		outStandingBatchIdxes := make([]msgID, 0)
		for digest, batch := range rbft.storeMgr.outstandingReqBatches {
			outStandingBatchIdxes = append(outStandingBatchIdxes, msgID{n: batch.SeqNo, d: digest})
		}
		rbft.softStartNewViewTimer(rbft.timerMgr.getTimeoutValue(requestTimer), fmt.Sprintf("outstanding request "+
			"batches num=%v, batches: %v", len(outStandingBatchIdxes), outStandingBatchIdxes), false)
	} else if rbft.timerMgr.getTimeoutValue(nullRequestTimer) > 0 {
		rbft.nullReqTimerReset()
	}
}

// nullReqTimerReset reset the null request timer with a certain timeout, for different replica, null request timeout is
// different:
// 1. for primary, null request timeout is the timeout written in the config
// 2. for non-primary, null request timeout =3*(timeout written in the config)+request timeout
func (rbft *rbftImpl[T, Constraint]) nullReqTimerReset() {
	timeout := rbft.timerMgr.getTimeoutValue(nullRequestTimer)
	if !rbft.isPrimary(rbft.peerPool.ID) {
		// we're waiting for the primary to deliver a null request - give it a bit more time
		timeout = 3*timeout + rbft.timerMgr.getTimeoutValue(requestTimer)
	}

	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNullRequestTimerEvent,
	}

	rbft.timerMgr.startTimerWithNewTT(nullRequestTimer, timeout, event)
}

// compareCheckpointWithWeakSet first checks the legality of this checkpoint, which seqNo
// must between [h, H] and we haven't received a same checkpoint message, then find the
// weak set with more than f + 1 members who have sent a checkpoint with the same seqNo
// and ID, if there exists more than one weak sets, we'll never find a stable cert for this
// seqNo, else checks if self's generated checkpoint has the same ID with the given one,
// if not, directly start state update to recover to a correct state.
func (rbft *rbftImpl[T, Constraint]) compareCheckpointWithWeakSet(signedCheckpoint *consensus.SignedCheckpoint) (bool, []*consensus.SignedCheckpoint) {
	// if checkpoint height is lower than current low watermark, ignore it as we have reached a higher h,
	// else, continue to find f+1 checkpoint messages with the same seqNo and ID
	checkpointHeight := signedCheckpoint.Checkpoint.Height()
	checkpointHash := hex.EncodeToString(signedCheckpoint.Checkpoint.Hash())
	if !rbft.inW(checkpointHeight) {
		if checkpointHeight != rbft.h && !rbft.in(SkipInProgress) {
			// It is perfectly normal that we receive checkpoints for the watermark we just raised, as we raise it after 2f+1, leaving f replies left
			rbft.logger.Warningf("SignedCheckpoint sequence number outside watermarks: seqNo %d, low water mark %d", checkpointHeight, rbft.h)
		} else {
			rbft.logger.Debugf("SignedCheckpoint sequence number outside watermarks: seqNo %d, low water mark %d", checkpointHeight, rbft.h)
		}
		return false, nil
	}

	cID := chkptID{
		author:   signedCheckpoint.GetAuthor(),
		sequence: checkpointHeight,
	}

	_, ok := rbft.storeMgr.checkpointStore[cID]
	if ok {
		rbft.logger.Warningf("Replica %d received duplicate checkpoint from replica %s "+
			"for seqNo %d, update storage", rbft.peerPool.ID, cID.author, cID.sequence)
	}
	rbft.storeMgr.checkpointStore[cID] = signedCheckpoint

	// track how much different checkpoint values we have for the seqNo.
	diffValues := make(map[string][]*consensus.SignedCheckpoint)
	// track how many "correct"(more than f + 1) checkpoint values we have for the seqNo.
	var correctHashes []string

	// track totally matching checkpoints.
	matching := 0
	for cp, signedChkpt := range rbft.storeMgr.checkpointStore {
		hash := hex.EncodeToString(signedChkpt.Checkpoint.Hash())
		if cp.sequence != checkpointHeight {
			continue
		}

		if hash == checkpointHash {
			matching++
		}

		if _, exist := diffValues[hash]; !exist {
			diffValues[hash] = []*consensus.SignedCheckpoint{signedChkpt}
		} else {
			diffValues[hash] = append(diffValues[hash], signedChkpt)
		}

		// if current network contains more than f + 1 checkpoints with the same seqNo
		// but different ID, we'll never be able to get a stable cert for this seqNo.
		if len(diffValues) > rbft.oneCorrectQuorum() {
			rbft.logger.Criticalf("Replica %d cannot find stable checkpoint with seqNo %d"+
				"(%d different values observed already).", rbft.peerPool.ID, checkpointHeight, len(diffValues))
			tc := consensus.TagContentInconsistentCheckpoint{
				Height: checkpointHeight,
			}
			checkpointSet := make(map[consensus.CommonCheckpointState][]string)
			for _, value := range diffValues {
				epoch := value[0].Checkpoint.NextEpoch()
				digest := value[0].Checkpoint.Digest()
				authors := make([]string, 0, len(value))
				for _, sc := range value {
					authors = append(authors, sc.GetAuthor())
				}
				checkpointSet[consensus.CommonCheckpointState{Epoch: epoch, Hash: digest}] = authors
			}
			tc.CheckpointSet = checkpointSet
			rbft.logger.Trace(consensus.TagNameCheckpoint, consensus.TagStageWarning, tc)
			rbft.on(Inconsistent)
			rbft.metrics.statusGaugeInconsistent.Set(Inconsistent)
			rbft.setAbNormal()
			rbft.stopNamespace()
			return false, nil
		}

		// record all correct checkpoint(weak cert) values.
		if len(diffValues[hash]) == rbft.oneCorrectQuorum() {
			correctHashes = append(correctHashes, hash)
		}
	}

	if len(correctHashes) == 0 {
		rbft.logger.Debugf("Replica %d hasn't got a weak cert for checkpoint %d", rbft.peerPool.ID, checkpointHeight)
		return true, nil
	}

	// if we encounter more than one correct weak set, we will never recover to a stable
	// consensus state.
	if len(correctHashes) > 1 {
		rbft.logger.Criticalf("Replica %d finds several weak certs for checkpoint %d, values: %v", rbft.peerPool.ID, checkpointHeight, correctHashes)
		rbft.on(Inconsistent)
		rbft.metrics.statusGaugeInconsistent.Set(Inconsistent)
		rbft.setAbNormal()
		rbft.stopNamespace()
		return false, nil
	}

	// if we can only find one weak cert with the same seqNo and ID, our generated checkpoint(if
	// existed) must have the same ID with that one.
	correctCheckpoints := diffValues[correctHashes[0]]
	correctID := correctCheckpoints[0].Checkpoint.Digest()
	localCheckpoint, exist := rbft.storeMgr.localCheckpoints[checkpointHeight]

	// if self's checkpoint with the same seqNo has a distinguished ID with a weak certs'
	// checkpoint ID, we should trigger state update right now to recover self block state.
	if exist && localCheckpoint.Checkpoint.Digest() != correctID {
		rbft.logger.Criticalf("Replica %d generated a checkpoint of %s, but a weak set of the network agrees on %s.",
			rbft.peerPool.ID, localCheckpoint, correctID)

		target := &types.MetaState{
			Height: checkpointHeight,
			Digest: correctID,
		}
		rbft.updateHighStateTarget(target, correctCheckpoints) // for incorrect state
		rbft.tryStateTransfer()
		return false, nil
	}

	return true, correctCheckpoints
}

// compareWholeStates compares whole networks' current status during sync state
// including :
// 1. view: current view of bft network
// 2. height: current latest blockChain height
// 3. digest: current latest blockChain hash
func (rbft *rbftImpl[T, Constraint]) compareWholeStates(states wholeStates) consensusEvent {
	// track all replica with same state used to find quorum consistent state
	sameRespRecord := make(map[nodeState][]*consensus.SignedCheckpoint)

	// check if we can find quorum nodeState who have the same view, height and digest, if we can
	// find, which means quorum nodes agree to same state, save to quorumRsp, set canFind to true
	// and update view if needed
	var quorumResp nodeState
	canFind := false

	// find the quorum nodeState
	for key, state := range states {
		sameRespRecord[state] = append(sameRespRecord[state], key)
		if len(sameRespRecord[state]) >= rbft.commonCaseQuorum() {
			rbft.logger.Debugf("Replica %d find quorum states, try to process", rbft.peerPool.ID)
			quorumResp = state
			canFind = true
			break
		}
	}

	// we can find the quorum nodeState with the same view, judge if the response.view equals to the
	// current view, if so, trigger recovery view change to recover view.
	if canFind {
		// trigger recovery view change if current view is incorrect.
		if rbft.view != quorumResp.view {
			rbft.logger.Noticef("Replica %d try to view change to quorum view %d in view %d",
				rbft.peerPool.ID, quorumResp.view, rbft.view)
			return rbft.initRecovery()
		}

		// get self-state to compare
		state := rbft.node.getCurrentState()
		if state == nil {
			rbft.logger.Warningf("Replica %d has a nil state", rbft.peerPool.ID)
			return nil
		}

		// we could stop sync-state timer here as we have already found quorum sync-state-response
		rbft.timerMgr.stopTimer(syncStateRspTimer)
		rbft.off(InSyncState)

		// case 1) wrong height [sync]:
		// self height of blocks is lower than others
		// trigger recovery
		if state.MetaState.Height != quorumResp.height {
			rbft.logger.Noticef("Replica %d finds quorum same block state which is different from self,"+
				"self height: %d, quorum height: %d", rbft.peerPool.ID, state.MetaState.Height, quorumResp.height)

			// node in lower height cannot become a primary node
			if rbft.isPrimary(rbft.peerPool.ID) {
				rbft.logger.Warningf("Primary %d finds itself not sync with quorum replicas, sending viewChange", rbft.peerPool.ID)
				return rbft.sendViewChange()
			}
			rbft.logger.Infof("Replica %d finds itself not sync with quorum replicas, try to recovery", rbft.peerPool.ID)
			return rbft.initRecovery()
		}

		// case 2) wrong block hash [error]:
		// we have correct block-height, but the hash of the latest block is wrong
		// trigger state-update
		if state.MetaState.Height == quorumResp.height && state.MetaState.Digest != quorumResp.digest {
			rbft.logger.Errorf("Replica %d finds quorum same block state whose hash is different from self,"+
				"in height: %d, selfHash: %s, quorumDigest: %s, need to state update",
				rbft.peerPool.ID, quorumResp.height, state.MetaState.Digest, quorumResp.digest)

			target := &types.MetaState{
				Height: quorumResp.height,
				Digest: quorumResp.digest,
			}
			rbft.updateHighStateTarget(target, sameRespRecord[quorumResp]) // for incorrect state
			rbft.tryStateTransfer()
			return nil
		}

		rbft.logger.Infof("======== Replica %d finished sync state for height: %d, epoch: %d, view %d",
			rbft.peerPool.ID, state.MetaState.Height, rbft.epoch, rbft.view)
		rbft.external.SendFilterEvent(types.InformTypeFilterStableCheckpoint, sameRespRecord[quorumResp])
	}

	return nil
}

// calcQSet selects Pre-prepares which satisfy the following conditions
// 1. Pre-prepares in previous qlist
// 2. Pre-prepares from certStore which is preprepared and its view <= its idx.v or not in qlist
func (rbft *rbftImpl[T, Constraint]) calcQSet() map[qidx]*consensus.Vc_PQ {

	qset := make(map[qidx]*consensus.Vc_PQ)

	for n, q := range rbft.vcMgr.qlist {
		qset[n] = q
	}

	for idx := range rbft.storeMgr.certStore {

		if !rbft.prePrepared(idx.v, idx.n, idx.d) {
			continue
		}

		qi := qidx{idx.d, idx.n}
		if q, ok := qset[qi]; ok && q.View > idx.v {
			continue
		}

		qset[qi] = &consensus.Vc_PQ{
			SequenceNumber: idx.n,
			BatchDigest:    idx.d,
			View:           idx.v,
		}
	}

	return qset
}

// calcPSet selects prepares which satisfy the following conditions:
// 1. prepares in previous qlist
// 2. prepares from certStore which is prepared and (its view <= its idx.v or not in plist)
func (rbft *rbftImpl[T, Constraint]) calcPSet() map[uint64]*consensus.Vc_PQ {

	pset := make(map[uint64]*consensus.Vc_PQ)

	for n, p := range rbft.vcMgr.plist {
		pset[n] = p
	}

	for idx := range rbft.storeMgr.certStore {

		if !rbft.prepared(idx.v, idx.n, idx.d) {
			continue
		}

		if p, ok := pset[idx.n]; ok && p.View > idx.v {
			continue
		}

		pset[idx.n] = &consensus.Vc_PQ{
			SequenceNumber: idx.n,
			BatchDigest:    idx.d,
			View:           idx.v,
		}
	}

	return pset
}

// getVcBasis helps re-calculate the plist and qlist then construct a vcBasis
// at teh same time, useless cert with lower .
func (rbft *rbftImpl[T, Constraint]) getVcBasis() *consensus.VcBasis {
	basis := &consensus.VcBasis{
		View:      rbft.view,
		H:         rbft.h,
		ReplicaId: rbft.peerPool.ID,
	}

	// clear qList and pList from DB as we will construct new QPList next.
	rbft.persistDelQPList()

	rbft.vcMgr.plist = rbft.calcPSet()
	rbft.vcMgr.qlist = rbft.calcQSet()

	// Note. before vc, we need to persist QPList to ensure we can restore committed entries after
	// above abnormal situations as we will delete all PQCSet when we enter abnormal, after finish vc,
	// we will re-broadcast and persist PQCSet which is enough to ensure continuity of committed entries in
	// next vc. However, QPList cannot be deleted immediately after finish vc as we may have missed
	// some committed entries after crash down in normal status.
	// So:
	// 1. during normal status, we have: QPSet with pre-prepare certs and prepare certs and QPList generated in
	// previous abnormal status which is used to catch some useful committed entries after system crash down.
	// 2. during abnormal status, we have no QPSet but we have QPList generated in current abnormal status.
	rbft.persistPList(rbft.vcMgr.plist)
	rbft.persistQList(rbft.vcMgr.qlist)

	for idx := range rbft.storeMgr.certStore {
		if idx.v < rbft.view {
			rbft.logger.Debugf("Replica %d clear cert with view=%d/seqNo=%d/digest=%s when construct VcBasis",
				rbft.peerPool.ID, idx.v, idx.n, idx.d)
			delete(rbft.storeMgr.certStore, idx)
			delete(rbft.storeMgr.seqMap, idx.n)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
		}
	}

	basis.Pset, basis.Qset, basis.Cset = rbft.gatherPQC()

	return basis
}

// gatherPQC just gather all checkpoints, p entries and q entries.
func (rbft *rbftImpl[T, Constraint]) gatherPQC() (pset []*consensus.Vc_PQ, qset []*consensus.Vc_PQ, signedCheckpoints []*consensus.SignedCheckpoint) {
	// Gather all the checkpoints
	rbft.logger.Debugf("Replica %d gather CSet:", rbft.peerPool.ID)
	for n, signedCheckpoint := range rbft.storeMgr.localCheckpoints {
		signedCheckpoints = append(signedCheckpoints, signedCheckpoint)
		rbft.logger.Debugf("seqNo: %d, ID: %s", n, signedCheckpoint.Checkpoint.Digest())
	}
	// Gather all the p entries
	rbft.logger.Debugf("Replica %d gather PSet:", rbft.peerPool.ID)
	for _, p := range rbft.vcMgr.plist {
		if p.SequenceNumber < rbft.h {
			rbft.logger.Errorf("Replica %d should not have anything in our pset less than h, found %+v", rbft.peerPool.ID, p)
			continue
		}
		pset = append(pset, p)
		rbft.logger.Debugf("seqNo: %d, view: %d, digest: %s", p.SequenceNumber, p.View, p.BatchDigest)
	}

	// Gather all the q entries
	rbft.logger.Debugf("Replica %d gather QSet:", rbft.peerPool.ID)
	for _, q := range rbft.vcMgr.qlist {
		if q.SequenceNumber < rbft.h {
			rbft.logger.Errorf("Replica %d should not have anything in our qset less than h, found %+v", rbft.peerPool.ID, q)
			continue
		}
		qset = append(qset, q)
		rbft.logger.Debugf("seqNo: %d, view: %d, digest: %s", q.SequenceNumber, q.View, q.BatchDigest)
	}

	return
}

// putBackRequestBatches reset all txs into 'non-batched' state in requestPool to prepare re-arrange by order.
func (rbft *rbftImpl[T, Constraint]) putBackRequestBatches(xset []*consensus.Vc_PQ) {

	// remove all the batches that smaller than initial checkpoint.
	// those batches are the dependency of duplicator,
	// but we can remove since we already have checkpoint after viewChange.
	var deleteList []string
	for digest, batch := range rbft.storeMgr.batchStore {
		if batch.SeqNo <= rbft.h {
			rbft.logger.Debugf("Replica %d clear batch %s with seqNo %d <= initial checkpoint %d", rbft.peerPool.ID, digest, batch.SeqNo, rbft.h)
			delete(rbft.storeMgr.batchStore, digest)
			rbft.persistDelBatch(digest)
			deleteList = append(deleteList, digest)
		}
	}
	rbft.metrics.batchesGauge.Set(float64(len(rbft.storeMgr.batchStore)))
	rbft.batchMgr.requestPool.RemoveBatches(deleteList)

	if !rbft.batchMgr.requestPool.IsPoolFull() {
		rbft.setNotFull()
	}

	// directly restore all batchedTxs back into non-batched txs and re-arrange them by order when processNewView.
	rbft.batchMgr.requestPool.RestorePool()

	// clear cacheBatch as they are useless and all related batches have been restored in requestPool.
	rbft.batchMgr.cacheBatch = nil

	rbft.metrics.cacheBatchNumber.Set(float64(0))

	hashListMap := make(map[string]bool)
	for _, msg := range xset {
		hashListMap[msg.BatchDigest] = true
	}

	// don't remove those batches which are not contained in xSet from batchStore as they may be useful
	// in next viewChange round.
	for digest := range rbft.storeMgr.batchStore {
		if !hashListMap[digest] {
			rbft.logger.Debugf("Replica %d finds temporarily useless batch %s which is not contained in xSet", rbft.peerPool.ID, digest)
		}
	}
}

// checkIfNeedStateUpdate checks if a replica needs to do state update
func (rbft *rbftImpl[T, Constraint]) checkIfNeedStateUpdate(checkpointState *types.CheckpointState, checkpointSet []*consensus.SignedCheckpoint) bool {
	// initial checkpoint height and digest.
	initialCheckpointHeight := checkpointState.Meta.Height
	initialCheckpointDigest := checkpointState.Meta.Digest

	// if current low watermark is lower than initial checkpoint, but we have executed to
	// initial checkpoint height.
	if rbft.h < initialCheckpointHeight && rbft.storeMgr.localCheckpoints[initialCheckpointHeight] != nil {
		// if we have reached this checkpoint height locally but haven't moved h to
		// this height(maybe caused by missing checkpoint msg from other nodes),
		// sync config checkpoint directly to avoid missing some config checkpoint
		// on ledger.
		localDigest := rbft.storeMgr.localCheckpoints[initialCheckpointHeight].Checkpoint.Digest()
		if localDigest == initialCheckpointDigest {
			if checkpointState.IsConfig {
				rbft.logger.Noticef("Replica %d finds config checkpoint %d when checkIfNeedStateUpdate",
					rbft.peerPool.ID, initialCheckpointHeight)
				rbft.atomicOn(InConfChange)
				rbft.epochMgr.configBatchInOrder = initialCheckpointHeight
				rbft.metrics.statusGaugeInConfChange.Set(InConfChange)
				// sync config checkpoint with ledger.
				success := rbft.syncConfigCheckpoint(initialCheckpointHeight, checkpointSet)
				if !success {
					rbft.logger.Warningf("Replica %d failed to sync config checkpoint", rbft.peerPool.ID)
				}
				rbft.atomicOff(InConfChange)
				rbft.metrics.statusGaugeInConfChange.Set(0)
			} else {
				rbft.logger.Debugf("Replica %d finds normal checkpoint %d when checkIfNeedStateUpdate",
					rbft.peerPool.ID, initialCheckpointHeight)
				rbft.moveWatermarks(initialCheckpointHeight, false)
			}
			return false
		}
		rbft.logger.Warningf("Replica %d finds mismatch checkpoint %d when checkIfNeedStateUpdate, "+
			"local digest: %s, quorum digest %s, try to sync chain", rbft.peerPool.ID, initialCheckpointHeight,
			localDigest, initialCheckpointDigest)
		rbft.updateHighStateTarget(&checkpointState.Meta, checkpointSet) // for incorrect state
		rbft.tryStateTransfer()
		return true
	}

	// If replica's lastExec < initial checkpoint, replica is out of date
	lastExec := rbft.exec.lastExec
	if lastExec < initialCheckpointHeight {
		rbft.logger.Warningf("Replica %d missing base checkpoint %d (%s), our most recent execution %d",
			rbft.peerPool.ID, initialCheckpointHeight, initialCheckpointDigest, lastExec)
		rbft.updateHighStateTarget(&checkpointState.Meta, checkpointSet) // for backwardness
		rbft.tryStateTransfer()
		return true
	}
	return false
}

func (rbft *rbftImpl[T, Constraint]) equalMetaState(s1 *types.MetaState, s2 *types.MetaState) bool {
	rbft.logger.Debugf("Replica %d check if meta states are equal: 1)%+v, 2)%+v", rbft.peerPool.ID, s1, s2)

	// nil pointer cannot be checked
	if s1 == nil || s2 == nil {
		return false
	}

	// check the height number
	if s1.Height != s2.Height {
		return false
	}
	// check the digest of the state
	if s1.Digest != s2.Digest {
		return false
	}
	return true
}

func (rbft *rbftImpl[T, Constraint]) stopNamespace() {
	defer func() {
		// delFlag channel might be closed by other modules at the same time
		// consensus requests to stop namespace
		if err := recover(); err != nil {
			rbft.logger.Warningf("Replica %d stops namespace error: %s", rbft.peerPool.ID, err)
		}
	}()

	rbft.logger.Criticalf("Replica %d requests to stop namespace", rbft.peerPool.ID)
	rbft.delFlag <- true
}

func requestHash[T any, Constraint consensus.TXConstraint[T]](data []byte) string {
	tx, err := consensus.DecodeTx[T, Constraint](data)
	if err != nil {
		panic("decodeTx err in requestHash")
	}
	return Constraint(tx).RbftGetTxHash()
}

// calculateMD5Hash calculate hash by MD5
func calculateMD5Hash(list []string, timestamp int64) string {
	h := md5.New()
	for _, hash := range list {
		_, _ = h.Write([]byte(hash))
	}
	if timestamp > 0 {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(timestamp))
		_, _ = h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func drainChannel(ch chan consensusEvent) ([][]byte, error) {
	remainTxs := make([][]byte, 0)

	for len(ch) > 0 {
		select {
		case ee := <-ch:
			switch e := ee.(type) {
			case *consensus.RequestSet:
				for _, tx := range e.Requests {
					remainTxs = append(remainTxs, tx)
				}
			default:
			}
		default:
		}
	}
	return remainTxs, nil
}

// generateSignedCheckpoint generates a signed checkpoint using given service state.
func (rbft *rbftImpl[T, Constraint]) generateSignedCheckpoint(state *types.ServiceState, isConfig bool) (*consensus.SignedCheckpoint, error) {
	signedCheckpoint := &consensus.SignedCheckpoint{
		Author: rbft.peerPool.hostname,
	}

	checkpoint := &consensus.Checkpoint{
		Epoch: rbft.epoch,
		ExecuteState: &consensus.Checkpoint_ExecuteState{
			Height: state.MetaState.Height,
			Digest: state.MetaState.Digest,
		},
	}

	if isConfig {
		ledgerStableCheckpoint := rbft.external.GetLastCheckpoint()
		// NOTE! compare current height with ledger stable checkpoint height.
		// if currentHeight is lower than ledger stable checkpoint height, which means
		// current checkpoint is generated in previous epoch, we need to decrease the
		// epoch value.
		if ledgerStableCheckpoint != nil {
			ledgerStableCheckpointHeight := ledgerStableCheckpoint.Height()
			currentHeight := state.MetaState.Height
			if currentHeight <= ledgerStableCheckpointHeight {
				rbft.logger.Infof("Replica %d decrease epoch as current height %d is lower "+
					"than the last stable checkpoint height %d", rbft.peerPool.ID, currentHeight, ledgerStableCheckpointHeight)
				checkpoint.Epoch--
			}
		}
		checkpoint.SetValidatorSet(rbft.external.GetNodeInfos())
		checkpoint.SetVersion(rbft.external.GetAlgorithmVersion())
		rbft.logger.Noticef("Replica %d generate a checkpoint with new validator set: %+v, new version: %s",
			rbft.peerPool.ID, checkpoint.ValidatorSet(), checkpoint.Version())
	}
	signedCheckpoint.Checkpoint = checkpoint

	signature, sErr := rbft.signCheckpoint(checkpoint)
	if sErr != nil {
		rbft.logger.Errorf("Replica %d sign checkpoint error: %s", rbft.peerPool.ID, sErr)
		rbft.stopNamespace()
		return nil, sErr
	}
	signedCheckpoint.Signature = signature

	rbft.logger.Infof("Replica %d generated signed checkpoint with epoch %d, height: %d, "+
		"digest: %s", rbft.peerPool.ID, checkpoint.Epoch, checkpoint.Height(), checkpoint.Digest())

	return signedCheckpoint, nil
}

// signCheckpoint generates a signature of certain checkpoint message.
func (rbft *rbftImpl[T, Constraint]) signCheckpoint(checkpoint *consensus.Checkpoint) ([]byte, error) {
	msg := checkpoint.Hash()
	sig, sErr := rbft.external.Sign(msg)
	if sErr != nil {
		return nil, sErr
	}
	return sig, nil
}

// verifySignedCheckpoint returns whether given signedCheckpoint contains a valid signature.
func (rbft *rbftImpl[T, Constraint]) verifySignedCheckpoint(signedCheckpoint *consensus.SignedCheckpoint) error {
	msg := signedCheckpoint.Checkpoint.Hash()
	return rbft.external.Verify(signedCheckpoint.GetAuthor(), signedCheckpoint.Signature, msg)
}

// syncConfigCheckpoint posts config checkpoint out and wait for its completion synchronously.
func (rbft *rbftImpl[T, Constraint]) syncConfigCheckpoint(checkpointHeight uint64, quorumCheckpoints []*consensus.SignedCheckpoint) bool {
	// waiting for stable checkpoint finish
	rbft.logger.Infof("Replica %d post stable checkpoint event for seqNo %d after "+
		"executed to the height with the same digest", rbft.peerPool.ID, checkpointHeight)

	rbft.external.SendFilterEvent(types.InformTypeFilterStableCheckpoint, quorumCheckpoints)
	rbft.epochMgr.configBatchToCheck = nil

	rbft.logger.Noticef("Replica %d is waiting for stable checkpoint %d finished...", rbft.peerPool.ID, checkpointHeight)
	for {
		select {
		case <-rbft.close:
			rbft.logger.Error("stop waiting stable checkpoint finished because of close")
			return false
		case height, ok := <-rbft.confChan:
			if !ok {
				rbft.logger.Notice("Config Channel Closed")
				return false
			}
			rbft.logger.Debugf("Replica %d received a stable checkpoint finished event at height %d", rbft.peerPool.ID, height)
			if height != checkpointHeight {
				rbft.logger.Noticef("mismatch stable checkpoint finished height: %d, retry", height)
				continue
			}
			return true
		}
	}
}

// syncEpoch tries to sync rbft.Epoch with current latest epoch on ledger and returns
// if epoch has been changed.
// turn into new epoch when epoch has been changed.
func (rbft *rbftImpl[T, Constraint]) syncEpoch() bool {
	lastEpochCheckpoint := rbft.external.GetLastCheckpoint()
	chainEpoch := lastEpochCheckpoint.NextEpoch()
	epochChanged := chainEpoch != rbft.epoch
	if epochChanged {
		rbft.logger.Infof("epoch changed from %d to %d", rbft.epoch, chainEpoch)
		rbft.turnIntoEpoch()
		rbft.moveWatermarks(lastEpochCheckpoint.Height(), true)
	} else {
		rbft.logger.Debugf("Replica %d don't change epoch as epoch %d has not been changed",
			rbft.peerPool.ID, rbft.epoch)
	}

	return epochChanged
}

// signViewChange generates a signature of certain ViewChange message.
func (rbft *rbftImpl[T, Constraint]) signViewChange(vc *consensus.ViewChange) ([]byte, error) {
	hash, hErr := rbft.calculateViewChangeHash(vc)
	if hErr != nil {
		return nil, hErr
	}
	sig, sErr := rbft.external.Sign(hash)
	if sErr != nil {
		rbft.logger.Warningf("Replica %d sign view change failed: %s", rbft.peerPool.ID, sErr)
		rbft.stopNamespace()
		return nil, sErr
	}
	return sig, nil
}

// verifySignedViewChange returns whether given ViewChange contains a valid signature.
func (rbft *rbftImpl[T, Constraint]) verifySignedViewChange(vc *consensus.ViewChange, replicaID uint64) error {
	hash, hErr := rbft.calculateViewChangeHash(vc)
	if hErr != nil {
		return hErr
	}
	host, ok := rbft.peerPool.router[replicaID]
	if !ok {
		return fmt.Errorf("invalid node %d not in router", replicaID)
	}
	return rbft.external.Verify(host, vc.Signature, hash)
}

func (rbft *rbftImpl[T, Constraint]) calculateViewChangeHash(vc *consensus.ViewChange) ([]byte, error) {
	hasher := sha3.NewLegacyKeccak256()
	//nolint
	hasher.Write(vc.GetBasis())
	hash := hasher.Sum(nil)
	return hash, nil
}

// signNewView generates a signature of certain NewView message.
func (rbft *rbftImpl[T, Constraint]) signNewView(nv *consensus.NewView) ([]byte, error) {
	hash, hErr := rbft.calculateNewViewHash(nv)
	if hErr != nil {
		return nil, hErr
	}
	sig, sErr := rbft.external.Sign(hash)
	if sErr != nil {
		rbft.logger.Warningf("Replica %d sign new view failed: %s", rbft.peerPool.ID, sErr)
		rbft.stopNamespace()
		return nil, sErr
	}
	return sig, nil
}

// verifySignedNewView returns whether given NewView contains a valid signature.
func (rbft *rbftImpl[T, Constraint]) verifySignedNewView(nv *consensus.NewView) ([]byte, error) {
	hash, hErr := rbft.calculateNewViewHash(nv)
	if hErr != nil {
		return nil, hErr
	}
	host, ok := rbft.peerPool.router[nv.GetReplicaId()]
	if !ok {
		return nil, fmt.Errorf("invalid node %d not in router", nv.GetReplicaId())
	}
	return hash, rbft.external.Verify(host, nv.Signature, hash)
}

func (rbft *rbftImpl[T, Constraint]) calculateNewViewHash(nv *consensus.NewView) ([]byte, error) {
	signValue := &consensus.NewView{
		ReplicaId: nv.ReplicaId,
		View:      nv.View,
		Xset:      nv.Xset,
	}
	res, mErr := proto.Marshal(signValue)
	if mErr != nil {
		rbft.logger.Warningf("Replica %d marshal new view failed: %s", rbft.peerPool.ID, mErr)
		rbft.stopNamespace()
		return nil, mErr
	}
	hasher := sha3.NewLegacyKeccak256()
	//nolint
	hasher.Write(res)
	hash := hasher.Sum(nil)
	return hash, nil
}
