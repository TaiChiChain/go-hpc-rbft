package rbft

import "github.com/ultramesh/flato-common/metrics"

// rbftMetrics helps collect all metrics in rbft
type rbftMetrics struct {
	// ========================== metrics related to basic RBFT info ==========================
	// track the current epoch number.
	epochGauge metrics.Gauge
	// track the current view number.
	viewGauge metrics.Gauge
	// track the current cluster size.
	clusterSizeGauge metrics.Gauge
	// track the current quorum size.
	quorumSizeGauge metrics.Gauge

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
	txsPerBlock metrics.Histogram

	// ========================== metrics related to batch duration info ==========================
	// monitor the batch persist time.
	batchPersistDuration metrics.Histogram
	// monitor the time from batch[primary] to commit.
	// may be negative due to the time difference between primary and replica.
	batchToCommitDuration metrics.Histogram

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
	// monitor the times of fetch request batch which is caused by missing batches after
	// vc/recovery.
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
}

func newRBFTMetrics(metricsProv metrics.Provider) *rbftMetrics {
	m := &rbftMetrics{}

	m.epochGauge, _ = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "epoch",
			Help: "rbft epoch number",
		},
	)

	m.viewGauge, _ = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "view",
			Help: "rbft view number",
		},
	)

	m.clusterSizeGauge, _ = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "cluster_size",
			Help: "rbft cluster size",
		},
	)

	m.quorumSizeGauge, _ = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "quorum_size",
			Help: "rbft quorum size",
		},
	)

	m.committedBlockNumber, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_block_number",
			Help: "rbft committed block number",
		},
	)

	m.committedConfigBlockNumber, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_config_block_number",
			Help: "rbft committed config block number",
		},
	)

	m.committedEmptyBlockNumber, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_empty_block_number",
			Help: "rbft committed empty block number",
		},
	)

	m.committedTxs, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "committed_txs",
			Help: "rbft committed tx number",
		},
	)

	m.txsPerBlock, _ = metricsProv.NewHistogram(
		metrics.HistogramOpts{
			Name:    "committed_txs_per_block",
			Help:    "rbft committed tx number per block",
			Buckets: []float64{0, 10, 50, 100, 200, 300, 400, 500},
		},
	)

	m.batchPersistDuration, _ = metricsProv.NewHistogram(
		metrics.HistogramOpts{
			Name:    "batch_persist_duration",
			Help:    "persist duration of batch",
			Buckets: []float64{0.001, 0.003, 0.005, 0.008, 0.01, 0.02, 0.1},
		},
	)

	m.batchToCommitDuration, _ = metricsProv.NewHistogram(
		metrics.HistogramOpts{
			Name:    "batch_to_commit_duration",
			Help:    "duration from batch to commit",
			Buckets: []float64{0.001, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 0.5, 1, 3, 5, 10},
		},
	)

	m.batchesGauge, _ = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "batch_number",
			Help: "rbft batch number cached in batchStore",
		},
	)

	m.outstandingBatchesGauge, _ = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "outstanding_batch_number",
			Help: "rbft outstanding batch number",
		},
	)

	m.cacheBatchNumber, _ = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "cache_batch_number",
			Help: "rbft cache batch number",
		},
	)

	m.stateUpdateCounter, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "state_update_times",
			Help: "rbft state update times",
		},
	)

	m.fetchMissingTxsCounter, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "fetch_missing_txs_times",
			Help: "rbft fetch missing txs times",
		},
	)

	m.fetchRequestBatchCounter, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "fetch_request_batch_times",
			Help: "rbft fetch request batch times",
		},
	)

	m.incomingLocalTxSets, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_local_tx_sets",
			Help: "rbft incoming local tx sets from API or NVP",
		},
	)

	m.incomingLocalTxs, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_local_txs",
			Help: "rbft incoming local txs from API or NVP",
		},
	)

	m.rejectedLocalTxs, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "rejected_local_txs",
			Help: "rbft rejected local txs from API or NVP",
		},
	)

	m.incomingRemoteTxSets, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_remote_tx_sets",
			Help: "rbft incoming remote tx sets from other VP",
		},
	)

	m.incomingRemoteTxs, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_remote_txs",
			Help: "rbft incoming remote txs from other VP",
		},
	)

	m.rejectedRemoteTxs, _ = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "rejected_remote_txs",
			Help: "rbft rejected remote txs from other VP",
		},
	)

	return m
}
