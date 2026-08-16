package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"tracko/internal/application"
	"tracko/internal/domain"
)

type RabbitSubscriber struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	service    application.TrackingService
	queue      string
	exchange   string
	routingKey string
}

func NewRabbitSubscriber(amqpURL, queue, exchange, routingKey string, service application.TrackingService) (*RabbitSubscriber, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("error opening channel in rabbitmq: %w", err)
	}

	if queue == "" {
		queue = "vehicle_telemetry_queue"
	}
	if exchange == "" {
		exchange = "amq.topic"
	}
	if routingKey == "" {
		routingKey = "vehicles.*.location"
	}

	_, err = ch.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("error declaring queue: %w", err)
	}

	err = ch.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("error binding queue to exchange: %w", err)
	}

	return &RabbitSubscriber{
		conn:       conn,
		channel:    ch,
		service:    service,
		queue:      queue,
		exchange:   exchange,
		routingKey: routingKey,
	}, nil
}

func (s *RabbitSubscriber) StartConsuming(ctx context.Context) error {
	msgs, err := s.channel.Consume(
		s.queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("error starting message consumption: %w", err)
	}

	go func() {
		log.Println("[RabbitSubscriber] Listening for telemetry events...")
		for {
			select {
			case <-ctx.Done():
				log.Println("[RabbitSubscriber] Stopping consumption by context cancellation.")
				return
			case d, ok := <-msgs:
				if !ok {
					log.Println("[RabbitSubscriber] Channel closed.")
					return
				}

				var loc domain.Location
				if err := json.Unmarshal(d.Body, &loc); err != nil {
					log.Printf("[RabbitSubscriber] Error deserializing payload: %v", err)
					d.Nack(false, false)
					continue
				}

				if err := s.service.ProcessTelemetry(ctx, &loc); err != nil {
					log.Printf("[RabbitSubscriber] Error processing telemetry: %v", err)
					d.Nack(false, true)
					continue
				}

				d.Ack(false)
			}
		}
	}()

	return nil
}

func (s *RabbitSubscriber) Close() {
	if s.channel != nil {
		s.channel.Close()
	}
	if s.conn != nil {
		s.conn.Close()
	}
}