package aerodynamics

import (
	"math"
)

type ViscousVortexParticle struct {
	X, Y, Z    float64
	Gamma      float64
	CoreRadius float64
	Vx, Vy, Vz float64
	Age        float64
	IsTrailing bool
}

type VPMSolver struct {
	particles        []ViscousVortexParticle
	chordwisePanels  int
	separationPoint  float64
	boundaryLayerH12 float64
	circulationLoss  float64
	separated        bool
}

func NewVPMSolver(chordPanels int) *VPMSolver {
	return &VPMSolver{
		particles:        make([]ViscousVortexParticle, 0),
		chordwisePanels:  chordPanels,
		separationPoint:  1.0,
		boundaryLayerH12: 2.59,
	}
}

func (vpm *VPMSolver) Reset() {
	vpm.particles = vpm.particles[:0]
	vpm.separationPoint = 1.0
	vpm.boundaryLayerH12 = 2.59
	vpm.separated = false
	vpm.circulationLoss = 0.0
}

func (vpm *VPMSolver) ComputeSeparationPoint(
	aoa, camber, chord, reynolds float64,
	velocityDistribution []float64,
) float64 {
	n := len(velocityDistribution)
	if n < 3 {
		return 1.0
	}

	shapeFactors := make([]float64, n)
	theta := make([]float64, n)
	deltaStar := make([]float64, n)

	for i := 0; i < n; i++ {
		x := float64(i) / float64(n-1)
		if x < 0.01 {
			theta[i] = 0
			deltaStar[i] = 0
			shapeFactors[i] = 2.59
			continue
		}

		localRe := reynolds * x
		cf := 0.0592 / math.Pow(localRe, 0.2)
		if localRe < 500000 {
			cf = 0.664 / math.Sqrt(localRe)
		}

		ue := velocityDistribution[i]
		theta[i] = 0.5 * cf * x / math.Max(ue, 0.01)

		H := 2.59
		if ue < 0 {
			H = 4.0
		}
		deltaStar[i] = theta[i] * H
		shapeFactors[i] = H
	}

	for i := n - 2; i > 0; i-- {
		uePrev := velocityDistribution[i+1]
		ueCurr := velocityDistribution[i]

		dUe := uePrev - ueCurr

		adversePressure := dUe > 0

		if adversePressure && shapeFactors[i] > 2.8 {
			vpm.separated = true
			sepX := float64(i) / float64(n-1)
			vpm.separationPoint = sepX
			vpm.boundaryLayerH12 = shapeFactors[i]
			return sepX
		}
	}

	vpm.separationPoint = 1.0
	return 1.0
}

func (vpm *VPMSolver) GenerateBoundaryLayerParticles(
	sailChord, sailSpan, camber float64,
	aoa float64,
	cl float64,
	cdPressure float64,
	reynolds float64,
	velocityDistribution []float64,
) int {
	vpm.Reset()

	nPanels := vpm.chordwisePanels
	particlesPerPanel := 4
	totalParticles := nPanels * particlesPerPanel

	sepIdx := int(vpm.separationPoint * float64(nPanels-1))
	if sepIdx >= nPanels {
		sepIdx = nPanels - 1
	}

	for i := 0; i < nPanels; i++ {
		x := float64(i) / float64(nPanels-1)
		isSeparated := i > sepIdx && vpm.separated

		ue := 1.0
		if i < len(velocityDistribution) {
			ue = math.Abs(velocityDistribution[i])
		}

		camberZ := 4 * camber * sailChord * (x - x*x)
		dZdx := 4 * camber * (1 - 2*x)
		slopeAngle := math.Atan(dZdx)

		for j := 0; j < particlesPerPanel; j++ {
			side := float64(j%2)*2 - 1
			offset := float64(j/2) * 0.3

			delta := 0.0
			localRe := reynolds * math.Max(x, 0.01)
			if localRe > 500000 {
				delta = 0.37 * sailChord * x / math.Pow(localRe, 0.2)
			} else {
				delta = 4.92 * sailChord * x / math.Sqrt(localRe)
			}

			normalDist := delta * (0.1 + offset*0.4)

			px := x * sailChord
			py := side * sailSpan * 0.4 * (0.3 + 0.7*math.Sin(x*math.Pi))
			pz := camberZ + side*normalDist*math.Cos(slopeAngle)

			gammaMag := ue * delta / float64(particlesPerPanel)
			if isSeparated {
				gammaMag *= 0.6
			}

			gamma := side * gammaMag * cl * 0.1

			coreRadius := delta * 0.3
			if isSeparated {
				coreRadius *= 2.0
			}

			vpm.particles = append(vpm.particles, ViscousVortexParticle{
				X: px, Y: py, Z: pz,
				Gamma:      gamma,
				CoreRadius: coreRadius,
				IsTrailing: isSeparated,
				Age:        0,
			})
		}
	}

	if vpm.separated {
		nTrailing := 20
		for i := 0; i < nTrailing; i++ {
			t := float64(i) / float64(nTrailing-1)
			px := vpm.separationPoint * sailChord
			py := (t - 0.5) * sailSpan * 0.8
			pz := 4 * camber * sailChord * (vpm.separationPoint - vpm.separationPoint*vpm.separationPoint)

			gamma := cl * sailChord * 0.03 * (1 - t*0.5)
			if i%2 == 0 {
				gamma = -gamma
			}

			vpm.particles = append(vpm.particles, ViscousVortexParticle{
				X: px + t*2, Y: py, Z: pz,
				Gamma:      gamma,
				CoreRadius: 0.5 + t,
				IsTrailing: true,
				Age:        t * 3,
			})
		}
	}

	totalGamma := 0.0
	for _, p := range vpm.particles {
		totalGamma += math.Abs(p.Gamma)
	}
	vpm.circulationLoss = totalGamma * 0.1

	return len(vpm.particles)
}

