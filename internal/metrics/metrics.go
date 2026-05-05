package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SnapshotsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "snapah_snapshots_total",
		Help: "Total de snapshots activos",
	})

	NodesOnline = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "snapah_nodes_online",
		Help: "Nodos actualmente online",
	})

	PoliciesActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "snapah_policies_active",
		Help: "Políticas de snapshot activas",
	})

	SnapshotCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "snapah_snapshot_created_total",
		Help: "Total de snapshots creados desde el inicio",
	})

	SnapshotDeletedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "snapah_snapshot_deleted_total",
		Help: "Total de snapshots eliminados",
	})

	ReplicationJobsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "snapah_replication_jobs_total",
		Help: "Jobs de replicación por estado",
	}, []string{"status"})

	APIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "snapah_api_request_duration_seconds",
		Help:    "Duración de requests HTTP",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	UptimeSeconds = promauto.NewCounterFunc(prometheus.CounterOpts{
		Name: "snapah_uptime_seconds_total",
		Help: "Segundos desde el inicio del servidor",
	}, func() float64 {
		return 0 // se actualiza externamente
	})
)
