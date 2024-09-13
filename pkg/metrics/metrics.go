package metrics

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	// Atomic counters
	UpstreamConn      = new(atomic.Int64)
	ActiveClientsConn = new(atomic.Int64)

	// Metrics
	UpstreamPullSpeed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rc_upstream_pull_speed_mbps",
			Help: "Pull speed in Mb/s per second",
		},
		[]string{"sha256", "type"},
	)
	TotalGCRuns = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rc_total_gc_run",
			Help: "total gc runs counter",
		},
	)
	EstimatedIndexSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rc_estimated_index_size_bytes",
			Help: "estimation of index size in bytes",
		},
	)
	CacheSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rc_cache_size_bytes",
			Help: "size of cache folder in bytes",
		},
	)
	FailedRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rc_failed_requests",
			Help: "failed requests",
		},
		[]string{"reason", "path"},
	)
	TotalCachedRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rc_total_cached_requests",
			Help: "total cached requests counter",
		},
		[]string{"type", "sha256"},
	)

	TotalCacheMiss = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rc_cache_miss",
			Help: "counter on cache miss",
		},
		[]string{"type", "sha256"},
	)
	TotalUpstreamActiveConn = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rc_active_upstream_conn",
			Help: "Number of upstream active connections",
		},
	)
	TotalActiveClientsConn = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rc_active_clients_conn",
			Help: "Number of active clients connections",
		},
	)
	TotalBytesServedFromCache = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rc_bytes_from_cache",
			Help: "total bytes served from cache",
		},
	)
)

func init() {
	prometheus.MustRegister(TotalGCRuns)
	prometheus.MustRegister(CacheSize)
	prometheus.MustRegister(FailedRequests)
	prometheus.MustRegister(TotalCachedRequests)
	prometheus.MustRegister(TotalCacheMiss)
	prometheus.MustRegister(TotalUpstreamActiveConn)
	prometheus.MustRegister(TotalActiveClientsConn)
	prometheus.MustRegister(TotalBytesServedFromCache)
	prometheus.MustRegister(EstimatedIndexSize)
	prometheus.MustRegister(UpstreamPullSpeed)
}

func Run(metricsAddr string, idx cache.Index) {

	// run metrics routines here
	go updateIndexSize(idx)
	go updateActiveUpstreamConns()
	go updateActiveStreamers()

	logrus.Infoln("starting metrics server on ", metricsAddr)
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(metricsAddr, nil)
}

// Gauge routines

func updateActiveUpstreamConns() {
	for {
		TotalUpstreamActiveConn.Set(float64(UpstreamConn.Load()))
		time.Sleep(time.Second * 15)
	}
}

func updateActiveStreamers() {
	for {
		TotalActiveClientsConn.Set(float64(ActiveClientsConn.Load()))
		time.Sleep(time.Second * 15)
	}
}

func updateIndexSize(idx cache.Index) {
	for {
		estimation := idx.Len() * 64 * 4
		EstimatedIndexSize.Set(float64(estimation))
		time.Sleep(time.Second * 15)
	}
}
