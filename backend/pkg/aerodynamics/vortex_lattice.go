package aerodynamics

import (
	"math"

	"sail-simulation/pkg/models"
)

type VortexLatticeSolver struct {
	panelsChordwise int
	panelsSpanwise  int
}

func NewVortexLatticeSolver(chordPanels, spanPanels int) *VortexLatticeSolver {
	return &VortexLatticeSolver{
		panelsChordwise: chordPanels,
		panelsSpanwise:  spanPanels,
	}
}

func (vls *VortexLatticeSolver) GenerateLattice(sail *models.Sail) *models.VortexLattice {
	vl := &models.VortexLattice{
		PanelsChordwise: vls.panelsChordwise,
		PanelsSpanwise:  vls.panelsSpanwise,
	}

	chord := sail.ChordLength
	span := sail.SpanLength
	camber := sail.Camber

	nTotal := vls.panelsChordwise * vls.panelsSpanwise
	vl.VortexPoints = make([]models.VortexPoint, 0, nTotal*4)
	vl.ControlPoints = make([]models.ControlPoint, 0, nTotal)

	for i := 0; i < vls.panelsSpanwise; i++ {
		y1 := -span/2 + float64(i)*span/float64(vls.panelsSpanwise)
		y2 := -span/2 + float64(i+1)*span/float64(vls.panelsSpanwise)
		yMid := (y1 + y2) / 2

		for j := 0; j < vls.panelsChordwise; j++ {
			x1 := float64(j) * chord / float64(vls.panelsChordwise)
			x2 := float64(j+1) * chord / float64(vls.panelsChordwise)
			x3q := x1 + 0.75*(x2-x1)
			xMid := x1 + 0.5*(x2-x1)

			zMid := 4 * camber * chord * (xMid/chord - xMid*xMid/(chord*chord))
			dZdx := 4 * camber * (1 - 2*xMid/chord)
			normalMag := math.Sqrt(dZdx*dZdx + 1)
			normalX := -dZdx / normalMag
			normalZ := 1.0 / normalMag

			vl.ControlPoints = append(vl.ControlPoints, models.ControlPoint{
				X: x3q, Y: yMid, Z: zMid,
				NormalX: normalX, NormalY: 0, NormalZ: normalZ,
			})

			quarterX := x1 + 0.25*(x2-x1)
			vl.VortexPoints = append(vl.VortexPoints,
				models.VortexPoint{X: quarterX, Y: y1, Z: 0},
				models.VortexPoint{X: quarterX, Y: y2, Z: 0},
				models.VortexPoint{X: quarterX, Y: y1, Z: 0},
				models.VortexPoint{X: quarterX, Y: y2, Z: 0},
			)
		}
	}

	return vl
}

func (vls *VortexLatticeSolver) CalculateInfluenceMatrix(vl *models.VortexLattice) {
	n := len(vl.ControlPoints)
	vl.InfluenceMatrix = make([][]float64, n)
	for i := range vl.InfluenceMatrix {
		vl.InfluenceMatrix[i] = make([]float64, n)
	}

	for i, cp := range vl.ControlPoints {
		for j := 0; j < n; j++ {
			vpIdx := j * 4
			if vpIdx+3 >= len(vl.VortexPoints) {
				continue
			}
			v1 := vl.VortexPoints[vpIdx]
			v2 := vl.VortexPoints[vpIdx+1]

			inf := vortexSegmentInducedVelocity(
				cp.X, cp.Y, cp.Z,
				v1.X, v1.Y, v1.Z,
				v2.X, v2.Y, v2.Z,
			)

			normalVel := inf[0]*cp.NormalX + inf[1]*cp.NormalY + inf[2]*cp.NormalZ
			vl.InfluenceMatrix[i][j] = normalVel
		}
	}
}

