package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"sail-simulation/pkg/aerodynamics"
	"sail-simulation/pkg/config"
	"sail-simulation/pkg/models"
	"sail-simulation/pkg/modules"
	"sail-simulation/pkg/optimizer"
	"sail-simulation/pkg/storage"
	ws "sail-simulation/pkg/websocket"
)

type Server struct {
	store      *storage.Storage
	hub        *ws.Hub
	aeroSolver *aerodynamics.AerodynamicSolver
	udpAddr    string
	httpAddr   string
	thresholds map[string]models.AlertThreshold
	aeroCfg    *config.AerodynamicsConfig
	optCfg     *config.OptimizerConfig
	alarmWS    *modules.AlarmWS
}

func NewServer() (*Server, error) {
	connStr := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/sail_simulation?sslmode=disable")
	store, err := storage.NewStorage(connStr)
	if err != nil {
		log.Printf("Warning: Failed to connect to database: %v", err)
		log.Println("Running in memory-only mode")
	}

	aeroCfg, err := config.LoadAerodynamicsConfig("../config/aerodynamics.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load aerodynamics config: %w", err)
	}
	optCfg, err := config.LoadOptimizerConfig("../config/optimizer.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load optimizer config: %w", err)
	}

	return &Server{
		store:      store,
		hub:        ws.NewHub(),
		aeroSolver: aerodynamics.NewAerodynamicSolver(
			&models.Sail{
				Area:        aeroCfg.DefaultSail.Area,
				AspectRatio: aeroCfg.DefaultSail.AspectRatio,
				ChordLength: aeroCfg.DefaultSail.ChordLength,
				SpanLength:  aeroCfg.DefaultSail.SpanLength,
				Camber:      aeroCfg.DefaultSail.Camber,
			},
			aeroCfg.VortexLattice.ChordwisePanels,
			aeroCfg.VortexLattice.SpanwisePanels,
			aeroCfg.BoundaryLayer.TransitionReynolds,
		),
		udpAddr:    getEnv("UDP_ADDR", ":8001"),
		httpAddr:   getEnv("HTTP_ADDR", ":8080"),
		thresholds: make(map[string]models.AlertThreshold),
		aeroCfg:    aeroCfg,
		optCfg:     optCfg,
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

	sensorRawCh := make(chan *models.SensorData, 100)
	validatedCh := make(chan *modules.ValidatedSensorData, 100)
	aeroReqCh := make(chan *modules.AeroSimRequest, 50)
	aeroResCh := make(chan *modules.AeroSimResult, 50)
	optReqCh := make(chan *modules.OptimizationRequest, 50)
	optResCh := make(chan *modules.OptimizationResult, 50)
	alertCh := make(chan *modules.AlertEvent, 20)

	udpReceiver := modules.NewUDPReceiver(s.udpAddr, sensorRawCh, validatedCh)
	if err := udpReceiver.Start(ctx); err != nil {
		return fmt.Errorf("failed to start UDP receiver: %w", err)
	}

	aeroSim := modules.NewAerodynamicsSimulator(aeroReqCh, aeroResCh, s.aeroCfg)
	go aeroSim.Run(ctx)

	sailOpt := modules.NewSailOptimizer(optReqCh, optResCh, s.optCfg, s.aeroCfg)
	go sailOpt.Run(ctx)

	s.alarmWS = modules.NewAlarmWS(alertCh, s.hub, 30*time.Second)
	go s.alarmWS.Run(ctx)
	go s.alarmWS.CleanupCooldowns(ctx)

	go s.hub.Run()
	log.Println("WebSocket hub started")

	go s.orchestrator(ctx, sensorRawCh, aeroReqCh, optReqCh, aeroResCh, optResCh, alertCh)
	log.Println("Pipeline orchestrator started")

	s.startHTTPServer(ctx, alertCh)

	return nil
}

