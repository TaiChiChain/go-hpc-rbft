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
	"fmt"

	"github.com/axiomesh/axiom-kit/txpool"
	"github.com/axiomesh/axiom-kit/types"
)

// batchManager manages basic batch issues, including:
// 1. requestPool which manages all transactions received from client or rpc layer
// 2. batch events timer management
type batchManager[T any, Constraint types.TXConstraint[T]] struct {
	seqNo                 uint64                         // track the prePrepare batch seqNo
	cacheBatch            []*RequestBatch[T, Constraint] // cache batches wait for prePreparing
	batchTimerActive      bool                           // track the batch timer event, true means there exists an undergoing batch timer event
	noTxBatchTimerActive  bool                           // track the batch timer event, true means there exists an undergoing batch timer event
	lastBatchTime         int64                          // track last batch time [only used by primary], reset when become primary.
	minTimeoutBatchTime   float64                        // track the min timeout batch time [only used by primary], reset when become primary.
	minTimeoutNoBatchTime float64                        // track the min no tx timeout batch time [only used by primary], reset when become primary.
	requestPool           txpool.TxPool[T, Constraint]
}

// newBatchManager initializes an instance of batchManager.
// batchManager caches batches that have been generated by requestPool but cannot
// be sent out because of high watermark limit.
func newBatchManager[T any, Constraint types.TXConstraint[T]](requestPool txpool.TxPool[T, Constraint], c Config) *batchManager[T, Constraint] {
	bm := &batchManager[T, Constraint]{
		requestPool: requestPool,
	}

	return bm
}

func (bm *batchManager[T, Constraint]) getSeqNo() uint64 {
	return bm.seqNo
}

func (bm *batchManager[T, Constraint]) setSeqNo(seqNo uint64) {
	bm.seqNo = seqNo
}

// isBatchTimerActive returns if the batch timer is active or not
func (bm *batchManager[T, Constraint]) isBatchTimerActive() bool {
	return bm.batchTimerActive
}

// isNoTxBatchTimerActive returns if the no tx batch timer is active or not
func (bm *batchManager[T, Constraint]) isNoTxBatchTimerActive() bool {
	return bm.noTxBatchTimerActive
}

// startBatchTimer starts the batch timer and sets the batchTimerActive to true
func (rbft *rbftImpl[T, Constraint]) startBatchTimer() bool {
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}

	rbft.timerMgr.startTimer(batchTimer, localEvent)
	rbft.batchMgr.batchTimerActive = true
	rbft.logger.Debugf("Replica %d started the batch timer", rbft.chainConfig.SelfID)

	return true
}

// startNoTxBatchTimer starts the batch timer and sets the batchTimerActive to true
func (rbft *rbftImpl[T, Constraint]) startNoTxBatchTimer() bool {
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNoTxBatchTimerEvent,
	}

	rbft.timerMgr.startTimer(noTxBatchTimer, localEvent)
	rbft.batchMgr.noTxBatchTimerActive = true
	rbft.logger.Debugf("Replica %d started the no tx batch timer", rbft.chainConfig.SelfID)

	return true
}

// stopNoTxBatchTimer stops the batch timer and reset the batchTimerActive to false
func (rbft *rbftImpl[T, Constraint]) stopNoTxBatchTimer() {
	rbft.timerMgr.stopTimer(noTxBatchTimer)
	rbft.batchMgr.noTxBatchTimerActive = false
	rbft.logger.Debugf("Replica %d stopped the no tx batch timer", rbft.chainConfig.SelfID)
}

// restartNoTxBatchTimer restarts the no tx batch timer
func (rbft *rbftImpl[T, Constraint]) restartNoTxBatchTimer() bool {
	rbft.timerMgr.stopTimer(noTxBatchTimer)

	// if timed gen empty block is disabled, we do not need to restart the no tx batch timer
	if !rbft.chainConfig.EpochInfo.ConsensusParams.EnableTimedGenEmptyBlock ||
		rbft.batchMgr.requestPool.HasPendingRequestInPool() {
		return false
	}
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNoTxBatchTimerEvent,
	}

	rbft.timerMgr.startTimer(noTxBatchTimer, localEvent)
	rbft.batchMgr.batchTimerActive = true
	// rbft.logger.Debugf("Replica %d restarted the no tx batch timer", rbft.chainConfig.SelfID)

	return true
}