func vortexSegmentInducedVelocity(px, py, pz, x1, y1, z1, x2, y2, z2 float64) [3]float64 {
	const core = 0.0001

	r1x := px - x1
	r1y := py - y1
	r1z := pz - z1
	r2x := px - x2
	r2y := py - y2
	r2z := pz - z2

	r1 := math.Sqrt(r1x*r1x + r1y*r1y + r1z*r1z)
	r2 := math.Sqrt(r2x*r2x + r2y*r2y + r2z*r2z)

	if r1 < core || r2 < core {
		return [3]float64{0, 0, 0}
	}

	r0x := x2 - x1
	r0y := y2 - y1
	r0z := z2 - z1

	crossX := r1y*r2z - r1z*r2y
	crossY := r1z*r2x - r1x*r2z
	crossZ := r1x*r2y - r1y*r2x

	crossMag2 := crossX*crossX + crossY*crossY + crossZ*crossZ
	if crossMag2 < 1e-10 {
		return [3]float64{0, 0, 0}
	}

	dot1 := r0x*r1x + r0y*r1y + r0z*r1z
	dot2 := r0x*r2x + r0y*r2y + r0z*r2z

	factor := (1.0 / (4 * math.Pi * crossMag2)) * (dot1/r1 - dot2/r2)

	return [3]float64{
		crossX * factor,
		crossY * factor,
		crossZ * factor,
	}
}

func (vls *VortexLatticeSolver) SolveCirculation(vl *models.VortexLattice, aoa float64, windSpeed float64) {
	n := len(vl.ControlPoints)
	rhs := make([]float64, n)

	aoaRad := aoa * math.Pi / 180.0
	uInf := windSpeed * math.Cos(aoaRad)
	wInf := windSpeed * math.Sin(aoaRad)

	for i, cp := range vl.ControlPoints {
		normalVel := uInf*cp.NormalX + wInf*cp.NormalZ
		rhs[i] = -normalVel
	}

	vl.Circulation = gaussElimination(vl.InfluenceMatrix, rhs)
	vl.TotalVortices = n
}

func gaussElimination(A [][]float64, b []float64) []float64 {
	n := len(b)
	Ac := make([][]float64, n)
	for i := range A {
		Ac[i] = make([]float64, len(A[i]))
		copy(Ac[i], A[i])
	}
	bc := make([]float64, n)
	copy(bc, b)

	for i := 0; i < n; i++ {
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(Ac[k][i]) > math.Abs(Ac[maxRow][i]) {
				maxRow = k
			}
		}
		Ac[i], Ac[maxRow] = Ac[maxRow], Ac[i]
		bc[i], bc[maxRow] = bc[maxRow], bc[i]

		pivot := Ac[i][i]
		if math.Abs(pivot) < 1e-10 {
			continue
		}
		for k := i + 1; k < n; k++ {
			factor := Ac[k][i] / pivot
			for j := i; j < n; j++ {
				Ac[k][j] -= factor * Ac[i][j]
			}
			bc[k] -= factor * bc[i]
		}
	}

	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := bc[i]
		for j := i + 1; j < n; j++ {
			sum -= Ac[i][j] * x[j]
		}
		if math.Abs(Ac[i][i]) > 1e-10 {
			x[i] = sum / Ac[i][i]
		}
	}
	return x
}

type BoundaryLayerSolver struct {
	transitionRe float64
}

func NewBoundaryLayerSolver() *BoundaryLayerSolver {
	return &BoundaryLayerSolver{
		transitionRe: 500000.0,
	}
}

func (bls *BoundaryLayerSolver) CalculateReynoldsNumber(windSpeed, chordLength, kinematicViscosity float64) float64 {
	return windSpeed * chordLength / kinematicViscosity
}

func (bls *BoundaryLayerSolver) CalculateBoundaryLayerThickness(Re, chordLength float64, isTurbulent bool) float64 {
	if isTurbulent {
		return 0.37 * chordLength / math.Pow(Re, 0.2)
	}
	return 4.92 * chordLength / math.Sqrt(Re)
}

func (bls *BoundaryLayerSolver) CalculateSkinFriction(Re float64, isTurbulent bool) float64 {
	if isTurbulent {
		logRe := math.Log10(Re)
		denom := (logRe - 2.58) * (logRe - 2.58)
		if denom < 1e-10 {
			return 0.0045
		}
		return 0.455 / denom
	}
	if Re < 1e-3 {
		return 0
	}
	return 1.328 / math.Sqrt(Re)
}

type AerodynamicSolver struct {
	vortexLattice *VortexLatticeSolver
	boundaryLayer *BoundaryLayerSolver
	vpm           *VPMSolver
	stallAngle    float64
	velocityDist  []float64
	separationX   float64
	isSeparated   bool
	shapeFactor   float64
	vpmParticles  int
}

