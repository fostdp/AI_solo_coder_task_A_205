package models

import "time"

type Ship struct {
	ShipID       int       `json:"ship_id"`
	ShipName     string    `json:"ship_name"`
	ShipType     string    `json:"ship_type"`
	HullLength   float64   `json:"hull_length"`
	HullWidth    float64   `json:"hull_width"`
	Displacement float64   `json:"displacement"`
	CreatedAt    time.Time `json:"created_at"`
}

type Sail struct {
	SailID       int       `json:"sail_id"`
	ShipID       int       `json:"ship_id"`
	SailName     string    `json:"sail_name"`
	SailPosition string    `json:"sail_position"`
	Area         float64   `json:"area"`
	AspectRatio  float64   `json:"aspect_ratio"`
	ChordLength  float64   `json:"chord_length"`
	SpanLength   float64   `json:"span_length"`
	Camber       float64   `json:"camber"`
	CreatedAt    time.Time `json:"created_at"`
}

type SensorData struct {
	Time          time.Time `json:"time"`
	ShipID        int       `json:"ship_id"`
	SailID        int       `json:"sail_id"`
	WindSpeed     float64   `json:"wind_speed"`
	WindDirection float64   `json:"wind_direction"`
	SailAngle     float64   `json:"sail_angle"`
	ShipSpeed     float64   `json:"ship_speed"`
	Heading       float64   `json:"heading"`
	AmbientTemp   float64   `json:"ambient_temp"`
	AirDensity    float64   `json:"air_density"`
}

type AerodynamicResult struct {
	Time                   time.Time `json:"time"`
	ShipID                 int       `json:"ship_id"`
	SailID                 int       `json:"sail_id"`
	AngleOfAttack          float64   `json:"angle_of_attack"`
	LiftCoefficient        float64   `json:"lift_coefficient"`
	DragCoefficient        float64   `json:"drag_coefficient"`
	LiftForce              float64   `json:"lift_force"`
	DragForce              float64   `json:"drag_force"`
	PressureDrag           float64   `json:"pressure_drag"`
	FrictionDrag           float64   `json:"friction_drag"`
	InducedDrag            float64   `json:"induced_drag"`
	ReynoldsNumber         float64   `json:"reynolds_number"`
	BoundaryLayerThickness float64   `json:"boundary_layer_thickness"`
	IsStalled              bool      `json:"is_stalled"`
	CirculationStrength    float64   `json:"circulation_strength"`
	TotalVortices          int       `json:"total_vortices"`
}

type OptimizationResult struct {
	Time                 time.Time `json:"time"`
	ShipID               int       `json:"ship_id"`
	SailID               int       `json:"sail_id"`
	InitialSailAngle     float64   `json:"initial_sail_angle"`
	OptimalSailAngle     float64   `json:"optimal_sail_angle"`
	InitialShipSpeed     float64   `json:"initial_ship_speed"`
	OptimizedShipSpeed   float64   `json:"optimized_ship_speed"`
	SpeedIncrease        float64   `json:"speed_increase"`
	WindSpeedUsed        float64   `json:"wind_speed_used"`
	WindDirectionUsed    float64   `json:"wind_direction_used"`
	Iterations           int       `json:"iterations"`
	ConvergenceThreshold float64   `json:"convergence_threshold"`
	HullDragInitial      float64   `json:"hull_drag_initial"`
	HullDragOptimized    float64   `json:"hull_drag_optimized"`
	NetThrustInitial     float64   `json:"net_thrust_initial"`
	NetThrustOptimized   float64   `json:"net_thrust_optimized"`
}

type AlertEvent struct {
	Time           time.Time `json:"time"`
	ShipID         int       `json:"ship_id"`
	SailID         *int      `json:"sail_id,omitempty"`
	AlertType      string    `json:"alert_type"`
	Severity       string    `json:"severity"`
	Message        string    `json:"message"`
	CurrentValue   *float64  `json:"current_value,omitempty"`
	ThresholdValue *float64  `json:"threshold_value,omitempty"`
	Acknowledged   bool      `json:"acknowledged"`
	Resolved       bool      `json:"resolved"`
}

type PolarCurve struct {
	PolarID         int       `json:"polar_id"`
	SailID          int       `json:"sail_id"`
	WindSpeed       float64   `json:"wind_speed"`
	AngleOfAttack   float64   `json:"angle_of_attack"`
	LiftCoefficient float64   `json:"lift_coefficient"`
	DragCoefficient float64   `json:"drag_coefficient"`
	LiftToDragRatio float64   `json:"lift_to_drag_ratio"`
	CreatedAt       time.Time `json:"created_at"`
}

type AlertThreshold struct {
	ThresholdID       int       `json:"threshold_id"`
	ShipID            *int      `json:"ship_id,omitempty"`
	ParameterName     string    `json:"parameter_name"`
	WarningThreshold  float64   `json:"warning_threshold"`
	CriticalThreshold float64   `json:"critical_threshold"`
	Enabled           bool      `json:"enabled"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type VortexLattice struct {
	PanelsChordwise int
	PanelsSpanwise  int
	VortexPoints    []VortexPoint
	ControlPoints   []ControlPoint
	InfluenceMatrix [][]float64
	Circulation     []float64
}

type VortexPoint struct {
	X, Y, Z float64
}

type ControlPoint struct {
	X, Y, Z                   float64
	NormalX, NormalY, NormalZ float64
}

type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	Time time.Time   `json:"time"`
}