// stopBatchTimer stops the batch timer and reset the batchTimerActive to false
func (rbft *rbftImpl[T, Constraint]) stopBatchTimer() {
	rbft.timerMgr.stopTimer(batchTimer)
	rbft.batchMgr.batchTimerActive = false
	// rbft.logger.Debugf("Replica %d stopped the batch timer", rbft.chainConfig.SelfID)
}

// restartBatchTimer restarts the batch timer
func (rbft *rbftImpl[T, Constraint]) restartBatchTimer() bool {
	rbft.timerMgr.stopTimer(batchTimer)

	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}

	rbft.timerMgr.startTimer(batchTimer, localEvent)
	rbft.batchMgr.batchTimerActive = true
	// rbft.logger.Debugf("Replica %d restarted the batch timer", rbft.chainConfig.SelfID)

	return true
}

func (rbft *rbftImpl[T, Constraint]) softRestartBatchTimer() bool {
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}

	hasStarted, _ := rbft.timerMgr.softStartTimerWithNewTT(batchTimer, rbft.timerMgr.getTimeoutValue(batchTimer), localEvent)
	if hasStarted {
		rbft.logger.Debugf("Replica %d has started batch timer before", rbft.chainConfig.SelfID)
		return false
	}
	rbft.batchMgr.batchTimerActive = true
	rbft.logger.Debugf("Replica %d softly restarted the batch timer", rbft.chainConfig.SelfID)

	return true
}

func (rbft *rbftImpl[T, Constraint]) isActiveCheckPoolTimer() bool {
	return rbft.timerMgr.tTimers[checkPoolTimer].count() > 0
}

// startCheckPoolTimer starts the check pool timer when node enter normal status.
func (rbft *rbftImpl[T, Constraint]) startCheckPoolTimer() {
	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreCheckPoolTimerEvent,
	}

	rbft.timerMgr.startTimer(checkPoolTimer, localEvent)
	rbft.logger.Debugf("Replica %d started the check pool timer", rbft.chainConfig.SelfID)
}

// stopCheckPoolTimer stops the check pool timer when node enter abnormal status.
func (rbft *rbftImpl[T, Constraint]) stopCheckPoolTimer() {
	rbft.timerMgr.stopTimer(checkPoolTimer)
	rbft.logger.Debugf("Replica %d stopped the check pool timer", rbft.chainConfig.SelfID)
}

// restartCheckPoolTimer restarts the check pool timer.
func (rbft *rbftImpl[T, Constraint]) restartCheckPoolTimer() {
	rbft.timerMgr.stopTimer(checkPoolTimer)

	localEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreCheckPoolTimerEvent,
	}

	rbft.timerMgr.startTimer(checkPoolTimer, localEvent)
	rbft.logger.Debugf("Replica %d restarted the check pool timer", rbft.chainConfig.SelfID)
}

// // startCheckPoolTimer starts the check pool remove timer when tx stay txpool too long.
// func (rbft *rbftImpl[T, Constraint]) startCheckPoolRemoveTimer() {
//	localEvent := &LocalEvent{
//		Service:   CoreRbftService,
//		EventType: CoreCheckPoolRemoveTimerEvent,
//	}
//
//	rbft.timerMgr.startTimer(checkPoolRemoveTimer, localEvent)
//	rbft.logger.Debugf("Replica %d started the check pool remove timer, need to remove invalid tx in txpool after %v", rbft.chainConfig.SelfID, rbft.timerMgr.getTimeoutValue(checkPoolRemoveTimer))
// }
//
// // restartCheckPoolTimer restarts the check pool timer.
// func (rbft *rbftImpl[T, Constraint]) restartCheckPoolRemoveTimer() {
//	rbft.timerMgr.stopTimer(checkPoolRemoveTimer)
//
//	localEvent := &LocalEvent{
//		Service:   CoreRbftService,
//		EventType: CoreCheckPoolRemoveTimerEvent,
//	}
//
//	rbft.timerMgr.startTimer(checkPoolRemoveTimer, localEvent)
//	rbft.logger.Debugf("Replica %d restarted the check pool remove timer, need to remove invalid tx in txpool after %v", rbft.chainConfig.SelfID, rbft.timerMgr.getTimeoutValue(checkPoolRemoveTimer))
// }

