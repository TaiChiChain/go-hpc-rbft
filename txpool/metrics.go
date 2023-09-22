package txpool

import "github.com/prometheus/client_golang/prometheus"

var (
	poolTxNum = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "txpool",
		Name:      "tx_counter",
		Help:      "the total number of transactions",
	})
)

func init() {
	prometheus.MustRegister(poolTxNum)
}
