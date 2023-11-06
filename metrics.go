package rbft

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/axiomesh/axiom-bft/common/metrics"
)

// rbftMetrics helps collect all metrics in rbft
type rbftMetrics struct {
	// ========================== metrics related to basic RBFT info ==========================
	// track the current node ID.
	idGauge metrics.Gauge

	// track the current epoch number.
	epochGauge metrics.Gauge

	// track the current view number.
	viewGauge metrics.Gauge

	// track the current cluster size.
	clusterSizeGauge metrics.Gauge

	// track the current quorum size.
	quorumSizeGauge metrics.Gauge

	// track if current node is in normal.
	statusGaugeInNormal metrics.Gauge

	// track if current node is in conf change.
	statusGaugeInConfChange metrics.Gauge

	// track if current node is in view change.
	statusGaugeInViewChange metrics.Gauge

	// track if current node is in recovery.
	statusGaugeInRecovery metrics.Gauge

	// track if current node is in state transferring.
	statusGaugeStateTransferring metrics.Gauge

	// track if current node is pool full.
	statusGaugePoolFull metrics.Gauge

	// track if current node is in pending.
	statusGaugePending metrics.Gauge

	// track if current node is in inconsistent.
	statusGaugeInconsistent metrics.Gauge

	// ========================== metrics related to commit info ==========================
	// monitor the totally committed block number by RBFT.
	// Notice! this number may be smaller than chain height because we may go around
	// consensus and state update if we are fell behind.
	committedBlockNumber metrics.Counter

	// monitor the totally committed config block number by RBFT.
	committedConfigBlockNumber metrics.Counter

	// monitor the totally committed empty block number by RBFT.
	committedEmptyBlockNumber metrics.Counter

	// monitor the totally committed tx number by RBFT.
	committedTxs metrics.Counter

	// monitor the tx number in each committed block.
	txsPerBlock metrics.Summary

	// ========================== metrics related to batch duration info ==========================
	// monitor the batch generation interval for primary.
	batchInterval metrics.Summary

	minBatchIntervalDuration metrics.Gauge

	// monitor the batch persist time.
	batchPersistDuration metrics.Summary

	// monitor the time from batch[primary] to commit.
	// may be negative due to the time difference between primary and replica.
	batchToCommitDuration metrics.Summary

	// monitor the time from recvChan to processEvent.
	processEventDuration metrics.Histogram

	// ========================== metrics related to batch number info ==========================
	// monitor the batch number in batchStore, including the batches that cached to help
	// other lagging nodes to fetch.
	batchesGauge metrics.Gauge

	// monitor the batch number in outstandingReqBatches, including the batches that have
	// started consensus but not finished.
	outstandingBatchesGauge metrics.Gauge

	// monitor the batch number in cacheBatches, including the batches that have been
	// batched by primary but cannot start consensus due to high watermark limit.
	// [primary only]
	cacheBatchNumber metrics.Gauge

	// ========================== metrics related to fell behind info ==========================
	// monitor the state update times.
	stateUpdateCounter metrics.Counter

	// monitor the times of fetch missing txs which is caused by missing txs before commit.
	fetchMissingTxsCounter metrics.Counter

	// monitor the times of return fetch missing txs which is caused by backup node missing
	// txs before commit.
	returnFetchMissingTxsCounter metrics.Counter

	// monitor the times of fetch request batch which is caused by missing batches after vc.
	fetchRequestBatchCounter metrics.Counter

	// ========================== metrics related to txs/txSets info ==========================
	// monitor part of incoming tx sets, including tx sets from API and relayed from NVP.
	incomingLocalTxSets metrics.Counter

	// monitor part of incoming txs, including txs from API and relayed from other NVP.
	incomingLocalTxs metrics.Counter

	// monitor part of rejected txs due to consensus abnormal status, including txs from
	// API and relayed from other NVP.
	rejectedLocalTxs metrics.Counter

	// monitor part of incoming tx sets, including tx sets relayed from other VP.
	incomingRemoteTxSets metrics.Counter

	// monitor part of incoming txs, including txs relayed from other VP.
	incomingRemoteTxs metrics.Counter

	// monitor part of rejected txs due to consensus abnormal status, including txs relayed
	// from other VP.
	rejectedRemoteTxs metrics.Counter

	// ========================== metrics related to consensus progress time ==========================
	// monitor the duration from batch to prePrepared
	batchToPrePrepared metrics.Summary

	// monitor the duration from prePrepared to prepared
	prePreparedToPrepared metrics.Summary

	// monitor the duration from prepared to committed
	preparedToCommitted metrics.Summary
}

