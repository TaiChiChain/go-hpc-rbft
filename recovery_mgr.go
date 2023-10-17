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
	rbft.logger.Debugf("Replica %d fetch PQC", rbft.peerMgr.selfID)

	fetch := &consensus.FetchPQCRequest{
		H:         rbft.chainConfig.H,
		ReplicaId: rbft.peerMgr.selfID,
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
	rbft.logger.Debugf("Replica %d received fetchPQCRequest from replica %d", rbft.peerMgr.selfID, fetch.ReplicaId)

	remoteH := fetch.H
	if remoteH >= rbft.chainConfig.H+rbft.chainConfig.L {
		rbft.logger.Warningf("Replica %d received fetchPQCRequest, but its rbft.h ≥ highwatermark", rbft.peerMgr.selfID)
		return nil
	}

	var prePres []*consensus.PrePrepare
	var pres []*consensus.Prepare
	var cmts []*consensus.Commit

	// replica just send all PQC info we had sent before
	for idx, cert := range rbft.storeMgr.certStore {
		// send all PQC that n > remoteH in current view, help remote node to advance.
		if idx.n > remoteH && idx.v == rbft.chainConfig.View {
			// only response with messages we have sent.
			if cert.prePrepare == nil {
				rbft.logger.Warningf("Replica %d in finds nil prePrepare for view=%d/seqNo=%d",
					rbft.peerMgr.selfID, idx.v, idx.n)
			} else if cert.prePrepare.ReplicaId == rbft.peerMgr.selfID {
				prePres = append(prePres, cert.prePrepare)
			}
			for _, pre := range cert.prepare {
				if pre.ReplicaId == rbft.peerMgr.selfID {
					prepare := pre
					pres = append(pres, prepare)
				}
			}
			for _, cmt := range cert.commit {
				if cmt.ReplicaId == rbft.peerMgr.selfID {
					commit := cmt
					cmts = append(cmts, commit)
				}
			}
		}
	}

	pqcResponse := &consensus.FetchPQCResponse{
		ReplicaId: rbft.peerMgr.selfID,
	}

	if prePres != nil {
		pqcResponse.PrepreSet = prePres
	}
	if pres != nil {
		pqcResponse.PreSet = pres
	}
	if cmts != nil {
		pqcResponse.CmtSet = cmts
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
	rbft.logger.Debugf("Replica %d send PQC response to %d, detailed: {view:%d,seq:%d,digest:%s}", rbft.peerMgr.selfID,
		fetch.ReplicaId, view, sequenceNumber, batchDigest)

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
		rbft.peerMgr.selfID, PQCInfo.ReplicaId, view, sequenceNumber, batchDigest)

	// post all the PQC
	if !rbft.isPrimary(rbft.peerMgr.selfID) {
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
			rbft.logger.Debugf("Replica %d not try to sync state as we are in abnormal now", rbft.peerMgr.selfID)
			return
		}
		rbft.logger.Infof("Replica %d need to start sync state progress after %v", rbft.peerMgr.selfID, rbft.timerMgr.getTimeoutValue(syncStateRestartTimer))
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
		rbft.logger.Warningf("Replica %d try to send syncState, but it's already in sync state", rbft.peerMgr.selfID)
		return nil
	}

	rbft.on(InSyncState)

	rbft.logger.Debugf("Replica %d now init sync state", rbft.peerMgr.selfID)

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
		ReplicaId: rbft.peerMgr.selfID,
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
		rbft.logger.Warningf("Replica %d has a nil node state", rbft.peerMgr.selfID)
		return nil
	}

	signedCheckpoint, sErr := rbft.generateSignedCheckpoint(state, isConfigBatch(state.MetaState.Height, rbft.chainConfig.EpochInfo))
	if sErr != nil {
		rbft.logger.Errorf("Replica %d generate checkpoint error: %s", rbft.peerMgr.selfID, sErr)
		rbft.stopNamespace()
		return nil
	}
	syncStateRsp := &consensus.SyncStateResponse{
		ReplicaId:        rbft.peerMgr.selfID,
		View:             rbft.chainConfig.View,
		SignedCheckpoint: signedCheckpoint,
	}
	rbft.recvSyncStateResponse(syncStateRsp, true)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) recvSyncState(sync *consensus.SyncState) consensusEvent {
	rbft.logger.Debugf("Replica %d received sync state from replica %d", rbft.peerMgr.selfID, sync.ReplicaId)

	if !rbft.isNormal() {
		rbft.logger.Debugf("Replica %d is in abnormal, don't send sync state response", rbft.peerMgr.selfID)
		return nil
	}

	// for normal case, send current state
	state := rbft.node.getCurrentState()
	if state == nil {
		rbft.logger.Warningf("Replica %d has a nil state", rbft.peerMgr.selfID)
		return nil
	}
	signedCheckpoint, sErr := rbft.generateSignedCheckpoint(state, isConfigBatch(state.MetaState.Height, rbft.chainConfig.EpochInfo))
	if sErr != nil {
		rbft.logger.Errorf("Replica %d generate checkpoint error: %s", rbft.peerMgr.selfID, sErr)
		rbft.stopNamespace()
		return nil
	}

	syncStateRsp := &consensus.SyncStateResponse{
		ReplicaId:        rbft.peerMgr.selfID,
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
		rbft.peerMgr.selfID, sync.ReplicaId, rbft.chainConfig.View, signedCheckpoint.GetCheckpoint().Pretty())
	return nil
}

func (rbft *rbftImpl[T, Constraint]) recvSyncStateResponse(rsp *consensus.SyncStateResponse, local bool) consensusEvent {
	if !rbft.in(InSyncState) {
		rbft.logger.Debugf("Replica %d is not in sync state, ignore it...", rbft.peerMgr.selfID)
		return nil
	}

	if rsp.GetSignedCheckpoint() == nil || rsp.GetSignedCheckpoint().GetCheckpoint() == nil {
		rbft.logger.Errorf("Replica %d reject sync state response with nil checkpoint info", rbft.peerMgr.selfID)
		return nil
	}
	// verify signature of remote checkpoint.
	if !local {
		vErr := rbft.verifySignedCheckpoint(rsp.GetSignedCheckpoint())
		if vErr != nil {
			rbft.logger.Errorf("Replica %d verify signature of checkpoint from %d error: %s",
				rbft.peerMgr.selfID, rsp.ReplicaId, vErr)
			return nil
		}
	}

	checkpoint := rsp.GetSignedCheckpoint().GetCheckpoint()
	rbft.logger.Debugf("Replica %d now received sync state response from replica %d: view=%d, checkpoint=%s",
		rbft.peerMgr.selfID, rsp.ReplicaId, rsp.View, checkpoint.Pretty())

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
	rbft.logger.Debugf("Replica %d now restart sync state", rbft.peerMgr.selfID)

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
	rbft.logger.Debugf("Replica %d now exit sync state", rbft.peerMgr.selfID)
	rbft.off(InSyncState)
	rbft.off(NeedSyncState)
	rbft.timerMgr.stopTimer(syncStateRspTimer)
	rbft.timerMgr.stopTimer(syncStateRestartTimer)
}
