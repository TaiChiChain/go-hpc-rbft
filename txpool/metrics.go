package txpool

import (
	"github.com/hyperchain/go-hpc-rbft/common/metrics"
)

// txPoolMetrics defines all metrics needed to be collected in txpool.
type txPoolMetrics struct {
	// monitor all incoming txs, including txs from API and other nodes
	// but excluding the txs that were fetchedMissing from other nodes.
	incomingTxsCounter metrics.Counter
	// monitor all duplicate txs detected in txpool.
	duplicateTxsCounter metrics.Counter
	// monitor all non-batched txs in txpool, should be 0 if consensus.
	// doesn't make any progress
	nonBatchedTxsNumber metrics.Gauge
	// monitor all batches txs in txpool, should be equal to all tx number.
	// in batchStore
	batchedTxsNumber metrics.Gauge
	// monitor all batches in txpool, should be equal to the length of
	// batchStore.
	batchNumber metrics.Gauge
}

func newTxPoolMetrics(metricsProv metrics.Provider) (*txPoolMetrics, error) {
	var err error
	m := &txPoolMetrics{}

	m.incomingTxsCounter, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "incoming_txs",
			Help: "totally incoming txs counter",
		},
	)
	if err != nil {
		return m, err
	}

	m.duplicateTxsCounter, err = metricsProv.NewCounter(
		metrics.CounterOpts{
			Name: "duplicate_txs",
			Help: "duplicate txs counter found in txpool",
		},
	)
	if err != nil {
		return m, err
	}

	m.nonBatchedTxsNumber, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "nonBatched_txs",
			Help: "currently non-batched txs",
		},
	)
	if err != nil {
		return m, err
	}

	m.batchedTxsNumber, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "batched_txs",
			Help: "currently batched txs",
		},
	)
	if err != nil {
		return m, err
	}

	m.batchNumber, err = metricsProv.NewGauge(
		metrics.GaugeOpts{
			Name: "batches",
			Help: "currently batch number",
		},
	)
	if err != nil {
		return m, err
	}

	return m, err
}

func (tpm *txPoolMetrics) unregisterMetrics() {
	if tpm.incomingTxsCounter != nil {
		tpm.incomingTxsCounter.Unregister()
	}
	if tpm.nonBatchedTxsNumber != nil {
		tpm.nonBatchedTxsNumber.Unregister()
	}
	if tpm.duplicateTxsCounter != nil {
		tpm.duplicateTxsCounter.Unregister()
	}
	if tpm.batchNumber != nil {
		tpm.batchNumber.Unregister()
	}
	if tpm.batchedTxsNumber != nil {
		tpm.batchedTxsNumber.Unregister()
	}
}