func newRBFTMetrics(metricsProv metrics.Provider) (*rbftMetrics, error) {
	var err error
	m := &rbftMetrics{}

	m.idGauge, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "ID",
			Help: "rbft node ID",
		},
	)
	if err != nil {
		return m, err
	}

	m.epochGauge, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "epoch",
			Help: "rbft epoch number",
		},
	)
	if err != nil {
		return m, err
	}

	m.viewGauge, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "view",
			Help: "rbft view number",
		},
	)
	if err != nil {
		return m, err
	}

	m.clusterSizeGauge, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "cluster_size",
			Help: "rbft cluster size",
		},
	)
	if err != nil {
		return m, err
	}

	m.quorumSizeGauge, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "quorum_size",
			Help: "rbft quorum size",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugeInNormal, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_normal",
			Help: "rbft is in normal or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugeInConfChange, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_conf_change",
			Help: "rbft is in conf change or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugeInViewChange, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_viewchange",
			Help: "rbft is in view change or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugeInRecovery, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_recovery",
			Help: "rbft is in recovery or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugeStateTransferring, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_state_update",
			Help: "rbft is in state transferring or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugePoolFull, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_pool_full",
			Help: "rbft is pool full or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugePending, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_pending",
			Help: "rbft is in pending or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.statusGaugeInconsistent, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "status_inconsistent",
			Help: "rbft is in inconsistent or not",
		},
	)
	if err != nil {
		return m, err
	}

	m.committedBlockNumber, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_block_number",
			Help: "rbft committed block number",
		},
	)
	if err != nil {
		return m, err
	}

	m.committedConfigBlockNumber, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_config_block_number",
			Help: "rbft committed config block number",
		},
	)
	if err != nil {
		return m, err
	}

	m.committedEmptyBlockNumber, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_empty_block_number",
			Help: "rbft committed empty block number",
		},
	)
	if err != nil {
		return m, err
	}

	m.committedTxs, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_txs",
			Help: "rbft committed tx number",
		},
	)
	if err != nil {
		return m, err
	}

	m.txsPerBlock, err = metricsProv.NewSummary(
		metrics.SummaryOpts{
			Name:       "committed_txs_per_block",
			Help:       "rbft committed tx number per block",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	if err != nil {
		return m, err
	}

	m.batchInterval, err = metricsProv.NewSummary(
		metrics.SummaryOpts{
			Name:       "batch_interval_duration",
			Help:       "interval duration of batch",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			LabelNames: []string{"type"},
		},
	)
	if err != nil {
		return m, err
	}

	m.minBatchIntervalDuration, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name:       "min_batch_interval_duration",
			Help:       "min timeout interval duration of batch",
			LabelNames: []string{"type"},
		})
	if err != nil {
		return m, err
	}

	m.batchPersistDuration, err = metricsProv.NewSummary(
		metrics.SummaryOpts{
			Name:       "batch_persist_duration",
			Help:       "persist duration of batch",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	if err != nil {
		return m, err
	}

	m.batchToCommitDuration, err = metricsProv.NewSummary(
		metrics.SummaryOpts{
			Name:       "batch_to_commit_duration",
			Help:       "duration from batch to commit",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	if err != nil {
		return m, err
	}

	m.processEventDuration, err = metricsProv.NewHistogram(
		metrics.HistogramOpts{
			Name:       "process_event_duration",
			Help:       "duration from recvChan to processEvent",
			Buckets:    prometheus.ExponentialBuckets(0.0005, 2, 10),
			LabelNames: []string{"event"},
		},
	)
	if err != nil {
		return m, err
	}

	m.batchesGauge, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "batch_number",
			Help: "rbft batch number cached in batchStore",
		},
	)
	if err != nil {
		return m, err
	}

	m.outstandingBatchesGauge, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "outstanding_batch_number",
			Help: "rbft outstanding batch number",
		},
	)
	if err != nil {
		return m, err
	}

	m.cacheBatchNumber, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "cache_batch_number",
			Help: "rbft cache batch number",
		},
	)
	if err != nil {
		return m, err
	}

	m.stateUpdateCounter, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "state_update_times",
			Help: "rbft state update times",
		},
	)
	if err != nil {
		return m, err
	}

	m.fetchMissingTxsCounter, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "fetch_missing_txs_times",
			Help: "rbft fetch missing txs times",
		},
	)
	if err != nil {
		return m, err
	}

	m.returnFetchMissingTxsCounter, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name:       "return_fetch_missing_txs_times",
			Help:       "rbft return fetch missing txs times",
			LabelNames: []string{"node"},
		},
	)
	if err != nil {
		return m, err
	}

	m.fetchRequestBatchCounter, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "fetch_request_batch_times",
			Help: "rbft fetch request batch times",
		},
	)
	if err != nil {
		return m, err
	}

	m.incomingLocalTxSets, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_local_tx_sets",
			Help: "rbft incoming local tx sets from API or NVP",
		},
	)
	if err != nil {
		return m, err
	}

	m.incomingLocalTxs, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_local_txs",
			Help: "rbft incoming local txs from API or NVP",
		},
	)
	if err != nil {
		return m, err
	}

	m.rejectedLocalTxs, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "rejected_local_txs",
			Help: "rbft rejected local txs from API or NVP",
		},
	)
	if err != nil {
		return m, err
	}
	m.rejectedLocalTxs.Add(0)

	m.incomingRemoteTxSets, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_remote_tx_sets",
			Help: "rbft incoming remote tx sets from other VP",
		},
	)
	if err != nil {
		return m, err
	}

	m.incomingRemoteTxs, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_remote_txs",
			Help: "rbft incoming remote txs from other VP",
		},
	)
	if err != nil {
		return m, err
	}

	m.rejectedRemoteTxs, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "rejected_remote_txs",
			Help: "rbft rejected remote txs from other VP",
		},
	)
	if err != nil {
		return m, err
	}
	m.rejectedRemoteTxs.Add(0)

	m.batchToPrePrepared, err = metricsProv.NewSummary(
		metrics.SummaryOpts{
			Name:       "batch_to_prePrepared",
			Help:       "interval from batch to prePrepared",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	if err != nil {
		return m, err
	}

	m.prePreparedToPrepared, err = metricsProv.NewSummary(
		metrics.SummaryOpts{
			Name:       "prePrepared_to_prepared",
			Help:       "interval from prePrepared to prepared",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	if err != nil {
		return m, err
	}

	m.preparedToCommitted, err = metricsProv.NewSummary(
		metrics.SummaryOpts{
			Name:       "prepared_to_committed",
			Help:       "interval from prepared to committed",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	if err != nil {
		return m, err
	}

	return m, nil
}

func (rm *rbftMetrics) unregisterMetrics() {
	if rm.idGauge != nil {
		rm.idGauge.Unregister()
	}
	if rm.epochGauge != nil {
		rm.epochGauge.Unregister()
	}
	if rm.viewGauge != nil {
		rm.viewGauge.Unregister()
	}
	if rm.clusterSizeGauge != nil {
		rm.clusterSizeGauge.Unregister()
	}
	if rm.quorumSizeGauge != nil {
		rm.quorumSizeGauge.Unregister()
	}
	if rm.committedBlockNumber != nil {
		rm.committedBlockNumber.Unregister()
	}
	if rm.statusGaugeInNormal != nil {
		rm.statusGaugeInNormal.Unregister()
	}
	if rm.statusGaugeInConfChange != nil {
		rm.statusGaugeInConfChange.Unregister()
	}
	if rm.statusGaugeInViewChange != nil {
		rm.statusGaugeInViewChange.Unregister()
	}
	if rm.statusGaugeInRecovery != nil {
		rm.statusGaugeInRecovery.Unregister()
	}
	if rm.statusGaugeStateTransferring != nil {
		rm.statusGaugeStateTransferring.Unregister()
	}
	if rm.statusGaugePoolFull != nil {
		rm.statusGaugePoolFull.Unregister()
	}
	if rm.statusGaugePending != nil {
		rm.statusGaugePending.Unregister()
	}
	if rm.statusGaugeInconsistent != nil {
		rm.statusGaugeInconsistent.Unregister()
	}

	if rm.committedConfigBlockNumber != nil {
		rm.committedConfigBlockNumber.Unregister()
	}
	if rm.committedEmptyBlockNumber != nil {
		rm.committedEmptyBlockNumber.Unregister()
	}
	if rm.committedTxs != nil {
		rm.committedTxs.Unregister()
	}
	if rm.txsPerBlock != nil {
		rm.txsPerBlock.Unregister()
	}
	if rm.batchInterval != nil {
		rm.batchInterval.Unregister()
	}
	if rm.minBatchIntervalDuration != nil {
		rm.minBatchIntervalDuration.Unregister()
	}
	if rm.batchPersistDuration != nil {
		rm.batchPersistDuration.Unregister()
	}
	if rm.batchToCommitDuration != nil {
		rm.batchToCommitDuration.Unregister()
	}
	if rm.processEventDuration != nil {
		rm.processEventDuration.Unregister()
	}
	if rm.batchesGauge != nil {
		rm.batchesGauge.Unregister()
	}
	if rm.outstandingBatchesGauge != nil {
		rm.outstandingBatchesGauge.Unregister()
	}
	if rm.cacheBatchNumber != nil {
		rm.cacheBatchNumber.Unregister()
	}
	if rm.stateUpdateCounter != nil {
		rm.stateUpdateCounter.Unregister()
	}
	if rm.fetchMissingTxsCounter != nil {
		rm.fetchMissingTxsCounter.Unregister()
	}
	if rm.returnFetchMissingTxsCounter != nil {
		rm.returnFetchMissingTxsCounter.Unregister()
	}
	if rm.fetchRequestBatchCounter != nil {
		rm.fetchRequestBatchCounter.Unregister()
	}
	if rm.incomingLocalTxSets != nil {
		rm.incomingLocalTxSets.Unregister()
	}
	if rm.incomingLocalTxs != nil {
		rm.incomingLocalTxs.Unregister()
	}
	if rm.rejectedLocalTxs != nil {
		rm.rejectedLocalTxs.Unregister()
	}
	if rm.incomingRemoteTxSets != nil {
		rm.incomingRemoteTxSets.Unregister()
	}
	if rm.incomingRemoteTxs != nil {
		rm.incomingRemoteTxs.Unregister()
	}
	if rm.rejectedRemoteTxs != nil {
		rm.rejectedRemoteTxs.Unregister()
	}
	if rm.batchToPrePrepared != nil {
		rm.batchToPrePrepared.Unregister()
	}
	if rm.prePreparedToPrepared != nil {
		rm.prePreparedToPrepared.Unregister()
	}
	if rm.preparedToCommitted != nil {
		rm.preparedToCommitted.Unregister()
	}
}
