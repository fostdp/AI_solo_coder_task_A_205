package optimizer

import (
	"math"

	"sail-simulation/pkg/aerodynamics"
	"sail-simulation/pkg/models"
)

type GradientDescentOptimizer struct {
	learningRate         float64
	maxIterations        int
	convergenceThreshold float64
	aeroSolver           *aerodynamics.AerodynamicSolver
}

func NewGradientDescentOptimizer(solver *aerodynamics.AerodynamicSolver) *GradientDescentOptimizer {
	return &GradientDescentOptimizer{
		learningRate:         0.1,
		maxIterations:        200,
		convergenceThreshold: 1e-5,
		aeroSolver:           solver,
	}
}

func (gdo *GradientDescentOptimizer) SetParams(learningRate float64, maxIter int, threshold float64) {
	gdo.learningRate = learningRate
	gdo.maxIterations = maxIter
	gdo.convergenceThreshold = threshold
}

func (gdo *GradientDescentOptimizer) CalculateHullDrag(ship *models.Ship, shipSpeed float64, waterDensity float64) float64 {
	if shipSpeed < 0.001 {
		return 0
	}

	L := ship.HullLength
	W := ship.HullWidth
	disp := ship.Displacement
	S := 1.7 * L * W

	V := shipSpeed
	g := 9.81
	Fn := V / math.Sqrt(g*L)

	Cf := 0.075 / math.Pow(math.Log10(1.0+V*L/1.188e-6)-2, 2)

	FroudeNum := math.Min(Fn, 0.5)
	Cw := 0.5 * math.Exp(-4*FroudeNum) * math.Pow(FroudeNum, 1.5)

	Ca := 0.002 * (1 + disp/(L*L*L))

	Ct := Cf + Cw + Ca

	q := 0.5 * waterDensity * V * V * S
	return Ct * q
}

func (gdo *GradientDescentOptimizer) EvaluateObjective(
	sailAngle float64,
	ship *models.Ship,
	sail *models.Sail,
	sensor *models.SensorData,
) (float64, *models.AerodynamicResult) {

	testSensor := &models.SensorData{
		Time:          sensor.Time,
		ShipID:        sensor.ShipID,
		SailID:        sensor.SailID,
		WindSpeed:     sensor.WindSpeed,
		WindDirection: sensor.WindDirection,
		SailAngle:     sailAngle,
		ShipSpeed:     sensor.ShipSpeed,
		Heading:       sensor.Heading,
		AmbientTemp:   sensor.AmbientTemp,
		AirDensity:    sensor.AirDensity,
	}

	aero := gdo.aeroSolver.Solve(sail, testSensor)

	aoa := gdo.aeroSolver.CalculateAngleOfAttack(sensor.WindDirection, sailAngle, sensor.Heading)
	thrust := gdo.aeroSolver.CalculateThrust(
		aero.LiftForce, aero.DragForce, aoa, sailAngle, sensor.Heading, sensor.WindDirection,
	)

	waterDensity := 1025.0
	hullDrag := gdo.CalculateHullDrag(ship, sensor.ShipSpeed, waterDensity)

	netForce := thrust - hullDrag
	if netForce < 0 {
		netForce = 0
	}

	mass := ship.Displacement * 1000
	shipMass := mass + 100000
	predictedSpeed := math.Sqrt(2 * netForce * 1.0 / shipMass)

	return predictedSpeed, aero
}

func (gdo *GradientDescentOptimizer) ComputeGradient(
	sailAngle float64,
	ship *models.Ship,
	sail *models.Sail,
	sensor *models.SensorData,
) float64 {
	eps := 0.5

	fPlus, _ := gdo.EvaluateObjective(sailAngle+eps, ship, sail, sensor)
	fMinus, _ := gdo.EvaluateObjective(sailAngle-eps, ship, sail, sensor)

	return (fPlus - fMinus) / (2 * eps)
}