func (s *Server) orchestrator(
	ctx context.Context,
	sensorRawCh <-chan *models.SensorData,
	aeroReqCh chan<- *modules.AeroSimRequest,
	optReqCh chan<- *modules.OptimizationRequest,
	aeroResCh <-chan *modules.AeroSimResult,
	optResCh <-chan *modules.OptimizationResult,
	alertCh chan<- *modules.AlertEvent,
) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Orchestrator stopping")
			return

		case sensor := <-sensorRawCh:
			log.Printf("Pipeline: received sensor ship=%d sail=%d", sensor.ShipID, sensor.SailID)

			if s.store != nil {
				if err := s.store.InsertSensorData(ctx, sensor); err != nil {
					log.Printf("Failed to insert sensor data: %v", err)
				}
			}

			s.alarmWS.BroadcastSensor(sensor)

			sail := s.getSail(ctx, sensor.SailID)
			select {
			case aeroReqCh <- &modules.AeroSimRequest{
				Sensor: sensor,
				Sail:   sail,
			}:
			case <-ctx.Done():
				return
			}

			ship := s.getShip(ctx, sensor.ShipID)
			select {
			case optReqCh <- &modules.OptimizationRequest{
				Sensor: sensor,
				Ship:   ship,
				Sail:   sail,
			}:
			case <-ctx.Done():
				return
			}

			s.checkSensorAlerts(ctx, sensor, alertCh)

		case aeroRes := <-aeroResCh:
			if aeroRes.Error != nil {
				log.Printf("Aero simulation error: %v", aeroRes.Error)
				continue
			}
			log.Printf("Pipeline: aero result ship=%d sail=%d Cl=%.3f Cd=%.3f",
				aeroRes.Sensor.ShipID, aeroRes.Sensor.SailID,
				aeroRes.Result.LiftCoefficient, aeroRes.Result.DragCoefficient)

			if s.store != nil {
				if err := s.store.InsertAerodynamicResult(ctx, aeroRes.Result); err != nil {
					log.Printf("Failed to insert aero result: %v", err)
				}
			}

			s.alarmWS.BroadcastAero(aeroRes.Result)

			if aeroRes.Result.IsStalled {
				s.triggerStallAlert(ctx, aeroRes.Sensor, aeroRes.Result, alertCh)
			}

		case optRes := <-optResCh:
			if optRes.Error != nil {
				log.Printf("Optimization error: %v", optRes.Error)
				continue
			}
			log.Printf("Pipeline: opt result ship=%d sail=%d optimal=%.1f° gain=%.1f%%",
				optRes.Request.Sensor.ShipID, optRes.Request.Sensor.SailID,
				optRes.Result.OptimalAngle, optRes.Result.EfficiencyGain)

			if s.store != nil {
				if err := s.store.InsertOptimizationResult(ctx, optRes.Result); err != nil {
					log.Printf("Failed to insert opt result: %v", err)
				}
			}

			s.alarmWS.BroadcastOptimization(optRes.Result)
		}
	}
}

func (s *Server) checkSensorAlerts(ctx context.Context, sensor *models.SensorData, alertCh chan<- *modules.AlertEvent) {
	if t, ok := s.thresholds["min_ship_speed"]; ok {
		if sensor.ShipSpeed < t.WarningThreshold {
			severity := "warning"
			threshold := t.WarningThreshold
			if sensor.ShipSpeed < t.CriticalThreshold {
				severity = "critical"
				threshold = t.CriticalThreshold
			}
			alert := &models.AlertEvent{
				Time:           time.Now(),
				ShipID:         sensor.ShipID,
				SailID:         &sensor.SailID,
				AlertType:      "low_speed",
				Severity:       severity,
				Message:        fmt.Sprintf("航速过低: %.2f m/s", sensor.ShipSpeed),
				CurrentValue:   &sensor.ShipSpeed,
				ThresholdValue: &threshold,
			}
			select {
			case alertCh <- &modules.AlertEvent{Alert: alert, Cooldown: true}:
			case <-ctx.Done():
			}
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
			alert := &models.AlertEvent{
				Time:           time.Now(),
				ShipID:         sensor.ShipID,
				SailID:         &sensor.SailID,
				AlertType:      "high_wind",
				Severity:       severity,
				Message:        fmt.Sprintf("风速过高: %.1f m/s", sensor.WindSpeed),
				CurrentValue:   &sensor.WindSpeed,
				ThresholdValue: &threshold,
			}
			select {
			case alertCh <- &modules.AlertEvent{Alert: alert, Cooldown: true}:
			case <-ctx.Done():
			}
		}
	}
}

func (s *Server) triggerStallAlert(ctx context.Context, sensor *models.SensorData, aero *models.AerodynamicResult, alertCh chan<- *modules.AlertEvent) {
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

	alert := &models.AlertEvent{
		Time:           time.Now(),
		ShipID:         sensor.ShipID,
		SailID:         &sensor.SailID,
		AlertType:      "stall",
		Severity:       severity,
		Message:        fmt.Sprintf("帆面失速预警: 攻角=%.1f° Cl=%.3f", aero.AngleOfAttack, aero.LiftCoefficient),
		CurrentValue:   &aero.AngleOfAttack,
		ThresholdValue: &threshold,
	}

	if s.store != nil {
		if err := s.store.InsertAlertEvent(ctx, alert); err != nil {
			log.Printf("Failed to insert alert: %v", err)
		}
	}

	select {
	case alertCh <- &modules.AlertEvent{Alert: alert, Cooldown: true}:
	case <-ctx.Done():
	}
}

