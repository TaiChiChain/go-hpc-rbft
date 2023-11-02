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

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
)

/**
This file contains recovery related issues
*/

// recoveryManager manages recovery related events
type recoveryManager struct {
	syncRspStore map[uint64]*consensus.SyncStateResponse // store sync state response

	logger common.Logger
}

// newRecoveryMgr news an instance of recoveryManager
func newRecoveryMgr(c Config) *recoveryManager {
	rm := &recoveryManager{
		syncRspStore: make(map[uint64]*consensus.SyncStateResponse),
		logger:       c.Logger,
	}
	return rm
}

// dispatchRecoveryMsg dispatches recovery service messages using service type
func (rbft *rbftImpl[T, Constraint]) dispatchRecoveryMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *consensus.SyncState:
		return rbft.recvSyncState(et)
	case *consensus.SyncStateResponse:
		return rbft.recvSyncStateResponse(et, false)
	case *consensus.FetchPQCRequest:
		return rbft.recvFetchPQCRequest(et)
	case *consensus.FetchPQCResponse:
		return rbft.recvFetchPQCResponse(et)
	}
	return nil
}

// fetchRecoveryPQC always fetches PQC info after recovery done to fetch PQC info after target checkpoint
func (rbft *rbftImpl[T, Constraint]) fetchRecoveryPQC() consensusEvent {
	rbft.logger.Debugf("Replica %d fetch PQC, h=%d", rbft.chainConfig.SelfID, rbft.chainConfig.H)

	fetch := &consensus.FetchPQCRequest{
		H:         rbft.chainConfig.H,
		ReplicaId: rbft.chainConfig.SelfID,
	}
	payload, err := fetch.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_FetchPQCRequest marshal error")
		return nil
	}
	conMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_FETCH_PQC_REQUEST,
		Payload: payload,
	}

	rbft.peerMgr.broadcast(context.TODO(), conMsg)

	return nil
}

