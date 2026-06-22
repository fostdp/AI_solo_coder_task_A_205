package modules

import (
	"context"
	"log"
	"math"
	"time"

	"sail-simulation/pkg/config"
	"sail-simulation/pkg/models"
	"sail-simulation/pkg/optimizer"
)

type SailOptimizer struct {
	inputChan  <-chan *OptimizationRequest
	outputChan chan<- *OptimizationResult
	optCfg     *config.OptimizerConfig
	aeroCfg    *config.AerodynamicsConfig
}

func NewSailOptimizer(
	inputChan <-chan *OptimizationRequest,
	outputChan chan<- *OptimizationResult,
	optCfg *config.OptimizerConfig,
	aeroCfg *config.AerodynamicsConfig,
) *SailOptimizer {
	return &SailOptimizer{
		inputChan:  inputChan,
		outputChan: outputChan,
		optCfg:     optCfg,
		aeroCfg:    aeroCfg,
	}
}

func (o *SailOptimizer) Run(ctx context.Context) {
	log.Println("Sail optimizer started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Sail optimizer stopping")
			return
		case req := <-o.inputChan:
			result := o.processRequest(req)
			select {
			case o.outputChan <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (o *SailOptimizer) processRequest(req *OptimizationRequest) *OptimizationResult {
	start := time.Now()
	result := &OptimizationResult{
		Request: req,
	}

	sail := req.Sail
	if sail == nil {
		sail = &models.Sail{
			ID:          req.Sensor.SailID,
			ShipID:      req.Sensor.ShipID,
			Area:        o.aeroCfg.DefaultSail.Area,
			AspectRatio: o.aeroCfg.DefaultSail.AspectRatio,
			ChordLength: o.aeroCfg.DefaultSail.ChordLength,
			SpanLength:  o.aeroCfg.DefaultSail.SpanLength,
			Camber:      o.aeroCfg.DefaultSail.Camber,
		}
	}
	ship := req.Ship
	if ship == nil {
		ship = &models.Ship{
			ID:           req.Sensor.ShipID,
			HullLength:   o.aeroCfg.DefaultShip.HullLength,
			HullWidth:    o.aeroCfg.DefaultShip.HullWidth,
			Displacement: o.aeroCfg.DefaultShip.Displacement,
		}
	}

	opt := optimizer.NewGradientDescentOptimizer(
		sail,
		ship,
		o.optCfg.Adam.InitialLearningRate,
	)

	optResult, err := opt.OptimizeSailAngle(
		req.Sensor.WindSpeed,
		req.Sensor.WindDirection,
		req.Sensor.ShipSpeed,
		req.Sensor.AirDensity,
		o.aeroCfg.Stall.StallAngleDeg,
	)

	if err != nil {
		result.Error = err
		return result
	}

	predictedSpeed := optimizer.PredictShipSpeed(sail, ship, req.Sensor, optResult.OptimalAngle, o.aeroCfg.Stall.StallAngleDeg)

	result.Result = &models.OptimizationResult{
		ShipID:            req.Sensor.ShipID,
		SailID:            req.Sensor.SailID,
		Time:              time.Now(),
		CurrentAngle:      req.Sensor.SailAngle,
		OptimalAngle:      optResult.OptimalAngle,
		PredictedSpeed:    predictedSpeed,
		PredictedLift:     optResult.OptimalLift,
		PredictedDrag:     optResult.OptimalDrag,
		PredictedThrust:   optimizer.CalculateThrust(sail, req.Sensor, optResult.OptimalAngle, o.aeroCfg.Stall.StallAngleDeg),
		Iterations:        optResult.Iterations,
		ConvergenceValue:  optResult.ConvergenceValue,
		Converged:         optResult.Converged,
		LearningRate:      optResult.FinalLearningRate,
		AngleAdjustment:   optResult.OptimalAngle - req.Sensor.SailAngle,
		EfficiencyGain:    math.Max(0, (predictedSpeed-req.Sensor.ShipSpeed)/math.Max(req.Sensor.ShipSpeed, 0.1)*100),
	}

	log.Printf("Optimization: ship=%d sail=%d current=%.1f° optimal=%.1f° predicted=%.2fm/s gain=%.1f%% iter=%d (%.2fms)",
		req.Sensor.ShipID, req.Sensor.SailID,
		req.Sensor.SailAngle,
		optResult.OptimalAngle,
		predictedSpeed,
		result.Result.EfficiencyGain,
		optResult.Iterations,
		float64(time.Since(start).Microseconds())/1000.0)

	return result
}
