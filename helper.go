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
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/ultramesh/flato-event/inner/protos"
	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
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

// primaryIndex returns the expected primary index in routing table with the given view v
func (rbft *rbftImpl) primaryIndex(v uint64) uint64 {
	return (v % uint64(rbft.N)) + 1
}

// primaryID returns the expected primary id with the given view v
func (rbft *rbftImpl) primaryID(v uint64) uint64 {
	index := v % uint64(rbft.N)
	if int(index) >= len(rbft.peerPool.router.Peers) {
		rbft.logger.Warningf("Can not find primary hash by view %d", v)
		return 0
	}
	router := rbft.peerPool.router.Peers[index]
	return router.Id
}

// isPrimary returns if current node is primary or not
func (rbft *rbftImpl) isPrimary(id uint64) bool {
	// for node with no id in routers(such as a new node), directly return false.
	if id > uint64(len(rbft.peerPool.router.Peers)) {
		return false
	}
	return rbft.primaryID(rbft.view) == id
}

// InW returns if the given seqNo is higher than h or not
func (rbft *rbftImpl) inW(n uint64) bool {
	return n > rbft.h
}

// InV returns if the given view equals the current view or not
func (rbft *rbftImpl) inV(v uint64) bool {
	return rbft.view == v
}

// InWV firstly checks if the given view is inV then checks if the given seqNo n is inW
func (rbft *rbftImpl) inWV(v uint64, n uint64) bool {
	return rbft.inV(v) && rbft.inW(n)
}

// sendInW used in maybeSendPrePrepare checks the given seqNo is between low
// watermark and high watermark or not.
func (rbft *rbftImpl) sendInW(n uint64) bool {
	return n > rbft.h && n <= rbft.h+rbft.L
}

// getAddNV calculates the new N and view after add a new node
func (rbft *rbftImpl) getAddNV() (n int64, v uint64) {
	n = int64(rbft.N) + 1
	if rbft.view < uint64(rbft.N) {
		v = rbft.view + uint64(n)
	} else {
		v = rbft.view/uint64(rbft.N)*uint64(rbft.N+1) + rbft.view%uint64(rbft.N)
	}

	return
}

// getDelNV calculates the new N and view after delete a new node
func (rbft *rbftImpl) getDelNV(del uint64) (n int64, v uint64) {
	n = int64(rbft.N) - 1

	rbft.logger.Debugf("Before update, N: %d, view: %d, delID: %d", rbft.N, rbft.view, del)

	// guarantee seed larger than view
	seed := uint64(rbft.N-1) * (rbft.view/uint64(rbft.N-1) + uint64(1))
	if rbft.primaryIndex(rbft.view) > del {
		v = rbft.view%uint64(rbft.N) - 1 + seed
	} else {
		v = rbft.view%uint64(rbft.N) + seed
	}
	rbft.logger.Debugf("After update, N: %d, view: %d", n, v)

	return
}

// cleanAllBatchAndCert cleans all outstandingReqBatches and committedCert
func (rbft *rbftImpl) cleanOutstandingAndCert() {
	rbft.storeMgr.outstandingReqBatches = make(map[string]*pb.RequestBatch)
	rbft.storeMgr.committedCert = make(map[msgID]string)
}

// When N=3F+1, this should be 2F+1 (N-F)
// More generally, we need every two common case quorum of size X to intersect in at least F+1
// hence 2X>=N+F+1
func (rbft *rbftImpl) commonCaseQuorum() int {
	return int(math.Ceil(float64(rbft.N+rbft.f+1) / float64(2)))
}

// oneCorrectQuorum returns the number of replicas in which correct numbers must be bigger than incorrect number
func (rbft *rbftImpl) allCorrectReplicasQuorum() int {
	return rbft.N - rbft.f
}

// oneCorrectQuorum returns the number of replicas in which there must exist at least one correct replica
func (rbft *rbftImpl) oneCorrectQuorum() int {
	return rbft.f + 1
}

// =============================================================================
// pre-prepare/prepare/commit check helper
// =============================================================================

