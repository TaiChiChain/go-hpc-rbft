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
	"fmt"

	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpool "github.com/ultramesh/flato-txpool"
)

// batchManager manages basic batch issues, including:
// 1. requestPool which manages all transactions received from client or rpc layer
// 2. batch events timer management
type batchManager struct {
	seqNo            uint64             // track the prePrepare batch seqNo
	cacheBatch       []*pb.RequestBatch // cache batches wait for prePreparing
	eventChan        chan<- interface{}
	batchTimerActive bool // track the batch timer event, true means there exists an undergoing batch timer event
	namespace        string
	config           Config
	requestPool      txpool.TxPool
}

// newBatchManager initializes an instance of batchManager. batchManager subscribes txHashBatch from requestPool module
// and push it to rbftQueue for primary to construct TransactionBatch for consensus
func newBatchManager(eventC chan<- interface{}, c Config) *batchManager {
	bm := &batchManager{
		eventChan:   eventC,
		requestPool: c.RequestPool,
	}

	return bm
}

func (bm *batchManager) getSeqNo() uint64 {
	return bm.seqNo
}

func (bm *batchManager) setSeqNo(seqNo uint64) {
	bm.seqNo = seqNo
}

// isBatchTimerActive returns if the batch timer is active or not
func (bm *batchManager) isBatchTimerActive() bool {
	return bm.batchTimerActive
}

// startBatchTimer starts the batch timer and sets the batchTimerActive to true
func (rbft *rbftImpl) startBatchTimer() {
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}

	rbft.timerMgr.startTimer(batchTimer, localEvent)
	rbft.batchMgr.batchTimerActive = true
	rbft.logger.Debugf("Primary %d started the batch timer", rbft.no)
}

// stopBatchTimer stops the batch timer and reset the batchTimerActive to false
func (rbft *rbftImpl) stopBatchTimer() {
	rbft.timerMgr.stopTimer(batchTimer)
	rbft.batchMgr.batchTimerActive = false
	rbft.logger.Debugf("Primary %d stopped the batch timer", rbft.no)
}

// restartBatchTimer restarts the batch timer
func (rbft *rbftImpl) restartBatchTimer() {
	rbft.timerMgr.stopTimer(batchTimer)

	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}

	rbft.timerMgr.startTimer(batchTimer, localEvent)
	rbft.batchMgr.batchTimerActive = true
	rbft.logger.Debugf("Primary %d restarted the batch timer", rbft.no)
}

func (rbft *rbftImpl) softRestartBatchTimer() {
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}

	hasStarted, _ := rbft.timerMgr.softStartTimerWithNewTT(batchTimer, rbft.timerMgr.getTimeoutValue(batchTimer), localEvent)
	if hasStarted {
		rbft.logger.Debugf("Replica %d has started batch timer before", rbft.no)
		return
	}
	rbft.batchMgr.batchTimerActive = true
	rbft.logger.Debugf("Primary %d softly restarted the batch timer", rbft.no)
}

// startCheckPoolTimer starts the check pool timer when node enter normal status.
func (rbft *rbftImpl) startCheckPoolTimer() {
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreCheckPoolTimerEvent,
	}

	rbft.timerMgr.startTimer(checkPoolTimer, localEvent)
	rbft.logger.Debugf("Primary %d started the check pool timer", rbft.no)
}

// stopCheckPoolTimer stops the check pool timer when node enter abnormal status.
func (rbft *rbftImpl) stopCheckPoolTimer() {
	rbft.timerMgr.stopTimer(checkPoolTimer)
	rbft.logger.Debugf("Primary %d stopped the check pool timer", rbft.no)
}

// restartCheckPoolTimer restarts the check pool timer.
func (rbft *rbftImpl) restartCheckPoolTimer() {
	rbft.timerMgr.stopTimer(checkPoolTimer)

	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreCheckPoolTimerEvent,
	}

	rbft.timerMgr.startTimer(checkPoolTimer, localEvent)
	rbft.logger.Debugf("Primary %d restarted the check pool timer", rbft.no)
}

