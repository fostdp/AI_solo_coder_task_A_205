package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"sail-simulation/pkg/aerodynamics"
	"sail-simulation/pkg/models"
	"sail-simulation/pkg/optimizer"
	"sail-simulation/pkg/storage"
	ws "sail-simulation/pkg/websocket"
)

type Server struct {
	store         *storage.Storage
	hub           *ws.Hub
	aeroSolver    *aerodynamics.AerodynamicSolver
	optimizer     *optimizer.GradientDescentOptimizer
	udpAddr       string
	httpAddr      string
	thresholds    map[string]models.AlertThreshold
	alertCooldown map[string]time.Time
}

func NewServer() (*Server, error) {
	connStr := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/sail_simulation?sslmode=disable")
	store, err := storage.NewStorage(connStr)
	if err != nil {
		log.Printf("Warning: Failed to connect to database: %v", err)
		log.Println("Running in memory-only mode")
	}

	return &Server{
		store:         store,
		hub:           ws.NewHub(),
		aeroSolver:    aerodynamics.NewAerodynamicSolver(),
		udpAddr:       getEnv("UDP_ADDR", ":8001"),
		httpAddr:      getEnv("HTTP_ADDR", ":8080"),
		thresholds:    make(map[string]models.AlertThreshold),
		alertCooldown: make(map[string]time.Time),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func (s *Server) loadThresholds(ctx context.Context) {
	if s.store == nil {
		s.thresholds["stall_angle"] = models.AlertThreshold{
			ParameterName:     "stall_angle",
			WarningThreshold:  18.0,
			CriticalThreshold: 25.0,
		}
		s.thresholds["min_ship_speed"] = models.AlertThreshold{
			ParameterName:     "min_ship_speed",
			WarningThreshold:  1.0,
			CriticalThreshold: 0.5,
		}
		s.thresholds["max_wind_speed"] = models.AlertThreshold{
			ParameterName:     "max_wind_speed",
			WarningThreshold:  20.0,
			CriticalThreshold: 30.0,
		}
		return
	}

	thresholds, err := s.store.GetAlertThresholds(ctx)
	if err != nil {
		log.Printf("Failed to load thresholds: %v", err)
		return
	}
	for _, t := range thresholds {
		s.thresholds[t.ParameterName] = t
	}
}

func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.loadThresholds(ctx)
	s.optimizer = optimizer.NewGradientDescentOptimizer(s.aeroSolver)

	go s.hub.Run()
	log.Println("WebSocket hub started")

	go s.startUDPServer(ctx)
	log.Printf("UDP server listening on %s", s.udpAddr)

	s.startHTTPServer(ctx)

	return nil
}

func (s *Server) startUDPServer(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp", s.udpAddr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to listen UDP: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 2048)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("UDP read error from %v: %v", remoteAddr, err)
			continue
		}

		s.handleUDPData(ctx, buf[:n])
	}
}

func (s *Server) handleUDPData(ctx context.Context, data []byte) {
	var sensor models.SensorData
	if err := json.Unmarshal(data, &sensor); err != nil {
		log.Printf("Failed to parse UDP data: %v", err)
		return
	}

	if sensor.Time.IsZero() {
		sensor.Time = time.Now()
	}
	if sensor.AirDensity == 0 {
		sensor.AirDensity = 1.225
	}
	if sensor.AmbientTemp == 0 {
		sensor.AmbientTemp = 25.0
	}

	log.Printf("Received sensor data: ship=%d sail=%d wind=%.1f angle=%.1f speed=%.2f",
		sensor.ShipID, sensor.SailID, sensor.WindSpeed, sensor.SailAngle, sensor.ShipSpeed)

	if s.store != nil {
		if err := s.store.InsertSensorData(ctx, &sensor); err != nil {
			log.Printf("Failed to insert sensor data: %v", err)
		}
	}

	s.hub.BroadcastSensorData(&sensor)

	s.processAerodynamics(ctx, &sensor)
	s.checkAlerts(ctx, &sensor)
	s.runOptimization(ctx, &sensor)
}

func (s *Server) processAerodynamics(ctx context.Context, sensor *models.SensorData) {
	sail, err := s.getSail(ctx, sensor.SailID)
	if err != nil || sail == nil {
		log.Printf("Failed to get sail %d: %v", sensor.SailID, err)
		return
	}

	result := s.aeroSolver.Solve(sail, sensor)

	if s.store != nil {
		if err := s.store.InsertAerodynamicResult(ctx, result); err != nil {
			log.Printf("Failed to insert aero result: %v", err)
		}
	}

	s.hub.BroadcastAerodynamicResult(result)

	if result.IsStalled {
		s.triggerStallAlert(ctx, sensor, result)
	}

	log.Printf("Aero result: AoA=%.1f Cl=%.4f Cd=%.4f Lift=%.1fN Drag=%.1fN Stalled=%v",
		result.AngleOfAttack, result.LiftCoefficient, result.DragCoefficient,
		result.LiftForce, result.DragForce, result.IsStalled)
}