func NewAerodynamicSolver() *AerodynamicSolver {
	return &AerodynamicSolver{
		vortexLattice: NewVortexLatticeSolver(8, 12),
		boundaryLayer: NewBoundaryLayerSolver(),
		vpm:           NewVPMSolver(20),
		stallAngle:    18.0,
	}
}

func (as *AerodynamicSolver) Solve(sail *models.Sail, sensor *models.SensorData) *models.AerodynamicResult {
	const kinematicViscosity = 1.460e-5

	aoa := as.CalculateAngleOfAttack(sensor.WindDirection, sensor.SailAngle, sensor.Heading)
	aoaAbs := math.Abs(aoa)

	effectiveWindSpeed := as.CalculateEffectiveWindSpeed(sensor.WindSpeed, sensor.ShipSpeed,
		sensor.WindDirection, sensor.Heading)

	result := &models.AerodynamicResult{
		Time:          sensor.Time,
		ShipID:        sensor.ShipID,
		SailID:        sensor.SailID,
		AngleOfAttack: aoa,
		IsStalled:     aoaAbs > as.stallAngle,
	}

	q := 0.5 * sensor.AirDensity * effectiveWindSpeed * effectiveWindSpeed * sail.Area

	cl, cdInduced := as.calculateAirfoilCoefficients(aoa, sail)
	Re := as.boundaryLayer.CalculateReynoldsNumber(effectiveWindSpeed, sail.ChordLength, kinematicViscosity)
	isTurbulent := Re > as.boundaryLayer.transitionRe

	blt := as.boundaryLayer.CalculateBoundaryLayerThickness(Re, sail.ChordLength, isTurbulent)
	cf := as.boundaryLayer.CalculateSkinFriction(Re, isTurbulent)

	cdFriction := 2 * cf * (1 + 2*sail.Camber)
	cdPressure := as.calculatePressureDrag(aoaAbs, sail.Camber)

	cdTotal := cdInduced + cdFriction + cdPressure

	if result.IsStalled {
		cl *= as.stallLiftFactor(aoaAbs)
		cdTotal *= as.stallDragFactor(aoaAbs)
	}

	vl := as.vortexLattice.GenerateLattice(sail)
	as.vortexLattice.CalculateInfluenceMatrix(vl)
	as.vortexLattice.SolveCirculation(vl, aoa, effectiveWindSpeed)

	totalCirculation := 0.0
	for _, gamma := range vl.Circulation {
		totalCirculation += math.Abs(gamma)
	}

	as.velocityDist = as.vpm.ComputeVelocityDistribution(aoa, sail.Camber, sail.ChordLength, cl, 30)
	as.separationX = as.vpm.ComputeSeparationPoint(aoa, sail.Camber, sail.ChordLength, Re, as.velocityDist)
	as.isSeparated = as.vpm.IsSeparated()
	as.shapeFactor = as.vpm.GetShapeFactor()

	as.vpmParticles = as.vpm.GenerateBoundaryLayerParticles(
		sail.ChordLength, sail.SpanLength, sail.Camber,
		aoa, cl, cdPressure, Re, as.velocityDist,
	)

	if as.isSeparated {
		correctedCl, correctedCd := as.vpm.CorrectAerodynamicCoefficients(cl, cdTotal, Re, aoa, sail.AspectRatio)
		cl = correctedCl
		cdTotal = correctedCd

		if aoaAbs < as.stallAngle {
			result.IsStalled = true
		}
	}

	if as.isSeparated && aoaAbs < as.stallAngle {
		sepProgress := (1.0 - as.separationX) / 0.5
		sepProgress = math.Min(1.0, sepProgress)
		cl *= (1.0 - sepProgress*0.15)
		cdTotal *= (1.0 + sepProgress*0.25)
	}

	bltVpm := blt
	if as.isSeparated {
		bltVpm = blt * (1.0 + (1.0-as.separationX)*2.5)
	}

	result.LiftCoefficient = cl
	result.DragCoefficient = cdTotal
	result.LiftForce = cl * q
	result.DragForce = cdTotal * q
	result.PressureDrag = cdPressure * q
	result.FrictionDrag = cdFriction * q
	result.InducedDrag = cdInduced * q
	result.ReynoldsNumber = Re
	result.BoundaryLayerThickness = bltVpm
	result.CirculationStrength = totalCirculation
	result.TotalVortices = vl.TotalVortices + as.vpmParticles

	return result
}

