package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"net/http"

	"tracko/internal/application"
	httpapi "tracko/internal/infrastructure/http"
	"tracko/internal/infrastructure/messaging"
	"tracko/internal/infrastructure/persistence"
	"tracko/internal/infrastructure/websocket"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	postgresDSN := getEnv("POSTGRES_DSN", "postgres://postgres:admin@localhost:5432/tracko?sslmode=disable")
	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	mqttURL := getEnv("MQTT_URL", "tcp://localhost:1883")
	httpAddr := getEnv("HTTP_ADDR", ":8080")

	log.Println("[Main] Initializing tracking microservice...")

	dbPool, err := persistence.NewPostgresPool(ctx, postgresDSN)
	if err != nil {
		log.Fatalf("[Main] Error connecting to PostgreSQL: %v", err)
	}
	defer dbPool.Close()
	log.Println("[Main] PostGIS connection established successfully.")

	repo := persistence.NewPostgresLocationRepository(dbPool)
	tripRepo := persistence.NewPostgresTripRepository(dbPool)
	wsHub := websocket.NewHub()
	go wsHub.Run()
	trackingService := application.NewTrackingService(repo, tripRepo, wsHub)
	wsHandler := websocket.NewWSHandler(wsHub)
	apiHandler := httpapi.NewHandler(trackingService)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "front.html")
	})
	http.Handle("/api/drivers/", apiHandler)
	http.Handle("/api/trips", apiHandler)
	http.Handle("/api/trips/", apiHandler)
	http.HandleFunc("/ws/tracking", wsHandler.ServeWS)

	go func() {
		log.Printf("[Main] HTTP/WebSocket server listening on %s", httpAddr)
		if err := http.ListenAndServe(httpAddr, nil); err != nil {
			log.Fatalf("[Main] HTTP server error: %v", err)
		}
	}()

	subscriber, err := messaging.NewRabbitSubscriber(rabbitURL, trackingService)
	if err != nil {
		log.Fatalf("[Main] Error connecting to RabbitMQ: %v", err)
	}
	defer subscriber.Close()

	if err := subscriber.StartConsuming(ctx); err != nil {
		log.Fatalf("[Main] Error starting consumer: %v", err)
	}

	mqttSubscriber, err := messaging.NewMQTTSubscriber(mqttURL, trackingService)
	if err != nil {
		log.Printf("[Main] MQTT broker not available (%v). mosquitto_pub to localhost:1883 will not reach this service.", err)
	} else {
		defer mqttSubscriber.Close()
		if err := mqttSubscriber.StartConsuming(ctx); err != nil {
			log.Printf("[Main] Error starting MQTT subscriber: %v", err)
		}
	}

	log.Println("[Main] Service running. Open http://localhost:8080 — Press CTRL+C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("[Main] Shutting down the service gracefully...")
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}