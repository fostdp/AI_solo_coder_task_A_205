package modules

import (
	"context"
	"log"
	"math"
	"time"

	"sail-simulation/pkg/aerodynamics"
	"sail-simulation/pkg/config"
	"sail-simulation/pkg/models"
)

type AerodynamicsSimulator struct {
	inputChan  <-chan *AeroSimRequest
	outputChan chan<- *AeroSimResult
	aeroCfg    *config.AerodynamicsConfig
	solver     *aerodynamics.AerodynamicSolver
}

func NewAerodynamicsSimulator(
	inputChan <-chan *AeroSimRequest,
	outputChan chan<- *AeroSimResult,
	aeroCfg *config.AerodynamicsConfig,
) *AerodynamicsSimulator {
	defaultSail := &models.Sail{
		Area:        aeroCfg.DefaultSail.Area,
		AspectRatio: aeroCfg.DefaultSail.AspectRatio,
		ChordLength: aeroCfg.DefaultSail.ChordLength,
		SpanLength:  aeroCfg.DefaultSail.SpanLength,
		Camber:      aeroCfg.DefaultSail.Camber,
	}
	solver := aerodynamics.NewAerodynamicSolver(defaultSail,
		aeroCfg.VortexLattice.ChordwisePanels,
		aeroCfg.VortexLattice.SpanwisePanels,
		aeroCfg.BoundaryLayer.TransitionReynolds)

	return &AerodynamicsSimulator{
		inputChan:  inputChan,
		outputChan: outputChan,
		aeroCfg:    aeroCfg,
		solver:     solver,
	}
}

func (s *AerodynamicsSimulator) Run(ctx context.Context) {
	log.Println("Aerodynamics simulator started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Aerodynamics simulator stopping")
			return
		case req := <-s.inputChan:
			result := s.processRequest(req)
			select {
			case s.outputChan <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *AerodynamicsSimulator) processRequest(req *AeroSimRequest) *AeroSimResult {
	start := time.Now()
	result := &AeroSimResult{
		Sensor: req.Sensor,
	}

	sail := req.Sail
	if sail == nil {
		sail = &models.Sail{
			ID:          req.Sensor.SailID,
			ShipID:      req.Sensor.ShipID,
			Area:        s.aeroCfg.DefaultSail.Area,
			AspectRatio: s.aeroCfg.DefaultSail.AspectRatio,
			ChordLength: s.aeroCfg.DefaultSail.ChordLength,
			SpanLength:  s.aeroCfg.DefaultSail.SpanLength,
			Camber:      s.aeroCfg.DefaultSail.Camber,
		}
	}

	s.solver.UpdateSailGeometry(sail)

	relativeWind := req.Sensor.WindDirection - req.Sensor.SailAngle
	if relativeWind > 180 {
		relativeWind -= 360
	}
	if relativeWind < -180 {
		relativeWind += 360
	}

	shipVel := models.VelocityVector{
		X: -req.Sensor.ShipSpeed,
		Z: 0,
	}
	windRad := req.Sensor.WindDirection * math.Pi / 180
	trueWind := models.VelocityVector{
		X: -req.Sensor.WindSpeed * math.Cos(windRad),
		Z: -req.Sensor.WindSpeed * math.Sin(windRad),
	}
	effectiveWind := models.VelocityVector{
		X: trueWind.X - shipVel.X,
		Z: trueWind.Z - shipVel.Z,
	}
	effectiveWindSpeed := math.Sqrt(effectiveWind.X*effectiveWind.X + effectiveWind.Z*effectiveWind.Z)

	aeroResult, err := s.solver.Solve(
		relativeWind,
		effectiveWindSpeed,
		req.Sensor.AirDensity,
		s.aeroCfg.Stall.StallAngleDeg,
	)

	if err != nil {
		result.Error = err
		return result
	}

	result.Result = &models.AerodynamicResult{
		ShipID:            req.Sensor.ShipID,
		SailID:            req.Sensor.SailID,
		Time:              time.Now(),
		AngleOfAttack:     relativeWind,
		LiftCoefficient:   aeroResult.LiftCoefficient,
		DragCoefficient:   aeroResult.DragCoefficient,
		LiftForce:         aeroResult.LiftForce,
		DragForce:         aeroResult.DragForce,
		EffectiveWind:     effectiveWindSpeed,
		ReynoldsNumber:    aeroResult.ReynoldsNumber,
		IsStalled:         aeroResult.IsStalled,
		PressureDrag:      aeroResult.PressureDrag,
		FrictionDrag:      aeroResult.FrictionDrag,
		InducedDrag:       aeroResult.InducedDrag,
		BoundaryLayerThk:  aeroResult.BoundaryLayerThickness,
		TransitionPoint:   aeroResult.TransitionPoint,
		LDRatio:           aeroResult.LiftCoefficient / math.Max(aeroResult.DragCoefficient, 0.0001),
		IsSeparated:       aeroResult.IsSeparated,
		SeparationPoint:   aeroResult.SeparationPoint,
		ShapeFactor:       aeroResult.ShapeFactor,
		VPMParticleCount:  aeroResult.VPMParticleCount,
		ClCorrected:       aeroResult.ClCorrected,
		CdCorrected:       aeroResult.CdCorrected,
	}

	log.Printf("Aero simulation: ship=%d sail=%d AoA=%.1f° Cl=%.3f Cd=%.3f L/D=%.1f stalled=%v separated=%v (%.2fms)",
		req.Sensor.ShipID, req.Sensor.SailID,
		relativeWind,
		aeroResult.LiftCoefficient,
		aeroResult.DragCoefficient,
		result.Result.LDRatio,
		aeroResult.IsStalled,
		aeroResult.IsSeparated,
		float64(time.Since(start).Microseconds())/1000.0)

	return result
}