func (s *Server) runOptimization(ctx context.Context, sensor *models.SensorData) {
	ship, err := s.getShip(ctx, sensor.ShipID)
	if err != nil || ship == nil {
		return
	}
	sail, err := s.getSail(ctx, sensor.SailID)
	if err != nil || sail == nil {
		return
	}

	result := s.optimizer.OptimizeSailAngle(ship, sail, sensor)

	if s.store != nil {
		if err := s.store.InsertOptimizationResult(ctx, result); err != nil {
			log.Printf("Failed to insert optimization result: %v", err)
		}
	}

	s.hub.BroadcastOptimizationResult(result)

	log.Printf("Optimization: initial=%.1f optimal=%.1f speed increase=%.2f%% iterations=%d",
		result.InitialSailAngle, result.OptimalSailAngle, result.SpeedIncrease*100, result.Iterations)
}

func (s *Server) checkAlerts(ctx context.Context, sensor *models.SensorData) {
	if t, ok := s.thresholds["min_ship_speed"]; ok {
		if sensor.ShipSpeed < t.WarningThreshold {
			severity := "warning"
			threshold := t.WarningThreshold
			if sensor.ShipSpeed < t.CriticalThreshold {
				severity = "critical"
				threshold = t.CriticalThreshold
			}
			s.sendAlert(ctx, &models.AlertEvent{
				Time:           time.Now(),
				ShipID:         sensor.ShipID,
				SailID:         &sensor.SailID,
				AlertType:      "low_speed",
				Severity:       severity,
				Message:        fmt.Sprintf("航速过低: %.2f m/s", sensor.ShipSpeed),
				CurrentValue:   &sensor.ShipSpeed,
				ThresholdValue: &threshold,
			})
		}
	}

	if t, ok := s.thresholds["max_wind_speed"]; ok {
		if sensor.WindSpeed > t.WarningThreshold {
			severity := "warning"
			threshold := t.WarningThreshold
			if sensor.WindSpeed > t.CriticalThreshold {
				severity = "critical"
				threshold = t.CriticalThreshold
			}
			s.sendAlert(ctx, &models.AlertEvent{
				Time:           time.Now(),
				ShipID:         sensor.ShipID,
				SailID:         &sensor.SailID,
				AlertType:      "high_wind",
				Severity:       severity,
				Message:        fmt.Sprintf("风速过高: %.1f m/s", sensor.WindSpeed),
				CurrentValue:   &sensor.WindSpeed,
				ThresholdValue: &threshold,
			})
		}
	}
}

func (s *Server) triggerStallAlert(ctx context.Context, sensor *models.SensorData, aero *models.AerodynamicResult) {
	t, ok := s.thresholds["stall_angle"]
	aoaAbs := math.Abs(aero.AngleOfAttack)

	severity := "warning"
	threshold := 18.0
	if ok {
		threshold = t.WarningThreshold
		if aoaAbs > t.CriticalThreshold {
			severity = "critical"
			threshold = t.CriticalThreshold
		}
	}

	s.sendAlert(ctx, &models.AlertEvent{
		Time:           time.Now(),
		ShipID:         sensor.ShipID,
		SailID:         &sensor.SailID,
		AlertType:      "stall",
		Severity:       severity,
		Message:        fmt.Sprintf("帆面失速预警: 攻角=%.1f° Cl=%.3f", aero.AngleOfAttack, aero.LiftCoefficient),
		CurrentValue:   &aero.AngleOfAttack,
		ThresholdValue: &threshold,
	})
}

func (s *Server) sendAlert(ctx context.Context, alert *models.AlertEvent) {
	key := fmt.Sprintf("%d_%d_%s", alert.ShipID, *alert.SailID, alert.AlertType)
	if last, ok := s.alertCooldown[key]; ok {
		if time.Since(last) < 30*time.Second {
			return
		}
	}
	s.alertCooldown[key] = time.Now()

	if s.store != nil {
		if err := s.store.InsertAlertEvent(ctx, alert); err != nil {
			log.Printf("Failed to insert alert: %v", err)
		}
	}
	s.hub.BroadcastAlert(alert)
	log.Printf("ALERT [%s] %s: %s", alert.Severity, alert.AlertType, alert.Message)
}

func (s *Server) getSail(ctx context.Context, sailID int) (*models.Sail, error) {
	if s.store == nil {
		return &models.Sail{
			SailID:      sailID,
			ShipID:      1,
			SailName:    "主桅帆",
			Area:        150.0,
			AspectRatio: 2.8,
			ChordLength: 7.3,
			SpanLength:  20.5,
			Camber:      0.13,
		}, nil
	}
	return s.store.GetSailByID(ctx, sailID)
}

