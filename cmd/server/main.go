package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tracko/config"
	"tracko/docs"
	"tracko/internal/application"
	httpapi "tracko/internal/infrastructure/http"
	"tracko/internal/infrastructure/messaging"
	"tracko/internal/infrastructure/persistence"
	"tracko/internal/infrastructure/websocket"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()

	log.Println("[Main] Initializing tracking microservice...")

	dbPool, err := persistence.NewPostgresPool(ctx, cfg.PostgresDSN)
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

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/health", httpapi.HealthHandler(dbPool.Ping))
	apiMux.HandleFunc("/openapi.yaml", httpapi.OpenAPIHandler(docs.OpenAPI))
	apiMux.Handle("/swagger/", httpapi.SwaggerUIHandler())
	apiMux.Handle("/api/drivers/", apiHandler)
	apiMux.Handle("/api/trips", apiHandler)
	apiMux.Handle("/api/trips/", apiHandler)

	if cfg.WSAddr == "" {
		apiMux.HandleFunc(cfg.WSPath, wsHandler.ServeWS)
	}

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: apiMux,
	}

	go func() {
		log.Printf("[Main] HTTP server listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Main] HTTP server error: %v", err)
		}
	}()

	var wsServer *http.Server
	if cfg.WSAddr != "" {
		wsMux := http.NewServeMux()
		wsMux.HandleFunc(cfg.WSPath, wsHandler.ServeWS)
		wsServer = &http.Server{
			Addr:    cfg.WSAddr,
			Handler: wsMux,
		}
		go func() {
			log.Printf("[Main] WebSocket server listening on %s%s", cfg.WSAddr, cfg.WSPath)
			if err := wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[Main] WebSocket server error: %v", err)
			}
		}()
	} else {
		log.Printf("[Main] WebSocket available on %s%s", cfg.HTTPAddr, cfg.WSPath)
	}

	subscriber, err := messaging.NewRabbitSubscriber(
		cfg.RabbitURL,
		cfg.RabbitQueue,
		cfg.RabbitExchange,
		cfg.RabbitRoutingKey,
		trackingService,
	)
	if err != nil {
		log.Fatalf("[Main] Error connecting to RabbitMQ: %v", err)
	}
	defer subscriber.Close()

	if err := subscriber.StartConsuming(ctx); err != nil {
		log.Fatalf("[Main] Error starting consumer: %v", err)
	}

	mqttSubscriber, err := messaging.NewMQTTSubscriber(cfg.MQTTURL, cfg.MQTTTopic, trackingService)
	if err != nil {
		log.Fatalf("[Main] Error connecting to MQTT: %v", err)
	}
	defer mqttSubscriber.Close()

	if err := mqttSubscriber.StartConsuming(ctx); err != nil {
		log.Fatalf("[Main] Error starting MQTT subscriber: %v", err)
	}

	log.Printf("[Main] Service running on %s — Press CTRL+C to exit.", cfg.HTTPAddr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("[Main] Shutting down the service gracefully...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Main] HTTP shutdown error: %v", err)
	}
	if wsServer != nil {
		if err := wsServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("[Main] WebSocket shutdown error: %v", err)
		}
	}
}
