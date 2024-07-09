package flags

import "github.com/prometheus/client_golang/prometheus"

var (
	ReconcileDeleteGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bucket_reconcile_delete_total",
			Help: "Number of reconcile delete events for a bucket CR.",
		},
		[]string{"bucket_name"},
	)
)

func RegisterGauge() {
	prometheus.MustRegister(ReconcileDeleteGauge)
}
