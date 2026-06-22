package config

import (
	"encoding/json"
	"os"
)

type VLMConfig struct {
	ChordwisePanels  int     `json:"chordwise_panels"`
	SpanwisePanels   int     `json:"spanwise_panels"`
	VortexCoreRadius float64 `json:"vortex_core_radius"`
}

type BoundaryLayerConfig struct {
	TransitionReynolds float64 `json:"transition_reynolds"`
	KinematicViscosity float64 `json:"kinematic_viscosity"`
}

type StallConfig struct {
	StallAngleDeg        float64 `json:"stall_angle_deg"`
	LiftDecayFactor      float64 `json:"lift_decay_factor"`
	DragIncreaseFactor   float64 `json:"drag_increase_factor"`
}

type VPMConfig struct {
	ChordwisePanels              int     `json:"chordwise_panels"`
	ParticlesPerPanel            int     `json:"particles_per_panel"`
	SeparationShapeFactorThresh  float64 `json:"separation_shape_factor_threshold"`
	TrailingParticlesCount       int     `json:"trailing_particles_count"`
	SeparationLiftCorrection     float64 `json:"separation_lift_correction"`
	SeparationDragBase           float64 `json:"separation_drag_base"`
	SpanEfficiencyBase           float64 `json:"span_efficiency_base"`
	SpanEfficiencySeparated      float64 `json:"span_efficiency_separated"`
	BoundaryLayerExpansionFactor float64 `json:"boundary_layer_expansion_factor"`
	BLSeparationCorrectionLift   float64 `json:"bl_separation_correction_lift"`
	BLSeparationCorrectionDrag   float64 `json:"bl_separation_correction_drag"`
}

type AirfoilConfig struct {
	MinimumDragCoefficient  float64 `json:"minimum_drag_coefficient"`
	CamberDragFactor        float64 `json:"camber_drag_factor"`
	PressureDragBase        float64 `json:"pressure_drag_base"`
	PressureDragExponent    float64 `json:"pressure_drag_exponent"`
}

type DefaultSailConfig struct {
	Area        float64 `json:"area"`
	AspectRatio float64 `json:"aspect_ratio"`
	ChordLength float64 `json:"chord_length"`
	SpanLength  float64 `json:"span_length"`
	Camber      float64 `json:"camber"`
}

type DefaultShipConfig struct {
	HullLength   float64 `json:"hull_length"`
	HullWidth    float64 `json:"hull_width"`
	Displacement float64 `json:"displacement"`
}

type AerodynamicsConfig struct {
	VortexLattice  VLMConfig          `json:"vortex_lattice"`
	BoundaryLayer  BoundaryLayerConfig `json:"boundary_layer"`
	Stall          StallConfig        `json:"stall"`
	VPM            VPMConfig          `json:"vpm"`
	Airfoil        AirfoilConfig      `json:"airfoil"`
	DefaultSail    DefaultSailConfig  `json:"default_sail"`
	DefaultShip    DefaultShipConfig  `json:"default_ship"`
}

type AdamConfig struct {
	Beta1                float64 `json:"beta1"`
	Beta2                float64 `json:"beta2"`
	Epsilon              float64 `json:"epsilon"`
	InitialLearningRate  float64 `json:"initial_learning_rate"`
	MaxLearningRate      float64 `json:"max_learning_rate"`
	LearningRateIncrease float64 `json:"learning_rate_increase"`
	LearningRateDecrease float64 `json:"learning_rate_decrease"`
}

type NesterovConfig struct {
	Enabled       bool    `json:"enabled"`
	LookaheadBias float64 `json:"lookahead_bias"`
}

type BacktrackingConfig struct {
	Enabled       bool    `json:"enabled"`
	Rho           float64 `json:"rho"`
	C             float64 `json:"c"`
	MaxIterations int     `json:"max_iterations"`
	MinStep       float64 `json:"min_step"`
}

type ConvergenceConfig struct {
	MaxIterations            int     `json:"max_iterations"`
	ConvergenceThreshold     float64 `json:"convergence_threshold"`
	MaxConsecutiveNoImprove  int     `json:"max_consecutive_no_improve"`
	GradientEpsilon          float64 `json:"gradient_epsilon"`
}

type GradientConfig struct {
	FiniteDifferenceEps float64 `json:"finite_difference_eps"`
}

type AngleConstraints struct {
	MinAngleDeg float64 `json:"min_angle_deg"`
	MaxAngleDeg float64 `json:"max_angle_deg"`
}

type HullDragConfig struct {
	WaterDensity float64 `json:"water_density"`
	FormFactor   float64 `json:"form_factor"`
	WaveDragCoeff float64 `json:"wave_drag_coeff"`
}

type ThrustConfig struct {
	ShipMassAddition float64 `json:"ship_mass_addition"`
}

type OptimizerConfig struct {
	Adam               AdamConfig         `json:"adam"`
	Nesterov           NesterovConfig     `json:"nesterov"`
	Backtracking       BacktrackingConfig `json:"backtracking_line_search"`
	Convergence        ConvergenceConfig  `json:"convergence"`
	Gradient           GradientConfig     `json:"gradient"`
	AngleConstraints   AngleConstraints   `json:"angle_constraints"`
	HullDrag           HullDragConfig     `json:"hull_drag"`
	Thrust             ThrustConfig       `json:"thrust"`
}

func LoadAerodynamicsConfig(path string) (*AerodynamicsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AerodynamicsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadOptimizerConfig(path string) (*OptimizerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg OptimizerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
