package modules

import (
	"context"
	"encoding/json"
	"log"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	"sail-simulation/pkg/metrics"
	"sail-simulation/pkg/models"
)

type MQTTReceiver struct {
	broker   string
	topic    string
	client   pahomqtt.Client
	outputCh chan<- *models.SensorData
}

func NewMQTTReceiver(broker, topic string, outputCh chan<- *models.SensorData) *MQTTReceiver {
	return &MQTTReceiver{
		broker:   broker,
		topic:    topic,
		outputCh: outputCh,
	}
}

func (m *MQTTReceiver) Start(ctx context.Context) error {
	opts := pahomqtt.NewClientOptions().
		AddBroker(m.broker).
		SetClientID("sail-simulation-mqtt-receiver").
		SetAutoReconnect(true).
		SetMaxReconnectInterval(30 * time.Second).
		SetOnConnectHandler(func(c pahomqtt.Client) {
			log.Printf("MQTT connected to %s", m.broker)
			token := c.Subscribe(m.topic, 1, m.handleMessage)
			token.Wait()
			log.Printf("MQTT subscribed to topic: %s", m.topic)
		}).
		SetConnectionLostHandler(func(c pahomqtt.Client, err error) {
			log.Printf("MQTT connection lost: %v", err)
		})

	m.client = pahomqtt.NewClient(opts)

	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	go func() {
		<-ctx.Done()
		log.Println("MQTT receiver stopping")
		m.client.Disconnect(250)
	}()

	return nil
}

func (m *MQTTReceiver) handleMessage(client pahomqtt.Client, msg pahomqtt.Message) {
	metrics.SensorReceived.WithLabelValues("mqtt").Inc()

	var sensor models.SensorData
	if err := json.Unmarshal(msg.Payload(), &sensor); err != nil {
		log.Printf("MQTT: failed to unmarshal sensor data: %v", err)
		metrics.SensorRejected.WithLabelValues("mqtt_parse_error").Inc()
		return
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

	if !validateSensorFields(&sensor) {
		metrics.SensorRejected.WithLabelValues("mqtt_validation").Inc()
		return
	}

	metrics.SensorValidated.Inc()

	select {
	case m.outputCh <- &sensor:
	default:
		log.Printf("MQTT: output channel full, dropping sensor data for ship=%d sail=%d",
			sensor.ShipID, sensor.SailID)
	}
}

func validateSensorFields(s *models.SensorData) bool {
	if s.ShipID <= 0 || s.ShipID > 3 {
		return false
	}
	if s.SailID <= 0 || s.SailID > 8 {
		return false
	}
	if s.WindSpeed < 0 || s.WindSpeed > 50 {
		return false
	}
	if s.WindDirection < 0 || s.WindDirection > 360 {
		return false
	}
	if s.ShipSpeed < 0 || s.ShipSpeed > 30 {
		return false
	}
	return true
}
