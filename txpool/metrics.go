package txpool

import "github.com/prometheus/client_golang/prometheus"

var (
	poolTxNum = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "txpool",
		Name:      "tx_counter",
		Help:      "the total number of transactions",
	})
	readyTxNum = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "txpool",
		Name:      "ready_tx_counter",
		Help:      "the total number of transactions which ready to generate batch",
	})
)

func init() {
	prometheus.MustRegister(poolTxNum)
	prometheus.MustRegister(readyTxNum)
}