// maybeSendPrePrepare used by primary helps primary stores this batch and send prePrepare,
// flag findCache indicates whether we need to get request batch from cacheBatch or not as
// we may store batch into cacheBatch temporarily if next demand seqNo is higher than our
// high watermark. (this is where we restrict the speed of consensus)
func (rbft *rbftImpl[T, Constraint]) maybeSendPrePrepare(batch *RequestBatch[T, Constraint], findCache bool) {
	if !rbft.chainConfig.isValidator() {
		rbft.logger.Debugf("Replica %d is not validator, not execute maybeSendPrePrepare",
			rbft.chainConfig.SelfID)
		return
	}

	var (
		nextSeqNo uint64
		nextBatch *RequestBatch[T, Constraint]
	)

	if rbft.in(waitCheckpointBatchExecute) {
		rbft.logger.Debugf("Replica %d is wait checkpoint block %d executed, not sending prePrepare",
			rbft.chainConfig.SelfID, rbft.exec.lastExec)
		return
	}

	nextSeqNo = rbft.batchMgr.getSeqNo() + 1
	// restrict the speed of sending prePrepare.
	if rbft.beyondRange(nextSeqNo) || !rbft.inPrimaryTerm() {
		batchHash := "<nil>"
		if batch != nil {
			batchHash = batch.BatchHash
		}
		if !rbft.inPrimaryTerm() {
			if batchHash != "<nil>" {
				rbft.logger.Debugf("Replica %d is primary, not sending prePrepare for request batch %s because "+
					"next seqNo is out of high watermark %d, restore the batch", rbft.chainConfig.SelfID, batchHash, rbft.chainConfig.H+rbft.chainConfig.L, findCache)
				if err := rbft.batchMgr.requestPool.RestoreOneBatch(batchHash); err != nil {
					rbft.logger.Debugf("Replica %d is primary, restore the batch failed: %s", rbft.chainConfig.SelfID, err.Error())
				}
			}
			return
		}

		rbft.logger.Debugf("Replica %d is primary, not sending prePrepare for request batch %s because "+
			"next seqNo is out of high watermark %d, while findCache is %t", rbft.chainConfig.SelfID, batchHash, rbft.chainConfig.H+rbft.chainConfig.L, findCache)

		if !findCache {
			rbft.batchMgr.cacheBatch = append(rbft.batchMgr.cacheBatch, batch)
			rbft.metrics.cacheBatchNumber.Add(float64(1))
		}
		// the cluster needs to generate a stable checkpoint every K blocks to collect the garbage.
		// here, the primary is trying to send a pre-prepare out of high-watermark, we need to start a timer for it
		// in order to generate the stable checkpoint
		rbft.softStartHighWatermarkTimer("primary send pre-prepare out of range")
		return
	}

	if findCache {
		nextBatch = rbft.batchMgr.cacheBatch[0]
		rbft.batchMgr.cacheBatch = rbft.batchMgr.cacheBatch[1:]
		rbft.metrics.cacheBatchNumber.Add(float64(-1))
		rbft.logger.Infof("Primary %d finds cached batch, hash = %s", rbft.chainConfig.SelfID, nextBatch.BatchHash)
	} else {
		nextBatch = batch
	}

	digest := nextBatch.BatchHash
	// check for other PRE-PREPARE for same digest, but different seqNo
	if rbft.storeMgr.existedDigest(rbft.chainConfig.View, nextSeqNo, digest) {
		return
	}

	nextBatch.SeqNo = nextSeqNo

	// cache and persist batch
	rbft.storeMgr.outstandingReqBatches[digest] = nextBatch
	rbft.storeMgr.batchStore[digest] = nextBatch
	rbft.persistBatch(digest)
	rbft.metrics.batchesGauge.Add(float64(1))
	rbft.metrics.outstandingBatchesGauge.Add(float64(1))

	// here we soft start a new view timer with requestTimeout, if primary cannot execute this batch
	// during that timeout, we think there may exist some problems with this primary which will trigger viewChange.
	rbft.softStartNewViewTimer(rbft.timerMgr.getTimeoutValue(requestTimer),
		fmt.Sprintf("New request batch for view=%d/seqNo=%d", rbft.chainConfig.View, nextSeqNo), false)

	rbft.sendPrePrepare(nextSeqNo, digest, nextBatch)

	if rbft.sendInW(nextSeqNo+1) && len(rbft.batchMgr.cacheBatch) > 0 {
		rbft.maybeSendPrePrepare(nil, true)
	}
}

