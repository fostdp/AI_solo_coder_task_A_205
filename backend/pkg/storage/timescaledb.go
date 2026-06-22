package storage

import (
	"context"
	"fmt"
	"time"

	"sail-simulation/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	pool *pgxpool.Pool
}

func NewStorage(connString string) (*Storage, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Storage{pool: pool}, nil
}

func (s *Storage) Close() {
	s.pool.Close()
}

func (s *Storage) InsertSensorData(ctx context.Context, data *models.SensorData) error {
	query := `INSERT INTO sensor_data (time, ship_id, sail_id, wind_speed, wind_direction, sail_angle, ship_speed, heading, ambient_temp, air_density)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.pool.Exec(ctx, query,
		data.Time, data.ShipID, data.SailID,
		data.WindSpeed, data.WindDirection, data.SailAngle,
		data.ShipSpeed, data.Heading, data.AmbientTemp, data.AirDensity,
	)
	return err
}

func (s *Storage) InsertAerodynamicResult(ctx context.Context, result *models.AerodynamicResult) error {
	query := `INSERT INTO aerodynamic_results (
		time, ship_id, sail_id, angle_of_attack, lift_coefficient, drag_coefficient,
		lift_force, drag_force, pressure_drag, friction_drag, induced_drag,
		reynolds_number, boundary_layer_thickness, is_stalled, circulation_strength, total_vortices
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	_, err := s.pool.Exec(ctx, query,
		result.Time, result.ShipID, result.SailID,
		result.AngleOfAttack, result.LiftCoefficient, result.DragCoefficient,
		result.LiftForce, result.DragForce, result.PressureDrag, result.FrictionDrag, result.InducedDrag,
		result.ReynoldsNumber, result.BoundaryLayerThickness, result.IsStalled,
		result.CirculationStrength, result.TotalVortices,
	)
	return err
}

func (s *Storage) InsertOptimizationResult(ctx context.Context, result *models.OptimizationResult) error {
	query := `INSERT INTO optimization_results (
		time, ship_id, sail_id, initial_sail_angle, optimal_sail_angle,
		initial_ship_speed, optimized_ship_speed, speed_increase,
		wind_speed_used, wind_direction_used, iterations, convergence_threshold,
		hull_drag_initial, hull_drag_optimized, net_thrust_initial, net_thrust_optimized
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	_, err := s.pool.Exec(ctx, query,
		result.Time, result.ShipID, result.SailID,
		result.InitialSailAngle, result.OptimalSailAngle,
		result.InitialShipSpeed, result.OptimizedShipSpeed, result.SpeedIncrease,
		result.WindSpeedUsed, result.WindDirectionUsed,
		result.Iterations, result.ConvergenceThreshold,
		result.HullDragInitial, result.HullDragOptimized,
		result.NetThrustInitial, result.NetThrustOptimized,
	)
	return err
}

func (s *Storage) InsertAlertEvent(ctx context.Context, alert *models.AlertEvent) error {
	query := `INSERT INTO alert_events (
		time, ship_id, sail_id, alert_type, severity, message,
		current_value, threshold_value, acknowledged, resolved
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.pool.Exec(ctx, query,
		alert.Time, alert.ShipID, alert.SailID,
		alert.AlertType, alert.Severity, alert.Message,
		alert.CurrentValue, alert.ThresholdValue,
		alert.Acknowledged, alert.Resolved,
	)
	return err
}

func (s *Storage) InsertPolarCurve(ctx context.Context, polars []models.PolarCurve) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO polar_curves (
		sail_id, wind_speed, angle_of_attack, lift_coefficient, drag_coefficient, lift_to_drag_ratio
	) VALUES ($1, $2, $3, $4, $5, $6)`

	for _, p := range polars {
		_, err := tx.Exec(ctx, query,
			p.SailID, p.WindSpeed, p.AngleOfAttack,
			p.LiftCoefficient, p.DragCoefficient, p.LiftToDragRatio,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Storage) GetShips(ctx context.Context) ([]models.Ship, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ship_id, ship_name, ship_type, hull_length, hull_width, displacement, created_at FROM ships`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ships []models.Ship
	for rows.Next() {
		var ship models.Ship
		err := rows.Scan(&ship.ShipID, &ship.ShipName, &ship.ShipType,
			&ship.HullLength, &ship.HullWidth, &ship.Displacement, &ship.CreatedAt)
		if err != nil {
			return nil, err
		}
		ships = append(ships, ship)
	}
	return ships, nil
}

func (s *Storage) GetSailsByShip(ctx context.Context, shipID int) ([]models.Sail, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT sail_id, ship_id, sail_name, sail_position, area, aspect_ratio, chord_length, span_length, camber, created_at
		 FROM sails WHERE ship_id = $1`, shipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sails []models.Sail
	for rows.Next() {
		var sail models.Sail
		err := rows.Scan(&sail.SailID, &sail.ShipID, &sail.SailName, &sail.SailPosition,
			&sail.Area, &sail.AspectRatio, &sail.ChordLength, &sail.SpanLength,
			&sail.Camber, &sail.CreatedAt)
		if err != nil {
			return nil, err
		}
		sails = append(sails, sail)
	}
	return sails, nil
}

func (s *Storage) GetSailByID(ctx context.Context, sailID int) (*models.Sail, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT sail_id, ship_id, sail_name, sail_position, area, aspect_ratio, chord_length, span_length, camber, created_at
		 FROM sails WHERE sail_id = $1`, sailID)

	var sail models.Sail
	err := row.Scan(&sail.SailID, &sail.ShipID, &sail.SailName, &sail.SailPosition,
		&sail.Area, &sail.AspectRatio, &sail.ChordLength, &sail.SpanLength,
		&sail.Camber, &sail.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &sail, nil
}

func (s *Storage) GetShipByID(ctx context.Context, shipID int) (*models.Ship, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT ship_id, ship_name, ship_type, hull_length, hull_width, displacement, created_at
		 FROM ships WHERE ship_id = $1`, shipID)

	var ship models.Ship
	err := row.Scan(&ship.ShipID, &ship.ShipName, &ship.ShipType,
		&ship.HullLength, &ship.HullWidth, &ship.Displacement, &ship.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &ship, nil
}

func (s *Storage) GetAlertThresholds(ctx context.Context) ([]models.AlertThreshold, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT threshold_id, ship_id, parameter_name, warning_threshold, critical_threshold, enabled, updated_at
		 FROM alert_thresholds WHERE enabled = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var thresholds []models.AlertThreshold
	for rows.Next() {
		var t models.AlertThreshold
		var shipID *int
		err := rows.Scan(&t.ThresholdID, &shipID, &t.ParameterName,
			&t.WarningThreshold, &t.CriticalThreshold, &t.Enabled, &t.UpdatedAt)
		if err != nil {
			return nil, err
		}
		t.ShipID = shipID
		thresholds = append(thresholds, t)
	}
	return thresholds, nil
}

func (s *Storage) GetRecentSensorData(ctx context.Context, shipID, sailID int, limit int) ([]models.SensorData, error) {
	query := `SELECT time, ship_id, sail_id, wind_speed, wind_direction, sail_angle, ship_speed, heading, ambient_temp, air_density
		FROM sensor_data WHERE ship_id = $1 AND sail_id = $2
		ORDER BY time DESC LIMIT $3`

	rows, err := s.pool.Query(ctx, query, shipID, sailID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []models.SensorData
	for rows.Next() {
		var d models.SensorData
		err := rows.Scan(&d.Time, &d.ShipID, &d.SailID,
			&d.WindSpeed, &d.WindDirection, &d.SailAngle,
			&d.ShipSpeed, &d.Heading, &d.AmbientTemp, &d.AirDensity)
		if err != nil {
			return nil, err
		}
		data = append(data, d)
	}
	return data, nil
}

func (s *Storage) GetRecentAeroResults(ctx context.Context, shipID, sailID int, limit int) ([]models.AerodynamicResult, error) {
	query := `SELECT time, ship_id, sail_id, angle_of_attack, lift_coefficient, drag_coefficient,
		lift_force, drag_force, pressure_drag, friction_drag, induced_drag,
		reynolds_number, boundary_layer_thickness, is_stalled, circulation_strength, total_vortices
		FROM aerodynamic_results WHERE ship_id = $1 AND sail_id = $2
		ORDER BY time DESC LIMIT $3`

	rows, err := s.pool.Query(ctx, query, shipID, sailID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.AerodynamicResult
	for rows.Next() {
		var r models.AerodynamicResult
		err := rows.Scan(&r.Time, &r.ShipID, &r.SailID,
			&r.AngleOfAttack, &r.LiftCoefficient, &r.DragCoefficient,
			&r.LiftForce, &r.DragForce, &r.PressureDrag, &r.FrictionDrag, &r.InducedDrag,
			&r.ReynoldsNumber, &r.BoundaryLayerThickness, &r.IsStalled,
			&r.CirculationStrength, &r.TotalVortices)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *Storage) GetRecentAlerts(ctx context.Context, shipID int, limit int) ([]models.AlertEvent, error) {
	query := `SELECT time, ship_id, sail_id, alert_type, severity, message,
		current_value, threshold_value, acknowledged, resolved
		FROM alert_events WHERE ship_id = $1
		ORDER BY time DESC LIMIT $2`

	rows, err := s.pool.Query(ctx, query, shipID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.AlertEvent
	for rows.Next() {
		var a models.AlertEvent
		var sailID *int
		var currentVal, thresholdVal *float64
		err := rows.Scan(&a.Time, &a.ShipID, &sailID,
			&a.AlertType, &a.Severity, &a.Message,
			&currentVal, &thresholdVal, &a.Acknowledged, &a.Resolved)
		if err != nil {
			return nil, err
		}
		a.SailID = sailID
		a.CurrentValue = currentVal
		a.ThresholdValue = thresholdVal
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (s *Storage) GetRecentOptimizations(ctx context.Context, shipID, sailID int, limit int) ([]models.OptimizationResult, error) {
	query := `SELECT time, ship_id, sail_id, initial_sail_angle, optimal_sail_angle,
		initial_ship_speed, optimized_ship_speed, speed_increase,
		wind_speed_used, wind_direction_used, iterations, convergence_threshold,
		hull_drag_initial, hull_drag_optimized, net_thrust_initial, net_thrust_optimized
		FROM optimization_results WHERE ship_id = $1 AND sail_id = $2
		ORDER BY time DESC LIMIT $3`

	rows, err := s.pool.Query(ctx, query, shipID, sailID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.OptimizationResult
	for rows.Next() {
		var r models.OptimizationResult
		err := rows.Scan(&r.Time, &r.ShipID, &r.SailID,
			&r.InitialSailAngle, &r.OptimalSailAngle,
			&r.InitialShipSpeed, &r.OptimizedShipSpeed, &r.SpeedIncrease,
			&r.WindSpeedUsed, &r.WindDirectionUsed,
			&r.Iterations, &r.ConvergenceThreshold,
			&r.HullDragInitial, &r.HullDragOptimized,
			&r.NetThrustInitial, &r.NetThrustOptimized)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *Storage) GetPolarCurve(ctx context.Context, sailID int, windSpeed float64) ([]models.PolarCurve, error) {
	query := `SELECT polar_id, sail_id, wind_speed, angle_of_attack, lift_coefficient, drag_coefficient, lift_to_drag_ratio, created_at
		FROM polar_curves WHERE sail_id = $1 AND ABS(wind_speed - $2) < 0.1
		ORDER BY angle_of_attack`

	rows, err := s.pool.Query(ctx, query, sailID, windSpeed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var polars []models.PolarCurve
	for rows.Next() {
		var p models.PolarCurve
		err := rows.Scan(&p.PolarID, &p.SailID, &p.WindSpeed, &p.AngleOfAttack,
			&p.LiftCoefficient, &p.DragCoefficient, &p.LiftToDragRatio, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		polars = append(polars, p)
	}
	return polars, nil
}

type TimeSeriesPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

func (s *Storage) GetSensorTimeSeries(ctx context.Context, shipID, sailID int, metric string, start, end time.Time, bucket string) ([]TimeSeriesPoint, error) {
	validMetrics := map[string]bool{
		"wind_speed": true, "wind_direction": true, "sail_angle": true, "ship_speed": true,
	}
	if !validMetrics[metric] {
		return nil, fmt.Errorf("invalid metric: %s", metric)
	}

	query := fmt.Sprintf(`SELECT time_bucket($1, time) as bucket, AVG(%s)
		FROM sensor_data WHERE ship_id = $2 AND sail_id = $3 AND time BETWEEN $4 AND $5
		GROUP BY bucket ORDER BY bucket`, metric)

	rows, err := s.pool.Query(ctx, query, bucket, shipID, sailID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		var t time.Time
		var v float64
		if err := rows.Scan(&t, &v); err != nil {
			return nil, err
		}
		points = append(points, TimeSeriesPoint{Time: t, Value: v})
	}
	return points, nil
}