func (gdo *GradientDescentOptimizer) OptimizeSailAngle(
	ship *models.Ship,
	sail *models.Sail,
	sensor *models.SensorData,
) *models.OptimizationResult {

	currentAngle := sensor.SailAngle
	bestAngle := currentAngle
	bestSpeed := 0.0
	var bestAero *models.AerodynamicResult

	initialSpeed, initialAero := gdo.EvaluateObjective(currentAngle, ship, sail, sensor)
	waterDensity := 1025.0
	initialHullDrag := gdo.CalculateHullDrag(ship, sensor.ShipSpeed, waterDensity)

	aoa := gdo.aeroSolver.CalculateAngleOfAttack(sensor.WindDirection, currentAngle, sensor.Heading)
	initialThrust := gdo.aeroSolver.CalculateThrust(
		initialAero.LiftForce, initialAero.DragForce, aoa, currentAngle, sensor.Heading, sensor.WindDirection,
	)
	initialNetThrust := initialThrust - initialHullDrag

	lr := gdo.learningRate
	iteration := 0
	prevSpeed := initialSpeed

	for iteration < gdo.maxIterations {
		grad := gdo.ComputeGradient(currentAngle, ship, sail, sensor)

		if math.Abs(grad) < 1e-8 {
			break
		}

		newAngle := currentAngle + lr*grad
		newAngle = math.Max(-80, math.Min(80, newAngle))

		newSpeed, newAero := gdo.EvaluateObjective(newAngle, ship, sail, sensor)

		if newSpeed > bestSpeed && !newAero.IsStalled {
			bestSpeed = newSpeed
			bestAngle = newAngle
			bestAero = newAero
		}

		if math.Abs(newSpeed-prevSpeed) < gdo.convergenceThreshold {
			break
		}

		if newSpeed <= prevSpeed {
			lr *= 0.5
			if lr < 1e-4 {
				break
			}
		} else {
			lr = math.Min(lr*1.1, 1.0)
		}

		currentAngle = newAngle
		prevSpeed = newSpeed
		iteration++
	}

	if bestAero == nil {
		bestAero = initialAero
		bestSpeed = initialSpeed
		bestAngle = currentAngle
	}

	optimizedHullDrag := gdo.CalculateHullDrag(ship, bestSpeed, waterDensity)
	aoaOpt := gdo.aeroSolver.CalculateAngleOfAttack(sensor.WindDirection, bestAngle, sensor.Heading)
	optimizedThrust := gdo.aeroSolver.CalculateThrust(
		bestAero.LiftForce, bestAero.DragForce, aoaOpt, bestAngle, sensor.Heading, sensor.WindDirection,
	)
	optimizedNetThrust := optimizedThrust - optimizedHullDrag

	speedIncrease := 0.0
	if initialSpeed > 0.001 {
		speedIncrease = (bestSpeed - initialSpeed) / initialSpeed
	}

	return &models.OptimizationResult{
		Time:                 sensor.Time,
		ShipID:               sensor.ShipID,
		SailID:               sensor.SailID,
		InitialSailAngle:     sensor.SailAngle,
		OptimalSailAngle:     bestAngle,
		InitialShipSpeed:     sensor.ShipSpeed,
		OptimizedShipSpeed:   bestSpeed,
		SpeedIncrease:        speedIncrease,
		WindSpeedUsed:        sensor.WindSpeed,
		WindDirectionUsed:    sensor.WindDirection,
		Iterations:           iteration,
		ConvergenceThreshold: gdo.convergenceThreshold,
		HullDragInitial:      initialHullDrag,
		HullDragOptimized:    optimizedHullDrag,
		NetThrustInitial:     initialNetThrust,
		NetThrustOptimized:   optimizedNetThrust,
	}
}

func (gdo *GradientDescentOptimizer) FindMaxLDRatioAngle(sail *models.Sail, windSpeed float64) float64 {
	polars := gdo.aeroSolver.GeneratePolarCurve(sail, windSpeed)

	bestLD := -math.MaxFloat64
	bestAngle := 0.0

	for _, p := range polars {
		if p.LiftToDragRatio > bestLD && p.AngleOfAttack >= -15 && p.AngleOfAttack <= 15 {
			bestLD = p.LiftToDragRatio
			bestAngle = p.AngleOfAttack
		}
	}

	return bestAngle
}
