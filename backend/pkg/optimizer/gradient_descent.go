package optimizer

import (
	"math"

	"sail-simulation/pkg/aerodynamics"
	"sail-simulation/pkg/models"
)

type AdamOptimizer struct {
	learningRate         float64
	beta1                float64
	beta2                float64
	epsilon              float64
	maxIterations        int
	convergenceThreshold float64
	aeroSolver           *aerodynamics.AerodynamicSolver
	useNesterov          bool
	useBacktracking      bool
}

func NewAdamOptimizer(solver *aerodynamics.AerodynamicSolver) *AdamOptimizer {
	return &AdamOptimizer{
		learningRate:         0.5,
		beta1:                0.9,
		beta2:                0.999,
		epsilon:              1e-8,
		maxIterations:        100,
		convergenceThreshold: 1e-5,
		aeroSolver:           solver,
		useNesterov:          true,
		useBacktracking:      true,
	}
}

func (adam *AdamOptimizer) SetParams(lr float64, maxIter int, threshold float64) {
	adam.learningRate = lr
	adam.maxIterations = maxIter
	adam.convergenceThreshold = threshold
}

func (adam *AdamOptimizer) SetAdamParams(beta1, beta2, epsilon float64) {
	adam.beta1 = beta1
	adam.beta2 = beta2
	adam.epsilon = epsilon
}

