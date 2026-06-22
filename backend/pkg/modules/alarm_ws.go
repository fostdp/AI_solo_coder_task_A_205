package modules

import (
	"context"
	"log"
	"sync"
	"time"

	"sail-simulation/pkg/models"
	"sail-simulation/pkg/websocket"
)

type AlarmWS struct {
	alertChan    <-chan *AlertEvent
	hub          *websocket.Hub
	cooldownMap  map[string]time.Time
	cooldownMu  sync.Mutex
	cooldownDur time.Duration
}

func NewAlarmWS(
	alertChan <-chan *AlertEvent,
	hub *websocket.Hub,
	cooldownDur time.Duration,
) *AlarmWS {
	return &AlarmWS{
		alertChan:   alertChan,
		hub:         hub,
		cooldownMap: make(map[string]time.Time),
		cooldownDur: cooldownDur,
	}
}

func (a *AlarmWS) Run(ctx context.Context) {
	log.Println("Alarm/WS module started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Alarm/WS module stopping")
			return
		case alert := <-a.alertChan:
			a.processAlert(alert)
		}
	}
}

func (a *AlarmWS) processAlert(alert *AlertEvent) {
	if alert.Cooldown {
		key := a.checkCooldown(alert.Alert)
		if key == "" {
			log.Printf("Alert throttled: %s %s", alert.Alert.AlertType, alert.Alert.AlertType)
			return
		}
	}

	log.Printf("ALERT [%s] ship=%d sail=%d: %s",
		alert.Alert.Severity,
		alert.Alert.ShipID,
		alert.Alert.SailID,
		alert.Alert.Message)

	a.hub.BroadcastAlert(alert.Alert)
}

func (a *AlarmWS) checkCooldown(alert *models.AlertEvent) string {
	key := alert.CompositeKey()
	now := time.Now()

	a.cooldownMu.Lock()
	defer a.cooldownMu.Unlock()

	if lastSent, exists := a.cooldownMap[key]; exists {
		if now.Sub(lastSent) < a.cooldownDur {
			return ""
		}
	}

	a.cooldownMap[key] = now
	return key
}

func (a *AlarmWS) BroadcastSensor(data *models.SensorData) {
	a.hub.BroadcastSensorData(data)
}

func (a *AlarmWS) BroadcastAero(result *models.AerodynamicResult) {
	a.hub.BroadcastAeroResult(result)
}

func (a *AlarmWS) BroadcastOptimization(result *models.OptimizationResult) {
	a.hub.BroadcastOptimizationResult(result)
}

func (a *AlarmWS) CleanupCooldowns(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.cooldownMu.Lock()
			now := time.Now()
			for k, t := range a.cooldownMap {
				if now.Sub(t) > a.cooldownDur {
					delete(a.cooldownMap, k)
				}
			}
			a.cooldownMu.Unlock()
		}
	}
}
