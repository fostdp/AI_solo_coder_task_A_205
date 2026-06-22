package metrics

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	SensorReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sail_sensor_received_total",
			Help: "Total number of sensor data packets received",
		},
		[]string{"transport"},
	)

	SensorValidated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "sail_sensor_validated_total",
			Help: "Total number of sensor data packets that passed validation",
		},
	)

	SensorRejected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sail_sensor_rejected_total",
			Help: "Total number of sensor data packets rejected by validation",
		},
		[]string{"reason"},
	)

	AeroSimulations = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "sail_aero_simulations_total",
			Help: "Total number of aerodynamic simulations completed",
		},
	)

	AeroSimDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "sail_aero_simulation_duration_seconds",
			Help:    "Duration of aerodynamic simulation in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	Optimizations = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "sail_optimizations_total",
			Help: "Total number of sail angle optimizations completed",
		},
	)

	OptimizationDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "sail_optimization_duration_seconds",
			Help:    "Duration of sail optimization in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	AlertsTriggered = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sail_alerts_triggered_total",
			Help: "Total number of alerts triggered",
		},
		[]string{"type", "severity"},
	)

	AlertsSuppressed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "sail_alerts_suppressed_total",
			Help: "Total number of alerts suppressed by cooldown",
		},
	)

	WebSocketConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sail_websocket_connections",
			Help: "Current number of active WebSocket connections",
		},
	)

	WebSocketMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sail_websocket_messages_total",
			Help: "Total number of WebSocket messages sent",
		},
		[]string{"type"},
	)

	DBOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sail_db_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"operation", "status"},
	)

	DBOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sail_db_operation_duration_seconds",
			Help:    "Duration of database operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	ChannelBacklog = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sail_channel_backlog",
			Help: "Current number of items in channel backlog",
		},
		[]string{"channel"},
	)

	LiftCoefficient = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sail_lift_coefficient",
			Help: "Latest lift coefficient from aerodynamic simulation",
		},
	)

	DragCoefficient = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sail_drag_coefficient",
			Help: "Latest drag coefficient from aerodynamic simulation",
		},
	)

	OptimalAngle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sail_optimal_angle_degrees",
			Help: "Latest optimal sail angle from optimization",
		},
	)

	EfficiencyGain = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sail_efficiency_gain_percent",
			Help: "Latest efficiency gain percentage from optimization",
		},
	)

	StallStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sail_stall_status",
			Help: "Stall status: 1=stalled, 0=normal",
		},
	)
)

var channelLens map[string]func() int

func RegisterChannelMetrics(name string, lenFn func() int) {
	if channelLens == nil {
		channelLens = make(map[string]func() int)
	}
	channelLens[name] = lenFn
}

func init() {
	prometheus.MustRegister(
		SensorReceived,
		SensorValidated,
		SensorRejected,
		AeroSimulations,
		AeroSimDuration,
		Optimizations,
		OptimizationDuration,
		AlertsTriggered,
		AlertsSuppressed,
		WebSocketConnections,
		WebSocketMessages,
		DBOperations,
		DBOperationDuration,
		ChannelBacklog,
		LiftCoefficient,
		DragCoefficient,
		OptimalAngle,
		EfficiencyGain,
		StallStatus,
	)
}

var httpRequests atomic.Int64

func InstrumentedHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpRequests.Add(1)
		next.ServeHTTP(w, r)
		_ = time.Since(start)
	})
}

func StartMetricsServer(addr string, ctx context.Context) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/debug/pprof/", func(w http.ResponseWriter, r *http.Request) {
		http.DefaultServeMux.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("Metrics/pprof server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	go updateChannelBacklog(ctx)
}

func updateChannelBacklog(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for name, lenFn := range channelLens {
				ChannelBacklog.WithLabelValues(name).Set(float64(lenFn()))
			}
		}
	}
}