// findNextPrepareBatch is used by the backup nodes to ensure that the batch corresponding to this cert exists.
// If it exists, then prepare it.
func (rbft *rbftImpl[T, Constraint]) findNextPrepareBatch(ctx context.Context, v uint64, n uint64, d string) error {
	rbft.logger.Debugf("Replica %d findNextPrepareBatch in cert with for view=%d/seqNo=%d/digest=%s", rbft.chainConfig.SelfID, v, n, d)

	if v != rbft.chainConfig.View {
		rbft.logger.Debugf("Replica %d excepts the cert with view=%d, but actually the cert is view=%d/seqNo=%d/digest=%s", rbft.chainConfig.SelfID, rbft.chainConfig.View, v, n, d)
		return nil
	}

	cert := rbft.storeMgr.getCert(v, n, d)
	if cert.prePrepare == nil {
		rbft.logger.Errorf("Replica %d get prePrepare failed for view=%d/seqNo=%d/digest=%s",
			rbft.chainConfig.SelfID, v, n, d)
		return nil
	}

	if rbft.in(SkipInProgress) {
		rbft.logger.Debugf("Replica %d do not try to send prepare because it's in stateUpdate", rbft.chainConfig.SelfID)
		return nil
	}

	if d == "" {
		rbft.logger.Infof("Replica %d send prepare for no-op batch with view=%d/seqNo=%d", rbft.chainConfig.SelfID, v, n)
		return rbft.sendPrepare(ctx, v, n, d)
	}
	// when system restart or finished vc, we need to resend prepare for cert
	// with seqNo <= lastExec. However, those batches may have been deleted from requestPool,
	// so we can get those batches from batchStore first.
	if existBatch, ok := rbft.storeMgr.batchStore[d]; ok {
		if isConfigBatch(existBatch.SeqNo, rbft.chainConfig.EpochInfo) {
			rbft.logger.Debugf("Replica %d generate a config batch", rbft.chainConfig.SelfID)
			rbft.atomicOn(InConfChange)
			cert.isConfig = true
			rbft.epochMgr.configBatchInOrder = n
			rbft.metrics.statusGaugeInConfChange.Set(InConfChange)
		}
		rbft.logger.Debugf("Replica %d prepare batch for view=%d/seqNo=%d, batch size: %d", rbft.chainConfig.SelfID, v, n, len(existBatch.RequestHashList))
		return rbft.sendPrepare(ctx, v, n, d)
	}
	prePrep := cert.prePrepare

	txList, localList, missingTxs, err := rbft.batchMgr.requestPool.GetRequestsByHashList(d, prePrep.HashBatch.Timestamp, prePrep.HashBatch.RequestHashList, prePrep.HashBatch.DeDuplicateRequestHashList)
	if err != nil {
		rbft.logger.Warningf("Replica %d get error when get txList, err: %v", rbft.chainConfig.SelfID, err)
		rbft.sendViewChange()
		return nil
	}
	if missingTxs != nil {
		rbft.fetchMissingTxs(ctx, prePrep, missingTxs)
		return nil
	}

	batch := &RequestBatch[T, Constraint]{
		RequestHashList: prePrep.HashBatch.RequestHashList,
		RequestList:     txList,
		Timestamp:       prePrep.HashBatch.Timestamp,
		SeqNo:           prePrep.SequenceNumber,
		LocalList:       localList,
		BatchHash:       d,
		Proposer:        prePrep.HashBatch.Proposer,
	}

	// store batch to outstandingReqBatches until execute this batch
	rbft.storeMgr.outstandingReqBatches[d] = batch
	rbft.storeMgr.batchStore[d] = batch
	rbft.persistBatch(d)
	rbft.metrics.batchesGauge.Add(float64(1))
	rbft.metrics.outstandingBatchesGauge.Add(float64(1))

	if isConfigBatch(batch.SeqNo, rbft.chainConfig.EpochInfo) {
		rbft.logger.Debugf("Replica %d generate a config batch", rbft.chainConfig.SelfID)
		rbft.atomicOn(InConfChange)
		cert.isConfig = true
		rbft.epochMgr.configBatchInOrder = n
		rbft.metrics.statusGaugeInConfChange.Set(InConfChange)
	}

	rbft.logger.Debugf("Replica %d prepare batch for view=%d/seqNo=%d, batch size: %d", rbft.chainConfig.SelfID, v, n, len(txList))

	return rbft.sendPrepare(ctx, v, n, d)
}

