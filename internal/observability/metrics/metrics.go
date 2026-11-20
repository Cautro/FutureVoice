package metrics

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

func New(db *sql.DB) *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "futurevoice_http_requests_total",
			Help: "Total number of completed HTTP requests.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "futurevoice_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "futurevoice_http_in_flight_requests",
			Help: "Number of HTTP requests currently being processed.",
		}),
	}
	dbOpen := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "futurevoice_db_open_connections",
		Help: "Number of PostgreSQL connections currently open.",
	}, func() float64 { return float64(db.Stats().OpenConnections) })
	dbInUse := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "futurevoice_db_in_use_connections",
		Help: "Number of PostgreSQL connections currently in use.",
	}, func() float64 { return float64(db.Stats().InUse) })
	dbIdle := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "futurevoice_db_idle_connections",
		Help: "Number of idle PostgreSQL connections.",
	}, func() float64 { return float64(db.Stats().Idle) })
	dbWaitCount := prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "futurevoice_db_wait_count_total",
		Help: "Total number of waits for a PostgreSQL connection.",
	}, func() float64 { return float64(db.Stats().WaitCount) })

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.requests,
		m.duration,
		m.inFlight,
		dbOpen,
		dbInUse,
		dbIdle,
		dbWaitCount,
	)
	return m
}

func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		m.inFlight.Inc()
		defer m.inFlight.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())

		m.requests.WithLabelValues(method, route, status).Inc()
		m.duration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
	}
}

func (m *Metrics) Handler(username, password string) http.Handler {
	handler := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providedUser, providedPassword, ok := r.BasicAuth()
		if !ok ||
			len(providedUser) != len(username) ||
			len(providedPassword) != len(password) ||
			subtle.ConstantTimeCompare([]byte(providedUser), []byte(username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(providedPassword), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="FutureVoice metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
