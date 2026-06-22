package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"sail-simulation/pkg/models"
)

type UDPReceiver struct {
	addr         string
	conn         *net.UDPConn
	outputChan   chan<- *models.SensorData
	validatedChan chan<- *ValidatedSensorData
	bufferSize   int
}

func NewUDPReceiver(addr string, outputChan chan<- *models.SensorData, validatedChan chan<- *ValidatedSensorData) *UDPReceiver {
	return &UDPReceiver{
		addr:         addr,
		outputChan:   outputChan,
		validatedChan: validatedChan,
		bufferSize:   2048,
	}
}

func (r *UDPReceiver) Start(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", r.addr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen UDP: %w", err)
	}
	r.conn = conn

	log.Printf("UDP receiver listening on %s", r.addr)

	go r.receiveLoop(ctx)

	return nil
}

func (r *UDPReceiver) receiveLoop(ctx context.Context) {
	defer r.conn.Close()

	buf := make([]byte, r.bufferSize)

	for {
		select {
		case <-ctx.Done():
			log.Println("UDP receiver stopping")
			return
		default:
		}

		r.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("UDP read error from %v: %v", remoteAddr, err)
			continue
		}

		sensor, validated := r.validateData(buf[:n])
		if validated.Valid {
			select {
			case r.outputChan <- sensor:
			case <-ctx.Done():
				return
			}
		}

		select {
		case r.validatedChan <- validated:
		case <-ctx.Done():
			return
		}
	}
}

func (r *UDPReceiver) validateData(data []byte) (*models.SensorData, *ValidatedSensorData) {
	var sensor models.SensorData
	result := &ValidatedSensorData{
		Data:      &sensor,
		Valid:     true,
		Processed: time.Now(),
	}

	if err := json.Unmarshal(data, &sensor); err != nil {
		result.Valid = false
		result.Error = fmt.Sprintf("JSON parse error: %v", err)
		return &sensor, result
	}

	if sensor.Time.IsZero() {
		sensor.Time = time.Now()
	}
	if sensor.AirDensity == 0 {
		sensor.AirDensity = 1.225
	}
	if sensor.AmbientTemp == 0 {
		sensor.AmbientTemp = 25.0
	}

	if sensor.ShipID <= 0 {
		result.Valid = false
		result.Error = "invalid ship_id"
		return &sensor, result
	}
	if sensor.SailID <= 0 {
		result.Valid = false
		result.Error = "invalid sail_id"
		return &sensor, result
	}
	if sensor.WindSpeed < 0 || sensor.WindSpeed > 50 {
		result.Valid = false
		result.Error = fmt.Sprintf("wind_speed out of range: %.2f", sensor.WindSpeed)
		return &sensor, result
	}
	if sensor.WindDirection < 0 || sensor.WindDirection >= 360 {
		result.Valid = false
		result.Error = fmt.Sprintf("wind_direction out of range: %.2f", sensor.WindDirection)
		return &sensor, result
	}
	if math.IsNaN(sensor.SailAngle) || math.IsInf(sensor.SailAngle, 0) {
		result.Valid = false
		result.Error = "invalid sail_angle"
		return &sensor, result
	}
	if sensor.ShipSpeed < 0 || sensor.ShipSpeed > 30 {
		result.Valid = false
		result.Error = fmt.Sprintf("ship_speed out of range: %.2f", sensor.ShipSpeed)
		return &sensor, result
	}

	log.Printf("Valid sensor: ship=%d sail=%d wind=%.1f angle=%.1f speed=%.2f",
		sensor.ShipID, sensor.SailID, sensor.WindSpeed, sensor.SailAngle, sensor.ShipSpeed)

	return &sensor, result
}