// prePrepared returns if there existed a pre-prepare message in certStore with the given digest,view,seqNo
func (rbft *rbftImpl) prePrepared(digest string, v uint64, n uint64) bool {
	// TODO(DH): we need to ensure that we actually have the request batch.
	cert := rbft.storeMgr.certStore[msgID{v, n, digest}]

	if cert != nil {
		p := cert.prePrepare
		if p != nil && p.View == v && p.SequenceNumber == n && p.BatchDigest == digest {
			return true
		}
	}

	rbft.logger.Debugf("Replica %d does not have view=%d/seqNo=%d prePrepared", rbft.no, v, n)

	return false
}

// prepared firstly checks if the cert with the given msgID has been prePrepared,
// then checks if this node has collected enough prepare messages for the cert with given msgID
func (rbft *rbftImpl) prepared(digest string, v uint64, n uint64) bool {

	if !rbft.prePrepared(digest, v, n) {
		return false
	}

	cert := rbft.storeMgr.certStore[msgID{v, n, digest}]

	prepCount := len(cert.prepare)

	rbft.logger.Debugf("Replica %d prepare count for view=%d/seqNo=%d is %d",
		rbft.no, v, n, prepCount)

	return prepCount >= rbft.commonCaseQuorum()-1
}

// committed firstly checks if the cert with the given msgID has been prepared,
// then checks if this node has collected enough commit messages for the cert with given msgID
func (rbft *rbftImpl) committed(digest string, v uint64, n uint64) bool {

	if !rbft.prepared(digest, v, n) {
		return false
	}

	cert := rbft.storeMgr.certStore[msgID{v, n, digest}]

	cmtCount := len(cert.commit)

	rbft.logger.Debugf("Replica %d commit count for view=%d/seqNo=%d is %d",
		rbft.no, v, n, cmtCount)

	return cmtCount >= rbft.commonCaseQuorum()
}

// =============================================================================
// helper functions for transfer message
// =============================================================================

// broadcastReqSet helps broadcast requestSet to others.
func (rbft *rbftImpl) broadcastReqSet(set *pb.RequestSet) {
	payload, err := proto.Marshal(set)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_TRANSACTION_SET Marshal Error: %s", err)
		return
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_REQUEST_SET,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
}

// =============================================================================
// helper functions for timer
// =============================================================================

// startTimerIfOutstandingRequests soft starts a new view timer if there exists some outstanding request batches,
// else reset the null request timer
func (rbft *rbftImpl) startTimerIfOutstandingRequests() {
	if rbft.in(SkipInProgress) || rbft.exec.currentExec != nil {
		// Do not start the view change timer if we are executing or state transferring, these take arbitrarily long amounts of time
		return
	}

	if len(rbft.storeMgr.outstandingReqBatches) > 0 {
		getOutstandingDigests := func() []string {
			var digests []string
			for digest := range rbft.storeMgr.outstandingReqBatches {
				digests = append(digests, digest)
			}
			return digests
		}()
		rbft.softStartNewViewTimer(rbft.timerMgr.getTimeoutValue(requestTimer), fmt.Sprintf("outstanding request "+
			"batches num=%v, batches: %v", len(getOutstandingDigests), getOutstandingDigests), false)
	} else if rbft.timerMgr.getTimeoutValue(nullRequestTimer) > 0 {
		rbft.nullReqTimerReset()
	}
}

// nullReqTimerReset reset the null request timer with a certain timeout, for different replica, null request timeout is
// different:
// 1. for primary, null request timeout is the timeout written in the config
// 2. for non-primary, null request timeout =3*(timeout written in the config)+request timeout
func (rbft *rbftImpl) nullReqTimerReset() {
	timeout := rbft.timerMgr.getTimeoutValue(nullRequestTimer)
	if !rbft.isPrimary(rbft.peerPool.localID) {
		// we're waiting for the primary to deliver a null request - give it a bit more time
		timeout = 3*timeout + rbft.timerMgr.getTimeoutValue(requestTimer)
	}

	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNullRequestTimerEvent,
	}

	rbft.timerMgr.startTimerWithNewTT(nullRequestTimer, timeout, event)
}

// stopFirstRequestTimer stops the first request timer event if current node is not primary
func (rbft *rbftImpl) stopFirstRequestTimer() {
	if !rbft.isPrimary(rbft.peerPool.localID) {
		rbft.timerMgr.stopTimer(firstRequestTimer)
	}
}