func (vpm *VPMSolver) ParticleInducedVelocity(px, py, pz float64) (float64, float64, float64) {
	ux, uy, uz := 0.0, 0.0, 0.0

	for _, p := range vpm.particles {
		dx := px - p.X
		dy := py - p.Y
		dz := pz - p.Z

		r2 := dx*dx + dy*dy + dz*dz
		r := math.Sqrt(r2)
		if r < p.CoreRadius*0.1 {
			continue
		}

		sigma := p.CoreRadius
		factor := p.Gamma / (4 * math.Pi * r2 * (r2 + sigma*sigma))

		ux += (dy*p.Gamma - dz*p.Gamma) * factor
		uy += (dz*p.Gamma - dx*p.Gamma) * factor * 0.5
		uz += (dx*p.Gamma - dy*p.Gamma) * factor * 0.3
	}

	return ux, uy, uz
}

func (vpm *VPMSolver) CorrectAerodynamicCoefficients(
	cl, cd, reynolds, aoa float64,
	sailAR float64,
) (float64, float64) {
	aoaRad := aoa * math.Pi / 180.0
	aoaAbs := math.Abs(aoa)

	clCorrection := 1.0
	cdCorrection := 1.0

	if vpm.separated {
		sepFactor := (1.0 - vpm.separationPoint)

		clCorrection = 1.0 - sepFactor*0.35

		cdViscousAddition := sepFactor * 0.08 * (1 + 0.5*cl*cl)
		cdCorrection = 1.0 + cdViscousAddition
	} else {
		cdBlasius := 1.328 / math.Sqrt(math.Max(reynolds, 100))
		cdCorrection = 1.0 + cdBlasius*50
	}

	if aoaAbs > 12 {
		aoaExcess := (aoaAbs - 12) / 6.0
		clCorrection *= math.Exp(-0.15 * aoaExcess * aoaExcess)
		cdCorrection *= (1.0 + 0.2*aoaExcess)
	}

	spanEfficiency := 0.95
	if vpm.separated {
		spanEfficiency = 0.85 - 0.1*(1.0-vpm.separationPoint)
	}
	cdInducedCorrection := (cl * cl) / (math.Pi * sailAR * spanEfficiency)

	newCl := cl * clCorrection
	newCd := cd*cdCorrection + cdInducedCorrection*0.05

	return newCl, newCd
}

func (vpm *VPMSolver) ComputeVelocityDistribution(
	aoa, camber, chord float64,
	cl float64,
	nPoints int,
) []float64 {
	velocities := make([]float64, nPoints)
	aoaRad := aoa * math.Pi / 180.0

	for i := 0; i < nPoints; i++ {
		x := float64(i) / float64(nPoints-1)

		camberSlope := 4 * camber * (1 - 2*x)
		camberAngle := math.Atan(camberSlope)

		localAOA := aoaRad + camberAngle

		xNonDim := x / chord
		if xNonDim < 0.001 {
			xNonDim = 0.001
		}

		thwaitesFactor := math.Sqrt(1 - xNonDim)
		suctionPeak := 1.5 + camber*3 + math.Abs(localAOA)*2

		if x < 0.1 {
			velocities[i] = 1.0 + suctionPeak*(1-x/0.1)
		} else if x < 0.7 {
			recovery := (x - 0.1) / 0.6
			velocities[i] = 1.0 + (suctionPeak-1.0)*(1-recovery*0.7)
		} else {
			recovery := (x - 0.7) / 0.3
			velocities[i] = 1.0 - 0.2*recovery
		}

		if localAOA < 0 {
			velocities[i] = 2.0 - velocities[i]
		}
	}

	return velocities
}

func (vpm *VPMSolver) GetTrailingVortexCount() int {
	count := 0
	for _, p := range vpm.particles {
		if p.IsTrailing {
			count++
		}
	}
	return count
}

func (vpm *VPMSolver) GetParticles() []ViscousVortexParticle {
	return vpm.particles
}

func (vpm *VPMSolver) GetSeparationPoint() float64 {
	return vpm.separationPoint
}

func (vpm *VPMSolver) IsSeparated() bool {
	return vpm.separated
}

func (vpm *VPMSolver) GetShapeFactor() float64 {
	return vpm.boundaryLayerH12
}