// recvFetchPQCRequest returns all PQC info we have sent before to the sender
func (rbft *rbftImpl[T, Constraint]) recvFetchPQCRequest(fetch *consensus.FetchPQCRequest) consensusEvent {
	if !rbft.chainConfig.isValidator() {
		rbft.logger.Debugf("Replica %d is not validator, not process fetchPQCRequest",
			rbft.chainConfig.SelfID)
		return nil
	}
	rbft.logger.Debugf("Replica %d received fetchPQCRequest from replica %d, remote h=%d", rbft.chainConfig.SelfID, fetch.ReplicaId, fetch.H)

	remoteH := fetch.H
	if remoteH >= rbft.chainConfig.H+rbft.chainConfig.L {
		rbft.logger.Warningf("Replica %d received fetchPQCRequest, but its rbft.h ≥ highwatermark", rbft.chainConfig.SelfID)
		return nil
	}

	var prePres []*consensus.PrePrepare
	var pres []*consensus.Prepare
	var cmts []*consensus.Commit

	findPQC := func(certStore map[msgID]*msgCert) {
		// replica just send all PQC info we had sent before
		for idx, cert := range certStore {
			// send all PQC that n > remoteH in current view, help remote node to advance.
			if idx.n > remoteH && idx.v == rbft.chainConfig.View {
				rbft.logger.Debugf("Replica %d find PQC response cert: %s, prePrepare==nil: %v, len(prepare): %d, len(commit): %d",
					rbft.chainConfig.SelfID,
					idx.ID(),
					cert.prePrepare == nil,
					len(cert.prepare),
					len(cert.commit),
				)

				// only response with messages we have sent.
				if cert.prePrepare == nil {
					rbft.logger.Warningf("Replica %d in finds nil prePrepare for view=%d/seqNo=%d",
						rbft.chainConfig.SelfID, idx.v, idx.n)
				} else if cert.prePrepare.ReplicaId == rbft.chainConfig.SelfID {
					rbft.logger.Debugf("Replica %d find PQC response, found matched prePre: %s", rbft.chainConfig.SelfID, cert.prePrepare.ID())
					prePres = append(prePres, cert.prePrepare)
				}
				for _, pre := range cert.prepare {
					if pre.ReplicaId == rbft.chainConfig.SelfID {
						rbft.logger.Debugf("Replica %d find PQC response, found matched prepare: %s", rbft.chainConfig.SelfID, pre.ID())
						prepare := pre
						pres = append(pres, prepare)
					}
				}
				for _, cmt := range cert.commit {
					if cmt.ReplicaId == rbft.chainConfig.SelfID {
						rbft.logger.Debugf("Replica %d find PQC response, found matched commit: %s", rbft.chainConfig.SelfID, cmt.ID())
						commit := cmt
						cmts = append(cmts, commit)
					}
				}
			}
		}
	}
	rbft.logger.Debugf("Replica %d find PQC response cert from certStore, size:%d", rbft.chainConfig.SelfID, len(rbft.storeMgr.certStore))
	findPQC(rbft.storeMgr.certStore)
	rbft.logger.Debugf("Replica %d find PQC response cert from committedCertCache, size:%d", rbft.chainConfig.SelfID, len(rbft.storeMgr.committedCertCache))
	findPQC(rbft.storeMgr.committedCertCache)

	pqcResponse := &consensus.FetchPQCResponse{
		ReplicaId: rbft.chainConfig.SelfID,
		PrepreSet: prePres,
		PreSet:    pres,
		CmtSet:    cmts,
	}

	payload, err := pqcResponse.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_FetchPQCResponse marshal error: %v", err)
		return nil
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_FETCH_PQC_RESPONSE,
		Payload: payload,
	}
	rbft.peerMgr.unicast(context.TODO(), consensusMsg, fetch.ReplicaId)

	var view, sequenceNumber uint64
	var batchDigest string
	if len(pqcResponse.PrepreSet) != 0 {
		view = pqcResponse.PrepreSet[0].View
		sequenceNumber = pqcResponse.PrepreSet[0].SequenceNumber
		batchDigest = pqcResponse.PrepreSet[0].BatchDigest
	}
	rbft.logger.Debugf("Replica %d send PQC response to %d, detailed: {view:%d,seq:%d,digest:%s}, len(prePrepare): %d, len(prepare): %d, len(commit): %d", rbft.chainConfig.SelfID,
		fetch.ReplicaId, view, sequenceNumber, batchDigest, len(pqcResponse.PrepreSet), len(pqcResponse.PreSet), len(pqcResponse.CmtSet))

	return nil
}

// recvFetchPQCResponse re-processes all the PQC received from others
func (rbft *rbftImpl[T, Constraint]) recvFetchPQCResponse(PQCInfo *consensus.FetchPQCResponse) consensusEvent {
	var view, sequenceNumber uint64
	var batchDigest string
	if len(PQCInfo.PrepreSet) != 0 {
		view = PQCInfo.PrepreSet[0].View
		sequenceNumber = PQCInfo.PrepreSet[0].SequenceNumber
		batchDigest = PQCInfo.PrepreSet[0].BatchDigest
	}
	rbft.logger.Debugf("Replica %d received fetchPQCResponse from replica %d, return_pqc {view:%d,seq:%d,digest:%s}",
		rbft.chainConfig.SelfID, PQCInfo.ReplicaId, view, sequenceNumber, batchDigest)

	// post all the PQC
	if !rbft.isPrimary(rbft.chainConfig.SelfID) {
		for _, preprep := range PQCInfo.GetPrepreSet() {
			_ = rbft.recvPrePrepare(context.TODO(), preprep)
		}
	}
	for _, prep := range PQCInfo.GetPreSet() {
		_ = rbft.recvPrepare(context.TODO(), prep)
	}
	for _, cmt := range PQCInfo.GetCmtSet() {
		_ = rbft.recvCommit(context.TODO(), cmt)
	}

	return nil
}