// =============================================================================
// helper functions for check the validity of consensus messages
// =============================================================================
// isPrePrepareLegal firstly checks if current status can receive pre-prepare or not, then checks pre-prepare message
// itself is legal or not
func (rbft *rbftImpl) isPrePrepareLegal(preprep *pb.PrePrepare) bool {
	if rbft.in(InRecovery) {
		rbft.logger.Debugf("Replica %d try to receive prePrepare, but it's in recovery", rbft.no)
		return false
	}

	if rbft.in(InViewChange) {
		rbft.logger.Debugf("Replica %d try to receive prePrepare, but it's in viewChange", rbft.no)
		return false
	}

	if rbft.in(InUpdatingN) {
		rbft.logger.Debugf("Replica %d try to receive prePrepare, but it's in updatingN", rbft.no)
		return false
	}

	// replica rejects prePrepare sent from non-primary.
	if !rbft.isPrimary(preprep.ReplicaId) {
		from := rbft.peerPool.noMap[preprep.ReplicaId]
		rbft.logger.Warningf("Replica %d received prePrepare from non-primary: got %d, should be %d",
			rbft.no, from, rbft.primaryIndex(rbft.view))
		return false
	}

	// primary reject prePrepare sent from itself.
	if rbft.isPrimary(rbft.peerPool.localID) {
		rbft.logger.Warningf("Primary %d reject prePrepare sent from itself", rbft.no)
		return false
	}

	if !rbft.inWV(preprep.View, preprep.SequenceNumber) {
		if preprep.SequenceNumber != rbft.h && !rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d received prePrepare with a different view or sequence "+
				"number outside watermarks: prePrep.View %d, expected.View %d, seqNo %d, low water mark %d",
				rbft.no, preprep.View, rbft.view, preprep.SequenceNumber, rbft.h)
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d received prePrepare with a different view or sequence "+
				"number outside watermarks: preprep.View %d, expected.View %d, seqNo %d, low water mark %d",
				rbft.no, preprep.View, rbft.view, preprep.SequenceNumber, rbft.h)
		}
		return false
	}

	if preprep.SequenceNumber <= rbft.exec.lastExec {
		rbft.logger.Debugf("Replica %d received a prePrepare with seqNo %d lower than lastExec %d, ignore it...", rbft.no, preprep.SequenceNumber, rbft.exec.lastExec)
		return false
	}

	return true
}

// isPrepareLegal firstly checks if current status can receive prepare or not, then checks prepare message itself is
// legal or not
func (rbft *rbftImpl) isPrepareLegal(prep *pb.Prepare) bool {

	// if we are not in recovery, but receive prepare from primary, which means primary behavior as a byzantine,
	// we don't send viewchange here, because in this case, replicas will eventually find primary abnormal in other cases.
	if rbft.isPrimary(prep.ReplicaId) {
		rbft.logger.Warningf("Replica %d received prepare from primary, ignore it", rbft.no)
		return false
	}

	if !rbft.inWV(prep.View, prep.SequenceNumber) {
		from := rbft.peerPool.noMap[prep.ReplicaId]
		if prep.SequenceNumber != rbft.h && !rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d ignore prepare from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.no, from, prep.View, prep.SequenceNumber, rbft.view, rbft.h)
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d ignore prepare from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d",
				rbft.no, from, prep.View, prep.SequenceNumber, rbft.view, rbft.h)
		}

		return false
	}
	return true
}

// isCommitLegal firstly checks if current status can receive commit or not, then checks commit message itself is legal
// or not
func (rbft *rbftImpl) isCommitLegal(commit *pb.Commit) bool {

	if !rbft.inWV(commit.View, commit.SequenceNumber) {
		sender := rbft.peerPool.noMap[commit.ReplicaId]
		if commit.SequenceNumber != rbft.h && !rbft.in(SkipInProgress) {
			rbft.logger.Warningf("Replica %d ignore commit from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d", rbft.no, sender, commit.View, commit.SequenceNumber, rbft.view, rbft.h)
		} else {
			// This is perfectly normal
			rbft.logger.Debugf("Replica %d ignore commit from replica %d for view=%d/seqNo=%d: not inWv, in view: %d, h: %d", rbft.no, sender, commit.View, commit.SequenceNumber, rbft.view, rbft.h)
		}
		return false
	}
	return true
}