func (adam *AdamOptimizer) CalculateHullDrag(ship *models.Ship, shipSpeed float64, waterDensity float64) float64 {
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

func (adam *AdamOptimizer) EvaluateObjective(
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

	aero := adam.aeroSolver.Solve(sail, testSensor)

	aoa := adam.aeroSolver.CalculateAngleOfAttack(sensor.WindDirection, sailAngle, sensor.Heading)
	thrust := adam.aeroSolver.CalculateThrust(
		aero.LiftForce, aero.DragForce, aoa, sailAngle, sensor.Heading, sensor.WindDirection,
	)

	waterDensity := 1025.0
	hullDrag := adam.CalculateHullDrag(ship, sensor.ShipSpeed, waterDensity)

	netForce := thrust - hullDrag
	if netForce < 0 {
		netForce = 0
	}

	mass := ship.Displacement * 1000
	shipMass := mass + 100000
	predictedSpeed := math.Sqrt(2 * netForce * 1.0 / shipMass)

	return predictedSpeed, aero
}

func (adam *AdamOptimizer) ComputeGradient(
	sailAngle float64,
	ship *models.Ship,
	sail *models.Sail,
	sensor *models.SensorData,
) float64 {
	eps := 0.25

	fPlus, _ := adam.EvaluateObjective(sailAngle+eps, ship, sail, sensor)
	fMinus, _ := adam.EvaluateObjective(sailAngle-eps, ship, sail, sensor)

	return (fPlus - fMinus) / (2 * eps)
}

func (adam *AdamOptimizer) backtrackingLineSearch(
	currentAngle float64,
	currentSpeed float64,
	grad float64,
	direction float64,
	ship *models.Ship,
	sail *models.Sail,
	sensor *models.SensorData,
	initialStep float64,
) (float64, float64, *models.AerodynamicResult) {
	if !adam.useBacktracking {
		newAngle := currentAngle + initialStep*direction
		newAngle = clampAngle(newAngle)
		newSpeed, newAero := adam.EvaluateObjective(newAngle, ship, sail, sensor)
		return initialStep, newSpeed, newAero
	}

	step := initialStep
	rho := 0.5
	c := 1e-4

	for i := 0; i < 20; i++ {
		newAngle := currentAngle + step*direction
		newAngle = clampAngle(newAngle)

		newSpeed, newAero := adam.EvaluateObjective(newAngle, ship, sail, sensor)

		sufficientIncrease := c * step * grad * direction

		if newSpeed >= currentSpeed+sufficientIncrease || step < 1e-4 {
			return step, newSpeed, newAero
		}

		step *= rho
	}

	return step, currentSpeed, nil
}

func clampAngle(angle float64) float64 {
	return math.Max(-80, math.Min(80, angle))
}

func (adam *AdamOptimizer) OptimizeSailAngle(
	ship *models.Ship,
	sail *models.Sail,
	sensor *models.SensorData,
) *models.OptimizationResult {

	currentAngle := sensor.SailAngle
	bestAngle := currentAngle
	bestSpeed := 0.0
	var bestAero *models.AerodynamicResult

	initialSpeed, initialAero := adam.EvaluateObjective(currentAngle, ship, sail, sensor)
	waterDensity := 1025.0
	initialHullDrag := adam.CalculateHullDrag(ship, sensor.ShipSpeed, waterDensity)

	aoa := adam.aeroSolver.CalculateAngleOfAttack(sensor.WindDirection, currentAngle, sensor.Heading)
	initialThrust := adam.aeroSolver.CalculateThrust(
		initialAero.LiftForce, initialAero.DragForce, aoa, currentAngle, sensor.Heading, sensor.WindDirection,
	)
	initialNetThrust := initialThrust - initialHullDrag

	m := 0.0
	v := 0.0

	lr := adam.learningRate

	iteration := 0
	prevSpeed := initialSpeed

	bestSpeed = initialSpeed
	bestAngle = currentAngle
	bestAero = initialAero

	consecutiveNoImprove := 0
	maxConsecutiveNoImprove := 8

	for iteration < adam.maxIterations {
		gradAngle := currentAngle
		if adam.useNesterov {
			lookaheadBias := 0.5
			lookahead := m * lookaheadBias
			gradAngle = currentAngle + lookahead
		}

		grad := adam.ComputeGradient(gradAngle, ship, sail, sensor)

		if math.Abs(grad) < 1e-8 {
			break
		}

		m = adam.beta1*m + (1-adam.beta1)*grad
		v = adam.beta2*v + (1-adam.beta2)*grad*grad

		mHat := m / (1 - math.Pow(adam.beta1, float64(iteration+1)))
		vHat := v / (1 - math.Pow(adam.beta2, float64(iteration+1)))

		update := lr * mHat / (math.Sqrt(vHat) + adam.epsilon)
		direction := 1.0
		if grad < 0 {
			direction = -1.0
		}

		step, newSpeed, newAero := adam.backtrackingLineSearch(
			currentAngle, prevSpeed, grad, direction, ship, sail, sensor, math.Abs(update),
		)

		newAngle := currentAngle + step*direction
		newAngle = clampAngle(newAngle)

		if newAero != nil && newSpeed > bestSpeed && !newAero.IsStalled {
			bestSpeed = newSpeed
			bestAngle = newAngle
			bestAero = newAero
			consecutiveNoImprove = 0
		} else {
			consecutiveNoImprove++
			if consecutiveNoImprove >= maxConsecutiveNoImprove {
				break
			}
		}

		if math.Abs(newSpeed-prevSpeed) < adam.convergenceThreshold {
			break
		}

		if newSpeed <= prevSpeed {
			lr *= 0.7
		} else {
			lr = math.Min(lr*1.05, 2.0)
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

	optimizedHullDrag := adam.CalculateHullDrag(ship, bestSpeed, waterDensity)
	aoaOpt := adam.aeroSolver.CalculateAngleOfAttack(sensor.WindDirection, bestAngle, sensor.Heading)
	optimizedThrust := adam.aeroSolver.CalculateThrust(
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
		ConvergenceThreshold: adam.convergenceThreshold,
		HullDragInitial:      initialHullDrag,
		HullDragOptimized:    optimizedHullDrag,
		NetThrustInitial:     initialNetThrust,
		NetThrustOptimized:   optimizedNetThrust,
	}
}

type GradientDescentOptimizer = AdamOptimizer

func NewGradientDescentOptimizer(solver *aerodynamics.AerodynamicSolver) *GradientDescentOptimizer {
	return NewAdamOptimizer(solver)
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