func (as *AerodynamicSolver) CalculateAngleOfAttack(windDirection, sailAngle, heading float64) float64 {
	relativeWind := math.Mod(windDirection-heading+360, 360)
	if relativeWind > 180 {
		relativeWind -= 360
	}
	aoa := relativeWind - sailAngle
	for aoa > 180 {
		aoa -= 360
	}
	for aoa < -180 {
		aoa += 360
	}
	return aoa
}

func (as *AerodynamicSolver) CalculateEffectiveWindSpeed(trueWind, shipSpeed, windDir, heading float64) float64 {
	windRad := windDir * math.Pi / 180.0
	headingRad := heading * math.Pi / 180.0

	wx := trueWind * math.Sin(windRad)
	wy := trueWind * math.Cos(windRad)
	sx := -shipSpeed * math.Sin(headingRad)
	sy := -shipSpeed * math.Cos(headingRad)

	appX := wx + sx
	appY := wy + sy
	return math.Sqrt(appX*appX + appY*appY)
}

func (as *AerodynamicSolver) calculateAirfoilCoefficients(aoa float64, sail *models.Sail) (float64, float64) {
	aoaRad := aoa * math.Pi / 180.0
	AR := sail.AspectRatio
	camber := sail.Camber

	cl0 := 2 * math.Pi * camber
	clAlpha := 2 * math.Pi * AR / (AR + 2)
	cl := cl0 + clAlpha*aoaRad

	cdMin := 0.008 + 0.01*camber*camber
	cdInduced := (cl * cl) / (math.Pi * AR * 0.95)
	cd := cdMin + cdInduced

	return cl, cdInduced
}

func (as *AerodynamicSolver) calculatePressureDrag(aoaAbs, camber float64) float64 {
	aoaRatio := aoaAbs / as.stallAngle
	return 0.02 * camber * math.Pow(aoaRatio, 1.5)
}

func (as *AerodynamicSolver) stallLiftFactor(aoaAbs float64) float64 {
	excess := aoaAbs - as.stallAngle
	return math.Exp(-0.08 * excess)
}

func (as *AerodynamicSolver) stallDragFactor(aoaAbs float64) float64 {
	excess := aoaAbs - as.stallAngle
	return 1.0 + 0.15*excess
}

func (as *AerodynamicSolver) GeneratePolarCurve(sail *models.Sail, windSpeed float64) []models.PolarCurve {
	polars := make([]models.PolarCurve, 0)
	for aoa := -20.0; aoa <= 30.0; aoa += 1.0 {
		cl, cd := as.calculateAirfoilCoefficients(aoa, sail)
		aoaAbs := math.Abs(aoa)
		if aoaAbs > as.stallAngle {
			cl *= as.stallLiftFactor(aoaAbs)
			cdFactor := as.stallDragFactor(aoaAbs)
			cdPressure := as.calculatePressureDrag(aoaAbs, sail.Camber)
			cd = (cd + cdPressure) * cdFactor
		}
		ld := 0.0
		if cd > 1e-6 {
			ld = cl / cd
		}
		polars = append(polars, models.PolarCurve{
			SailID:          sail.SailID,
			WindSpeed:       windSpeed,
			AngleOfAttack:   aoa,
			LiftCoefficient: cl,
			DragCoefficient: cd,
			LiftToDragRatio: ld,
		})
	}
	return polars
}

func (as *AerodynamicSolver) CalculateThrust(lift, drag, aoa, sailAngle, heading, windDir float64) float64 {
	aoaRad := aoa * math.Pi / 180.0
	sailAngleRad := sailAngle * math.Pi / 180.0

	liftAngle := sailAngleRad + math.Pi/2
	liftX := lift * math.Cos(liftAngle)
	liftY := lift * math.Sin(liftAngle)

	dragAngle := sailAngleRad
	dragX := drag * math.Cos(dragAngle)
	dragY := drag * math.Sin(dragAngle)

	headingRad := heading * math.Pi / 180.0
	thrustX := (liftX + dragX) * math.Cos(headingRad)
	thrustY := (liftY + dragY) * math.Sin(headingRad)

	return math.Sqrt(thrustX*thrustX + thrustY*thrustY)
}