// when we are in abnormal or there are some requests in process, we don't need to sync state,
// we only need to sync state when primary is sending null request which means system is in
// normal status and there are no requests in process.
func (rbft *rbftImpl[T, Constraint]) trySyncState() {
	if !rbft.in(NeedSyncState) {
		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d not try to sync state as we are in abnormal now", rbft.chainConfig.SelfID)
			return
		}
		rbft.logger.Infof("Replica %d need to start sync state progress after %v", rbft.chainConfig.SelfID, rbft.timerMgr.getTimeoutValue(syncStateRestartTimer))
		rbft.on(NeedSyncState)

		event := &LocalEvent{
			Service:   RecoveryService,
			EventType: RecoverySyncStateRestartTimerEvent,
		}

		// start sync state restart timer to cycle sync state while there are no new requests.
		rbft.timerMgr.startTimer(syncStateRestartTimer, event)
	}
}

// initSyncState prepares to sync state.
// if we are in syncState, which means last syncState progress hasn't finished, reject a new syncState request
func (rbft *rbftImpl[T, Constraint]) initSyncState() consensusEvent {
	if rbft.in(InSyncState) {
		rbft.logger.Warningf("Replica %d try to send syncState, but it's already in sync state", rbft.chainConfig.SelfID)
		return nil
	}

	rbft.on(InSyncState)

	rbft.logger.Debugf("Replica %d now init sync state", rbft.chainConfig.SelfID)

	event := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRspTimerEvent,
	}

	// start sync state response timer to wait for quorum response, if we cannot receive
	// enough response during this timeout, don't restart sync state as we will restart
	// sync state after syncStateRestartTimer expired.
	rbft.timerMgr.startTimer(syncStateRspTimer, event)

	rbft.recoveryMgr.syncRspStore = make(map[uint64]*consensus.SyncStateResponse)

	// broadcast sync state message to others.
	syncStateMsg := &consensus.SyncState{
		ReplicaId: rbft.chainConfig.SelfID,
	}
	payload, err := syncStateMsg.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_SYNC_STATE marshal error: %v", err)
		return nil
	}
	msg := &consensus.ConsensusMessage{
		Type:    consensus.Type_SYNC_STATE,
		Payload: payload,
	}
	rbft.peerMgr.broadcast(context.TODO(), msg)

	// post the sync state response message event to myself
	state := rbft.node.getCurrentState()
	if state == nil {
		rbft.logger.Warningf("Replica %d has a nil node state", rbft.chainConfig.SelfID)
		return nil
	}

	signedCheckpoint, sErr := rbft.generateSignedCheckpoint(state, isConfigBatch(state.MetaState.Height, rbft.chainConfig.EpochInfo))
	if sErr != nil {
		rbft.logger.Errorf("Replica %d generate checkpoint error: %s", rbft.chainConfig.SelfID, sErr)
		rbft.stopNamespace()
		return nil
	}
	syncStateRsp := &consensus.SyncStateResponse{
		ReplicaId:        rbft.chainConfig.SelfID,
		View:             rbft.chainConfig.View,
		SignedCheckpoint: signedCheckpoint,
	}
	rbft.recvSyncStateResponse(syncStateRsp, true)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) recvSyncState(sync *consensus.SyncState) consensusEvent {
	rbft.logger.Debugf("Replica %d received sync state from replica %d", rbft.chainConfig.SelfID, sync.ReplicaId)

	if !rbft.isNormal() {
		rbft.logger.Debugf("Replica %d is in abnormal, don't send sync state response", rbft.chainConfig.SelfID)
		return nil
	}

	// for normal case, send current state
	state := rbft.node.getCurrentState()
	if state == nil {
		rbft.logger.Warningf("Replica %d has a nil state", rbft.chainConfig.SelfID)
		return nil
	}
	signedCheckpoint, sErr := rbft.generateSignedCheckpoint(state, isConfigBatch(state.MetaState.Height, rbft.chainConfig.EpochInfo))
	if sErr != nil {
		rbft.logger.Errorf("Replica %d generate checkpoint error: %s", rbft.chainConfig.SelfID, sErr)
		rbft.stopNamespace()
		return nil
	}

	syncStateRsp := &consensus.SyncStateResponse{
		ReplicaId:        rbft.chainConfig.SelfID,
		View:             rbft.chainConfig.View,
		SignedCheckpoint: signedCheckpoint,
	}

	payload, err := syncStateRsp.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("Marshal SyncStateResponse Error!")
		return nil
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_SYNC_STATE_RESPONSE,
		Payload: payload,
	}
	rbft.peerMgr.unicast(context.TODO(), consensusMsg, sync.ReplicaId)
	rbft.logger.Debugf("Replica %d send sync state response to replica %d: view=%d, checkpoint=%s",
		rbft.chainConfig.SelfID, sync.ReplicaId, rbft.chainConfig.View, signedCheckpoint.GetCheckpoint().Pretty())
	return nil
}

