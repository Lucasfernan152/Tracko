package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"tracko/internal/application"
	"tracko/internal/domain"
)

type MQTTSubscriber struct {
	client  mqtt.Client
	service application.TrackingService
}

func NewMQTTSubscriber(brokerURL string, service application.TrackingService) (*MQTTSubscriber, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(fmt.Sprintf("tracko-subscriber-%d", time.Now().UnixNano())).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectTimeout(5 * time.Second).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			log.Printf("[MQTTSubscriber] Connection lost: %v", err)
		}).
		SetOnConnectHandler(func(_ mqtt.Client) {
			log.Println("[MQTTSubscriber] Connected to MQTT broker.")
		})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("timeout connecting to mqtt broker at %s", brokerURL)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("error connecting to mqtt broker: %w", err)
	}

	return &MQTTSubscriber{
		client:  client,
		service: service,
	}, nil
}

func (s *MQTTSubscriber) StartConsuming(ctx context.Context) error {
	handler := func(_ mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTTSubscriber] Message received on topic %s", msg.Topic())

		var loc domain.Location
		if err := json.Unmarshal(msg.Payload(), &loc); err != nil {
			log.Printf("[MQTTSubscriber] Error deserializing payload: %v", err)
			return
		}

		if loc.TripID == "" {
			if tripID := tripIDFromTopic(msg.Topic()); tripID != "" {
				loc.TripID = tripID
			}
		}

		if err := s.service.ProcessTelemetry(ctx, &loc); err != nil {
			log.Printf("[MQTTSubscriber] Error processing telemetry: %v", err)
		}
	}

	token := s.client.Subscribe("trips/+/location", 1, handler)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout subscribing to mqtt topic")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("error subscribing to mqtt topic: %w", err)
	}

	log.Println("[MQTTSubscriber] Subscribed to trips/+/location")
	return nil
}

func tripIDFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) == 3 && parts[0] == "trips" && parts[2] == "location" {
		return parts[1]
	}
	return ""
}

func (s *MQTTSubscriber) Close() {
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect(250)
	}
}