func (s *Server) getShip(ctx context.Context, shipID int) (*models.Ship, error) {
	if s.store == nil {
		return &models.Ship{
			ShipID:       shipID,
			ShipName:     "南海一号",
			HullLength:   30.5,
			HullWidth:    9.8,
			Displacement: 600.0,
		}, nil
	}
	return s.store.GetShipByID(ctx, shipID)
}

func (s *Server) startHTTPServer(ctx context.Context) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", s.hub.HandleConnection)

	mux.HandleFunc("/api/ships", func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		ships, err := s.store.GetShips(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(ships)
	})

	mux.HandleFunc("/api/sails", func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		shipID, _ := strconv.Atoi(r.URL.Query().Get("ship_id"))
		sails, err := s.store.GetSailsByShip(r.Context(), shipID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(sails)
	})

	mux.HandleFunc("/api/sensor-data", func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		shipID, _ := strconv.Atoi(r.URL.Query().Get("ship_id"))
		sailID, _ := strconv.Atoi(r.URL.Query().Get("sail_id"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 100
		}
		data, err := s.store.GetRecentSensorData(r.Context(), shipID, sailID, limit)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(data)
	})

	mux.HandleFunc("/api/aero-results", func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		shipID, _ := strconv.Atoi(r.URL.Query().Get("ship_id"))
		sailID, _ := strconv.Atoi(r.URL.Query().Get("sail_id"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 100
		}
		data, err := s.store.GetRecentAeroResults(r.Context(), shipID, sailID, limit)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(data)
	})

	mux.HandleFunc("/api/optimizations", func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		shipID, _ := strconv.Atoi(r.URL.Query().Get("ship_id"))
		sailID, _ := strconv.Atoi(r.URL.Query().Get("sail_id"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 100
		}
		data, err := s.store.GetRecentOptimizations(r.Context(), shipID, sailID, limit)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(data)
	})

	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		shipID, _ := strconv.Atoi(r.URL.Query().Get("ship_id"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 50
		}
		data, err := s.store.GetRecentAlerts(r.Context(), shipID, limit)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(data)
	})

	mux.HandleFunc("/api/polar-curve", func(w http.ResponseWriter, r *http.Request) {
		sailID, _ := strconv.Atoi(r.URL.Query().Get("sail_id"))
		windSpeed, _ := strconv.ParseFloat(r.URL.Query().Get("wind_speed"), 64)
		if windSpeed == 0 {
			windSpeed = 10
		}
		sail := &models.Sail{
			SailID:      sailID,
			Area:        150.0,
			AspectRatio: 2.8,
			ChordLength: 7.3,
			SpanLength:  20.5,
			Camber:      0.13,
		}
		polars := s.aeroSolver.GeneratePolarCurve(sail, windSpeed)
		json.NewEncoder(w).Encode(polars)
	})

	mux.HandleFunc("/api/optimize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		var req struct {
			ShipID        int     `json:"ship_id"`
			SailID        int     `json:"sail_id"`
			WindSpeed     float64 `json:"wind_speed"`
			WindDirection float64 `json:"wind_direction"`
			SailAngle     float64 `json:"sail_angle"`
			ShipSpeed     float64 `json:"ship_speed"`
			Heading       float64 `json:"heading"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		ship := &models.Ship{
			ShipID:       req.ShipID,
			HullLength:   30.5,
			HullWidth:    9.8,
			Displacement: 600.0,
		}
		sail := &models.Sail{
			SailID:      req.SailID,
			ShipID:      req.ShipID,
			Area:        150.0,
			AspectRatio: 2.8,
			ChordLength: 7.3,
			SpanLength:  20.5,
			Camber:      0.13,
		}
		sensor := &models.SensorData{
			Time:          time.Now(),
			ShipID:        req.ShipID,
			SailID:        req.SailID,
			WindSpeed:     req.WindSpeed,
			WindDirection: req.WindDirection,
			SailAngle:     req.SailAngle,
			ShipSpeed:     req.ShipSpeed,
			Heading:       req.Heading,
			AirDensity:    1.225,
		}
		result := s.optimizer.OptimizeSailAngle(ship, sail, sensor)
		json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/api/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		var sensor models.SensorData
		if err := json.NewDecoder(r.Body).Decode(&sensor); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if sensor.Time.IsZero() {
			sensor.Time = time.Now()
		}
		s.handleUDPData(r.Context(), func() []byte { b, _ := json.Marshal(sensor); return b }())
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	fs := http.FileServer(http.Dir("../frontend"))
	mux.Handle("/", fs)

	server := &http.Server{
		Addr:    s.httpAddr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Printf("HTTP server listening on %s", s.httpAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Sail Simulation Backend...")

	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