// primaryResubmitTransactions tries to submit transactions for primary after abnormal
// or stable checkpoint.
func (rbft *rbftImpl[T, Constraint]) primaryResubmitTransactions() {
	if !rbft.atomicIn(InConfChange) {
		// reset lastBatchTime for primary.
		rbft.batchMgr.lastBatchTime = 0
		rbft.logger.Debugf("======== Primary %d resubmit transactions", rbft.chainConfig.SelfID)

		if !rbft.isNormal() {
			rbft.logger.Debugf("Primary %d is in abnormal, reject resubmit", rbft.chainConfig.SelfID)
			return
		}

		// try cached batches first if any.
		if len(rbft.batchMgr.cacheBatch) > 0 {
			rbft.timerMgr.stopTimer(nullRequestTimer)
			rbft.maybeSendPrePrepare(nil, true)
		}

		// if primary has transactions in requestPool, generate batches of the transactions
		for rbft.batchMgr.requestPool.HasPendingRequestInPool() {
			if !rbft.isNormal() {
				rbft.logger.Debugf("Primary %d is in abnormal, reject resubmit", rbft.chainConfig.SelfID)
				return
			}

			if rbft.inPrimaryTerm() {
				// if not test, must generate a full txs batch
				if !rbft.isTest && !rbft.batchMgr.requestPool.PendingRequestsNumberIsReady() {
					// we need to reply signal to requestPool for replying next batch signal
					rbft.logger.Debugf("Primary %d is in primary term, but pending requests number is not ready, reply batch signal", rbft.chainConfig.SelfID)
					rbft.batchMgr.requestPool.ReplyBatchSignal()
					break
				}
				var (
					batch *txpool.RequestHashBatch[T, Constraint]
					err   error
				)
				if rbft.isTest {
					batch, err = rbft.batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchTimeoutEvent)
					if err != nil {
						rbft.logger.Warningf("Primary %d failed to generate a batch, err: %v", rbft.chainConfig.SelfID, err)
						return
					}
				} else {
					batch, err = rbft.batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchFirstEvent)
					if err != nil {
						rbft.logger.Debugf("Primary %d failed to generate a batch, err: %v", rbft.chainConfig.SelfID, err)
						return
					}
					if batch == nil {
						rbft.batchMgr.requestPool.ReplyBatchSignal()
					}
				}

				rbft.postBatches([]*txpool.RequestHashBatch[T, Constraint]{batch})
			} else {
				break
			}

			// if we have just generated a config batch, we need to break resubmit
			if rbft.atomicIn(InConfChange) {
				rbft.logger.Debug("Primary has just generated a config batch, break the resubmit")
				break
			}
		}
		rbft.logger.Debugf("======== Primary %d finished transactions resubmit", rbft.chainConfig.SelfID)
	}
}

func isConfigBatch(seqNo uint64, epochInfo *EpochInfo) bool {
	// when epoch change trigger
	return seqNo == epochInfo.StartBlock+epochInfo.EpochPeriod-1
}