// compareCheckpointWithWeakSet first checks the legality of this checkpoint, which seqNo
// must between [h, H] and we haven't received a same checkpoint message, then find the
// weak set with more than f + 1 members who have sent a checkpoint with the same seqNo
// and ID, if there exists more than one weak sets, we'll never find a stable cert for this
// seqNo, else checks if self's generated checkpoint has the same ID with the given one,
// if not, directly start state update to recover to a correct state.
func (rbft *rbftImpl) compareCheckpointWithWeakSet(chkpt *pb.Checkpoint) (bool, int) {
	// if checkpoint height is lower than current low watermark, ignore it as we have reached a higher h,
	// else, continue to find f+1 checkpoint messages with the same seqNo and ID
	if !rbft.inW(chkpt.SequenceNumber) {
		if chkpt.SequenceNumber != rbft.h && !rbft.in(SkipInProgress) {
			// It is perfectly normal that we receive checkpoints for the watermark we just raised, as we raise it after 2f+1, leaving f replies left
			rbft.logger.Warningf("Checkpoint sequence number outside watermarks: seqNo %d, low water mark %d", chkpt.SequenceNumber, rbft.h)
		} else {
			rbft.logger.Debugf("Checkpoint sequence number outside watermarks: seqNo %d, low water mark %d", chkpt.SequenceNumber, rbft.h)
		}
		return false, 0
	}

	if rbft.storeMgr.checkpointStore[*chkpt] {
		rbft.logger.Warningf("Replica %d ignore duplicate checkpoint from replica %d, seqNo=%d", rbft.no, chkpt.ReplicaId, chkpt.SequenceNumber)
		return false, 0
	}
	rbft.storeMgr.checkpointStore[*chkpt] = true

	// track how many different checkpoint values we have for the seqNo.
	diffValues := make(map[string][]uint64)
	// track how many "correct"(more than f + 1) checkpoint values we have for the seqNo.
	var correctValues []string
	// track members with the same checkpoint ID and seqNo which may be used in state update
	// when self's checkpoint ID is not the same as a weak cert's checkpoint ID.
	var checkpointMembers []replicaInfo
	// track totally matching checkpoints.
	matching := 0
	for cp := range rbft.storeMgr.checkpointStore {
		if cp.SequenceNumber != chkpt.SequenceNumber {
			continue
		}

		if cp.Digest == chkpt.Digest {
			// we shouldn't put self in target
			if cp.ReplicaId != rbft.peerPool.localID {
				checkpointMembers = append(checkpointMembers, replicaInfo{
					replicaID: cp.ReplicaId,
				})
			}
			matching++
		}

		if _, ok := diffValues[cp.Digest]; !ok {
			diffValues[cp.Digest] = []uint64{cp.ReplicaId}
		} else {
			diffValues[cp.Digest] = append(diffValues[cp.Digest], cp.ReplicaId)
		}

		// if current network contains more than f + 1 checkpoints with the same seqNo
		// but different ID, we'll never be able to get a stable cert for this seqNo.
		if len(diffValues) > rbft.f+1 {
			rbft.logger.Criticalf("Replica %d cannot find stable checkpoint with seqNo %d"+
				"(%d different values observed already).", rbft.no, chkpt.SequenceNumber, len(diffValues))
			rbft.on(Pending)
			rbft.setAbNormal()
			return false, 0
		}

		// record all correct checkpoint(weak cert) values.
		if len(diffValues[cp.Digest]) == rbft.f+1 {
			correctValues = append(correctValues, cp.Digest)
		}
	}

	if len(correctValues) == 0 {
		rbft.logger.Debugf("Replica %d hasn't got a weak cert for checkpoint %d", rbft.no, chkpt.SequenceNumber)
		return true, matching
	}

	// if we encounter more than one correct weak set, we will never recover to a stable
	// consensus state.
	if len(correctValues) > 1 {
		rbft.logger.Criticalf("Replica %d finds several weak certs for checkpoint %d, values: %v", rbft.no, chkpt.SequenceNumber, correctValues)
		rbft.on(Pending)
		rbft.setAbNormal()
		return false, 0
	}

	// if we can only find one weak cert with the same seqNo and ID, our generated checkpoint(if
	// existed) must have the same ID with that one.
	correctID := correctValues[0]
	selfID, ok := rbft.storeMgr.chkpts[chkpt.SequenceNumber]
	// if self's checkpoint with the same seqNo has a distinguished ID with a weak certs'
	// checkpoint ID, we should trigger state update right now to recover self block state.
	if ok && selfID != correctID {
		rbft.logger.Criticalf("Replica %d generated a checkpoint of %s, but a weak set of the network agrees on %s.",
			rbft.no, selfID, correctID)

		target := &stateUpdateTarget{
			targetMessage: targetMessage{
				height: chkpt.SequenceNumber,
				digest: correctID,
			},
			replicas: checkpointMembers,
		}
		rbft.updateHighStateTarget(target)
		rbft.tryStateTransfer(target)
		return false, 0
	}

	return true, matching
}