func (rbft *rbftImpl[T, Constraint]) recvSyncStateResponse(rsp *consensus.SyncStateResponse, local bool) consensusEvent {
	if !rbft.in(InSyncState) {
		rbft.logger.Debugf("Replica %d is not in sync state, ignore it...", rbft.chainConfig.SelfID)
		return nil
	}

	if rsp.GetSignedCheckpoint() == nil || rsp.GetSignedCheckpoint().GetCheckpoint() == nil {
		rbft.logger.Errorf("Replica %d reject sync state response with nil checkpoint info", rbft.chainConfig.SelfID)
		return nil
	}
	// verify signature of remote checkpoint.
	if !local {
		vErr := rbft.verifySignedCheckpoint(rsp.GetSignedCheckpoint())
		if vErr != nil {
			rbft.logger.Errorf("Replica %d verify signature of checkpoint from %d error: %s",
				rbft.chainConfig.SelfID, rsp.ReplicaId, vErr)
			return nil
		}
	}

	checkpoint := rsp.GetSignedCheckpoint().GetCheckpoint()
	rbft.logger.Debugf("Replica %d now received sync state response from replica %d: view=%d, checkpoint=%s",
		rbft.chainConfig.SelfID, rsp.ReplicaId, rsp.View, checkpoint.Pretty())

	if oldRsp, ok := rbft.recoveryMgr.syncRspStore[rsp.ReplicaId]; ok {
		if oldRsp.GetSignedCheckpoint().GetCheckpoint().Height() > checkpoint.Height() {
			rbft.logger.Debugf("Duplicate sync state response, new height=%d is lower than old height=%d, reject it",
				checkpoint.Height(), oldRsp.GetSignedCheckpoint().GetCheckpoint().Height())
			return nil
		}
	}
	rbft.recoveryMgr.syncRspStore[rsp.ReplicaId] = rsp
	if len(rbft.recoveryMgr.syncRspStore) >= rbft.commonCaseQuorum() {
		states := make(wholeStates)
		for _, response := range rbft.recoveryMgr.syncRspStore {
			states[response.GetSignedCheckpoint()] = nodeState{
				view:   response.View,
				height: response.GetSignedCheckpoint().GetCheckpoint().Height(),
				digest: response.GetSignedCheckpoint().GetCheckpoint().Digest(),
			}
		}
		return rbft.compareWholeStates(states)
	}
	return nil
}

// restartSyncState restart syncState immediately, only can be invoked after sync state
// restart timer expired.
func (rbft *rbftImpl[T, Constraint]) restartSyncState() consensusEvent {
	rbft.logger.Debugf("Replica %d now restart sync state", rbft.chainConfig.SelfID)

	rbft.recoveryMgr.syncRspStore = make(map[uint64]*consensus.SyncStateResponse)

	event := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}

	// start sync state restart timer to cycle sync state while there are no new requests.
	rbft.timerMgr.startTimer(syncStateRestartTimer, event)

	return rbft.initSyncState()
}

// exitSyncState exit syncState immediately.
func (rbft *rbftImpl[T, Constraint]) exitSyncState() {
	rbft.logger.Debugf("Replica %d now exit sync state", rbft.chainConfig.SelfID)
	rbft.off(InSyncState)
	rbft.off(NeedSyncState)
	rbft.timerMgr.stopTimer(syncStateRspTimer)
	rbft.timerMgr.stopTimer(syncStateRestartTimer)
}