// maybeSendPrePrepare used by primary helps primary stores this batch and send prePrepare,
// flag findCache indicates whether we need to get request batch from cacheBatch or not as
// we may store batch into cacheBatch temporarily if next demand seqNo is higher than our
// high watermark. (this is where we restrict the speed of consensus)
func (rbft *rbftImpl) maybeSendPrePrepare(batch *pb.RequestBatch, findCache bool) {
	var (
		nextSeqNo uint64
		nextBatch *pb.RequestBatch
	)

	nextSeqNo = rbft.batchMgr.getSeqNo() + 1
	// restrict the speed of sending prePrepare.
	if !rbft.sendInW(nextSeqNo) {
		if !findCache {
			rbft.logger.Debugf("Replica %d is primary, not sending prePrepare for request batch %s because "+
				"next seqNo is out of high watermark %d", rbft.no, batch.BatchHash, rbft.h+rbft.L)
			rbft.batchMgr.cacheBatch = append(rbft.batchMgr.cacheBatch, batch)
		}
		rbft.logger.Debugf("Replica %d is primary, not sending prePrepare for request batch in cache because "+
			"next seqNo is out of high watermark %d", rbft.no, rbft.h+rbft.L)
		return
	}

	if findCache {
		nextBatch = rbft.batchMgr.cacheBatch[0]
		rbft.batchMgr.cacheBatch = rbft.batchMgr.cacheBatch[1:]
		rbft.logger.Infof("Primary %d finds cached batch, hash = %s", rbft.no, nextBatch.BatchHash)
	} else {
		nextBatch = batch
	}

	digest := nextBatch.BatchHash
	// check for other PRE-PREPARE for same digest, but different seqNo
	if rbft.storeMgr.existedDigest(nextSeqNo, rbft.view, digest) {
		return
	}

	nextBatch.SeqNo = nextSeqNo

	// cache and persist batch
	rbft.storeMgr.outstandingReqBatches[digest] = nextBatch
	rbft.storeMgr.batchStore[digest] = nextBatch
	rbft.persistBatch(digest)

	// here we soft start a new view timer with requestTimeout, if primary cannot execute this batch
	// during that timeout, we think there may exist some problems with this primary which will trigger viewChange.
	rbft.softStartNewViewTimer(rbft.timerMgr.getTimeoutValue(requestTimer),
		fmt.Sprintf("New request batch for view=%d/seqNo=%d", rbft.view, nextSeqNo), false)

	rbft.sendPrePrepare(nextSeqNo, digest, nextBatch)

	if rbft.sendInW(nextSeqNo+1) && len(rbft.batchMgr.cacheBatch) > 0 {
		rbft.restartBatchTimer()
		rbft.maybeSendPrePrepare(nil, true)
	}
}

// findNextCommitBatch used by backup nodes finds batch and commits
func (rbft *rbftImpl) findNextCommitBatch(digest string, v uint64, n uint64) error {
	cert := rbft.storeMgr.getCert(v, n, digest)
	if v != rbft.view {
		rbft.logger.Debugf("Replica %d find incorrect view in prepared cert with view=%d/seqNo=%d", rbft.no, v, n)
		return nil
	}

	if cert.prePrepare == nil {
		rbft.logger.Errorf("Replica %d get prePrepare failed for view=%d/seqNo=%d/digest=%s",
			rbft.no, v, n, digest)
		return nil
	}

	if rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d do not try to send commit because it's in stateUpdate", rbft.no)
		return nil
	}

	if digest == "" {
		rbft.logger.Infof("Replica %d send commit for no-op batch with view=%d/seqNo=%d", rbft.no, v, n)
		return rbft.sendCommit(digest, v, n)
	}
	// when system restart or finished vc/recovery, we need to resend commit for cert
	// with seqNo <= lastExec. However, those batches may have been deleted from requestPool
	// so we can get those batches from batchStore first.
	if existBatch, ok := rbft.storeMgr.batchStore[digest]; ok {
		rbft.logger.Debugf("Replica %d commit batch for view=%d/seqNo=%d, batch size: %d", rbft.no, v, n, len(existBatch.RequestHashList))
		return rbft.sendCommit(digest, v, n)
	}
	prePrep := cert.prePrepare

	txList, localList, missingTxs, err := rbft.batchMgr.requestPool.GetRequestsByHashList(digest, prePrep.HashBatch.Timestamp, prePrep.HashBatch.RequestHashList, prePrep.HashBatch.DeDuplicateRequestHashList)
	if err != nil {
		rbft.logger.Warningf("Replica %d get error when get txList, err: %v", rbft.no, err)
		rbft.sendViewChange()
		return nil
	}
	if missingTxs != nil {
		_ = rbft.fetchMissingTxs(prePrep, missingTxs)
		return nil
	}

	batch := &pb.RequestBatch{
		RequestHashList: prePrep.HashBatch.RequestHashList,
		RequestList:     txList,
		Timestamp:       prePrep.HashBatch.Timestamp,
		SeqNo:           prePrep.SequenceNumber,
		LocalList:       localList,
		BatchHash:       digest,
	}

	// store batch to outstandingReqBatches until execute this batch
	rbft.storeMgr.outstandingReqBatches[prePrep.BatchDigest] = batch
	rbft.storeMgr.batchStore[prePrep.BatchDigest] = batch
	rbft.persistBatch(digest)

	rbft.logger.Debugf("Replica %d commit batch for view=%d/seqNo=%d, batch size: %d", rbft.no, v, n, len(txList))
	return rbft.sendCommit(digest, v, n)
}

// primaryResubmitTransactions handles the transactions put in requestPool during
// viewChange, updateN and recovery if current node is new primary.
func (rbft *rbftImpl) primaryResubmitTransactions() {
	if rbft.isPrimary(rbft.peerPool.localID) {
		// if primary has transactions in requestPool, generate batches of the transactions
		for rbft.batchMgr.requestPool.HasPendingRequestInPool() {
			batches := rbft.batchMgr.requestPool.GenerateRequestBatch()
			rbft.postBatches(batches)
		}
	}
}