// compareWholeStates compares whole networks' current status during recovery or sync state
// Those status including :
// 1. N: current consensus nodes number
// 2. view: current view of bft network
// 3. routerHash: current consensus network's router info which contains all nodes' hostname et al...
// 4. appliedIndex(only compared in sync state): current latest blockChain height
// 5. digest(only compared in sync state): current latest blockChain hash
func (rbft *rbftImpl) compareWholeStates(states wholeStates) consensusEvent {
	// track all replica hash with same state used to update routing table if needed
	sameRespCount := make(map[nodeState][]uint64)
	// track all replica info with same state used to state update if needed
	replicaRecord := make(map[nodeState][]replicaInfo)

	// check if we can find quorum nodeState who have the same n and view, routers, if we can find, which means
	// quorum nodes agree to a N and view, save to quorumRsp, set canFind to true and update N, view if needed
	var quorumResp nodeState
	canFind := false

	var quorumInfo []byte
	// find the quorum nodeState
	for key, state := range states {
		sameRespCount[state] = append(sameRespCount[state], key)
		replicaRecord[state] = append(replicaRecord[state], replicaInfo{replicaID: key})

		// if quorum agree with a same N,view,routers, check if we need to update routing table first.
		if len(sameRespCount[state]) >= rbft.commonCaseQuorum() {
			quorumResp = state
			canFind = true
			// if add/delete node happened in downtime, we may find self's routing table is
			// not the same as quorum nodes, we need to update routing table and optionally
			// connect to other nodes.
			quorumInfo = hex2Bytes(state.routerInfo)
			selfInfo := rbft.peerPool.serializeRouterInfo()
			if !rbft.in(isNewNode) && !bytes.Equal(quorumInfo, selfInfo) {
				rbft.logger.Warningf("Replica %d finds quorum routing table which is different "+
					"from self's, pending updating routing table", rbft.no)
				rbft.peerPool.updateRouter(quorumInfo)
			}
			break
		}
	}

	// we can find the quorum nodeState with the same N and view, judge if the response.view equals to the
	// current view, if so, just update N and view, else update N, view and then re-constructs certStore
	if canFind {
		// update view if needed
		if rbft.view != quorumResp.view {
			rbft.view = quorumResp.view
		}

		// update N and f if needed
		if rbft.N != int(quorumResp.n) {
			rbft.N = int(quorumResp.n)
			rbft.f = (rbft.N - 1) / 3
		}

		rbft.logger.Infof("Replica %d persist view=%d/N=%d after found quorum same response.", rbft.no, rbft.view, rbft.N)
		// always persist view and N to consensus database no matter we need to update view or not.
		rbft.persistView(rbft.view)
		rbft.persistN(rbft.N)

		if rbft.in(InSyncState) {
			rbft.timerMgr.stopTimer(syncStateRspTimer)
			rbft.off(InSyncState)
			selfState, ok := states[rbft.peerPool.localID]
			if !ok {
				rbft.logger.Warningf("Replica %d cannot find self state in wholeStates", rbft.no)
				return nil
			}
			selfHeight := selfState.appliedIndex
			quorumHeight := quorumResp.appliedIndex
			selfHash := selfState.digest
			quorumDigest := quorumResp.digest
			if selfHeight == quorumHeight && selfHash != quorumDigest {
				rbft.logger.Errorf("Replica %d finds quorum same block state whose hash is different from self,"+
					"in height: %d, selfHash: %s, quorumDigest: %s, need to state update", rbft.no, quorumHeight, selfHash, quorumDigest)

				target := &stateUpdateTarget{
					targetMessage: targetMessage{height: quorumHeight, digest: quorumDigest},
					replicas:      replicaRecord[quorumResp],
				}
				rbft.updateHighStateTarget(target)
				rbft.tryStateTransfer(target)
				return nil
			}
			if selfHeight != quorumHeight {
				rbft.logger.Noticef("Replica %d finds quorum same block state which is different from self,"+
					"self height: %d, quorum height: %d, selfHash: %s, quorumDigest: %s", rbft.no, selfHeight, quorumHeight, selfHash, quorumDigest)

				if rbft.isPrimary(rbft.peerPool.localID) {
					rbft.logger.Warningf("Primary %d finds itself not sync with quorum replicas, sending viewChange", rbft.no)
					return rbft.sendViewChange()
				}
				rbft.logger.Infof("Replica %d finds itself not sync with quorum replicas, try to recovery", rbft.no)
				return rbft.initRecovery()
			}
			rbft.logger.Infof("======== Replica %d finished sync state for height: %d, hash: %s", rbft.no, selfHeight, selfHash)
			return nil
		}

		if rbft.in(InRecovery) {
			if rbft.in(isNewNode) {
				quorumRouter := rbft.peerPool.unSerializeRouterInfo(quorumInfo)
				// find if current node has been included into quorum node's routers.
				included := false
				for _, peer := range quorumRouter.Peers {
					if peer.Id == rbft.peerPool.localID {
						included = true
					}
				}

				rbft.peerPool.updateRouter(quorumInfo)
				// If new node find itself needs to updateN which must be caused by timeout
				// event in adding node, check if quorum nodes have agree admittance of myself,
				// if not, resend readyForN request after recovery.
				if included {
					rbft.logger.Noticef("New node %d finds itself has been accepted by quorum nodes with "+
						"N=%d/view=%d, start normal consensus process.", rbft.no, rbft.N, rbft.view)

					rbft.off(isNewNode)
					rbft.logger.Noticef("======== Replica %d finished updateN, primary=%d, n=%d/f=%d/view=%d/h=%d",
						rbft.no, rbft.primaryIndex(rbft.view), rbft.N, rbft.f, rbft.view, rbft.h)
				}
			} else {
				// if current node finds itself become primary, but quorum other replicas
				// are in normal status, directly send viewChange as we don't want to
				// resend prePrepares after sync view.
				if rbft.isPrimary(rbft.peerPool.localID) {
					rbft.logger.Warningf("Replica %d become primary after sync view, sending viewChange", rbft.no)
					rbft.timerMgr.stopTimer(recoveryRestartTimer)
					rbft.off(InRecovery)
					rbft.sendViewChange()
					return nil
				}
			}

			return rbft.resetStateForRecovery()
		}
	}

	return nil
}

