package modules

import (
	"time"

	"sail-simulation/pkg/models"
)

type ValidatedSensorData struct {
	Data      *models.SensorData
	Valid     bool
	Error     string
	Processed time.Time
}

type AeroSimRequest struct {
	Sensor *models.SensorData
	Sail   *models.Sail
}

type AeroSimResult struct {
	Sensor *models.SensorData
	Result *models.AerodynamicResult
	Error  error
}

type OptimizationRequest struct {
	Sensor *models.SensorData
	Ship   *models.Ship
	Sail   *models.Sail
}

type OptimizationResult struct {
	Request *OptimizationRequest
	Result  *models.OptimizationResult
	Error   error
}

type AlertEvent struct {
	Alert  *models.AlertEvent
	Cooldown bool
}

type ModuleChannels struct {
	SensorInput      <-chan *models.SensorData
	SensorValidated  chan<- *ValidatedSensorData
	AeroRequest      chan<- *AeroSimRequest
	AeroResult       <-chan *AeroSimResult
	OptimizationReq  chan<- *OptimizationRequest
	OptimizationRes  <-chan *OptimizationResult
	AlertInput       chan<- *AlertEvent
}