func (s *Server) getSail(ctx context.Context, sailID int) *models.Sail {
	if s.store == nil {
		return &models.Sail{
			ID:          sailID,
			ShipID:      1,
			Name:        "主桅帆",
			Area:        s.aeroCfg.DefaultSail.Area,
			AspectRatio: s.aeroCfg.DefaultSail.AspectRatio,
			ChordLength: s.aeroCfg.DefaultSail.ChordLength,
			SpanLength:  s.aeroCfg.DefaultSail.SpanLength,
			Camber:      s.aeroCfg.DefaultSail.Camber,
		}
	}
	sail, _ := s.store.GetSailByID(ctx, sailID)
	return sail
}

func (s *Server) getShip(ctx context.Context, shipID int) *models.Ship {
	if s.store == nil {
		return &models.Ship{
			ID:           shipID,
			Name:         "南海一号",
			HullLength:   s.aeroCfg.DefaultShip.HullLength,
			HullWidth:    s.aeroCfg.DefaultShip.HullWidth,
			Displacement: s.aeroCfg.DefaultShip.Displacement,
		}
	}
	ship, _ := s.store.GetShipByID(ctx, shipID)
	return ship
}

func (s *Server) startHTTPServer(ctx context.Context, alertCh chan<- *modules.AlertEvent) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", s.hub.HandleConnection)

	mux.HandleFunc("/api/ships", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		sailID, _ := strconv.Atoi(r.URL.Query().Get("sail_id"))
		windSpeed, _ := strconv.ParseFloat(r.URL.Query().Get("wind_speed"), 64)
		if windSpeed == 0 {
			windSpeed = 10
		}
		sail := &models.Sail{
			ID:          sailID,
			Area:        s.aeroCfg.DefaultSail.Area,
			AspectRatio: s.aeroCfg.DefaultSail.AspectRatio,
			ChordLength: s.aeroCfg.DefaultSail.ChordLength,
			SpanLength:  s.aeroCfg.DefaultSail.SpanLength,
			Camber:      s.aeroCfg.DefaultSail.Camber,
		}
		polars := s.aeroSolver.GeneratePolarCurve(sail, windSpeed)
		json.NewEncoder(w).Encode(polars)
	})

	mux.HandleFunc("/api/optimize", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		ship := s.getShip(ctx, req.ShipID)
		sail := s.getSail(ctx, req.SailID)
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

		opt := optimizer.NewGradientDescentOptimizer(sail, ship, s.optCfg.Adam.InitialLearningRate)
		optResult, _ := opt.OptimizeSailAngle(req.WindSpeed, req.WindDirection, req.ShipSpeed, 1.225, s.aeroCfg.Stall.StallAngleDeg)
		predictedSpeed := optimizer.PredictShipSpeed(sail, ship, sensor, optResult.OptimalAngle, s.aeroCfg.Stall.StallAngleDeg)

		result := &models.OptimizationResult{
			ShipID:           req.ShipID,
			SailID:           req.SailID,
			Time:             time.Now(),
			CurrentAngle:     req.SailAngle,
			OptimalAngle:     optResult.OptimalAngle,
			PredictedSpeed:   predictedSpeed,
			PredictedLift:    optResult.OptimalLift,
			PredictedDrag:    optResult.OptimalDrag,
			PredictedThrust:  optimizer.CalculateThrust(sail, sensor, optResult.OptimalAngle, s.aeroCfg.Stall.StallAngleDeg),
			Iterations:       optResult.Iterations,
			ConvergenceValue: optResult.ConvergenceValue,
			Converged:        optResult.Converged,
			LearningRate:     optResult.FinalLearningRate,
			AngleAdjustment:  optResult.OptimalAngle - req.SailAngle,
			EfficiencyGain:   math.Max(0, (predictedSpeed-req.ShipSpeed)/math.Max(req.ShipSpeed, 0.1)*100),
		}
		json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/api/ingest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		if sensor.AirDensity == 0 {
			sensor.AirDensity = 1.225
		}
		if sensor.AmbientTemp == 0 {
			sensor.AmbientTemp = 25.0
		}
		log.Printf("HTTP ingest: ship=%d sail=%d wind=%.1f", sensor.ShipID, sensor.SailID, sensor.WindSpeed)
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
	log.Println("Starting Sail Simulation Backend (Modular Architecture)...")

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