// calcQSet selects Pre-prepares which satisfy the following conditions
// 1. Pre-prepares in previous qlist
// 2. Pre-prepares from certStore which is preprepared and its view <= its idx.v or not in qlist
func (rbft *rbftImpl) calcQSet() map[qidx]*pb.Vc_PQ {

	qset := make(map[qidx]*pb.Vc_PQ)

	for n, q := range rbft.vcMgr.qlist {
		qset[n] = q
	}

	for idx := range rbft.storeMgr.certStore {

		if !rbft.prePrepared(idx.d, idx.v, idx.n) {
			continue
		}

		qi := qidx{idx.d, idx.n}
		if q, ok := qset[qi]; ok && q.View > idx.v {
			continue
		}

		qset[qi] = &pb.Vc_PQ{
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
func (rbft *rbftImpl) calcPSet() map[uint64]*pb.Vc_PQ {

	pset := make(map[uint64]*pb.Vc_PQ)

	for n, p := range rbft.vcMgr.plist {
		pset[n] = p
	}

	for idx := range rbft.storeMgr.certStore {

		if !rbft.prepared(idx.d, idx.v, idx.n) {
			continue
		}

		if p, ok := pset[idx.n]; ok && p.View > idx.v {
			continue
		}

		pset[idx.n] = &pb.Vc_PQ{
			SequenceNumber: idx.n,
			BatchDigest:    idx.d,
			View:           idx.v,
		}
	}

	return pset
}

// getVcBasis helps re-calculate the plist and qlist then construct a vcBasis
// at teh same time, useless cert with lower .
func (rbft *rbftImpl) getVcBasis() *pb.VcBasis {
	basis := &pb.VcBasis{
		View:      rbft.view,
		H:         rbft.h,
		ReplicaId: rbft.peerPool.localID,
	}

	// clear qList and pList from DB as we will construct new QPList next.
	rbft.persistDelQPList()

	rbft.vcMgr.plist = rbft.calcPSet()
	rbft.vcMgr.qlist = rbft.calcQSet()

	// Note. before vc/recovery/updateN, we need to persist QPList to ensure we can restore committed entries
	// after above abnormal situations as we will delete all PQCSet when we enter abnormal, after finish
	// vc/recovery/updateN we will re-broadcast and persist PQCSet which is enough to ensure continuity of
	// committed entries in next vc/recovery/updateN. However, QPList cannot be deleted immediately after
	// finish vc/recovery/updateN as we may loss some committed entries after crash down in normal status.
	// So:
	// 1. during normal status, we have: QPSet with pre-prepare certs and prepare certs and QPList generated in
	// previous abnormal status which is used to catch some useful committed entries after system crash down.
	// 2. during abnormal status, we have no QPSet but we have QPList generated in current abnormal status.
	rbft.persistPList(rbft.vcMgr.plist)
	rbft.persistQList(rbft.vcMgr.qlist)

	for idx := range rbft.storeMgr.certStore {
		if idx.v < rbft.view {
			rbft.logger.Debugf("Replica %d clear cert with view=%d/seqNo=%d/digest=%s when construct VcBasis",
				rbft.no, idx.v, idx.n, idx.d)
			delete(rbft.storeMgr.certStore, idx)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
		}
	}

	basis.Cset, basis.Pset, basis.Qset = rbft.gatherPQC()

	return basis
}

// gatherPQC just gather all checkpoints, p entries and q entries.
func (rbft *rbftImpl) gatherPQC() (cset []*pb.Vc_C, pset []*pb.Vc_PQ, qset []*pb.Vc_PQ) {
	// Gather all the checkpoints
	rbft.logger.Debugf("Replica %d gather CSet:", rbft.no)
	for n, id := range rbft.storeMgr.chkpts {
		cset = append(cset, &pb.Vc_C{
			SequenceNumber: n,
			Digest:         id,
		})
		rbft.logger.Debugf("seqNo: %d, ID: %s", n, id)
	}
	// Gather all the p entries
	rbft.logger.Debugf("Replica %d gather PSet:", rbft.no)
	for _, p := range rbft.vcMgr.plist {
		if p.SequenceNumber < rbft.h {
			rbft.logger.Errorf("Replica %d should not have anything in our pset less than h, found %+v", rbft.no, p)
			continue
		}
		pset = append(pset, p)
		rbft.logger.Debugf("seqNo: %d, view: %d, digest: %s", p.SequenceNumber, p.View, p.BatchDigest)
	}

	// Gather all the q entries
	rbft.logger.Debugf("Replica %d gather QSet:", rbft.no)
	for _, q := range rbft.vcMgr.qlist {
		if q.SequenceNumber < rbft.h {
			rbft.logger.Errorf("Replica %d should not have anything in our qset less than h, found %+v", rbft.no, q)
			continue
		}
		qset = append(qset, q)
		rbft.logger.Debugf("seqNo: %d, view: %d, digest: %s", q.SequenceNumber, q.View, q.BatchDigest)
	}

	return
}

// byte2Hex returns the hex encode result of data
func byte2Hex(data []byte) string {
	str := hex.EncodeToString(data)
	return str
}

func hex2Bytes(str string) []byte {
	if len(str) >= 2 && str[0:2] == "0x" {
		str = str[2:]
	}
	h, _ := hex.DecodeString(str)

	return h
}

func requestHash(tx *protos.Transaction) string {
	return protos.GetHash(tx).Hex()
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
