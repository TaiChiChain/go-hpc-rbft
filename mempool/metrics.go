package mempool

import "github.com/prometheus/client_golang/prometheus"

var (
	poolTxNum = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "mempool",
		Name:      "tx_counter",
		Help:      "the total number of transactions in mempool",
	})
)

func init() {
	prometheus.MustRegister(poolTxNum)
}
