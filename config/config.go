package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	PostgresDSN string

	MQTTURL   string
	MQTTTopic string

	RabbitURL        string
	RabbitQueue      string
	RabbitExchange   string
	RabbitRoutingKey string

	HTTPAddr string
	WSPath   string
	WSAddr   string
}

func Load() Config {
	cfg := Config{
		PostgresDSN:      firstNonEmpty(os.Getenv("POSTGRES_DSN"), buildPostgresDSN()),
		MQTTURL:          firstNonEmpty(os.Getenv("MQTT_URL"), buildMQTTURL()),
		MQTTTopic:        envOr("MQTT_TOPIC", "trips/+/location"),
		RabbitURL:        firstNonEmpty(os.Getenv("RABBITMQ_URL"), buildRabbitURL()),
		RabbitQueue:      envOr("RABBITMQ_QUEUE", "vehicle_telemetry_queue"),
		RabbitExchange:   envOr("RABBITMQ_EXCHANGE", "amq.topic"),
		RabbitRoutingKey: envOr("RABBITMQ_ROUTING_KEY", "vehicles.*.location"),
		HTTPAddr:         httpAddr(),
		WSPath:           wsPath(envOr("WS_PATH", "/ws/tracking")),
		WSAddr:           strings.TrimSpace(os.Getenv("WS_ADDR")),
	}
	return cfg
}

func httpAddr() string {
	if addr := strings.TrimSpace(os.Getenv("HTTP_ADDR")); addr != "" {
		return addr
	}
	return ":" + envOr("HTTP_PORT", "8080")
}

func wsPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func buildPostgresDSN() string {
	user := envOr("POSTGRES_USER", "postgres")
	password := envOr("POSTGRES_PASSWORD", "admin")
	host := envOr("POSTGRES_HOST", "localhost")
	port := envOr("POSTGRES_PORT", "5432")
	db := envOr("POSTGRES_DB", "tracko")
	sslmode := envOr("POSTGRES_SSLMODE", "disable")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   "/" + db,
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

func buildMQTTURL() string {
	host := envOr("MQTT_HOST", "localhost")
	port := envOr("MQTT_PORT", "1883")
	return fmt.Sprintf("tcp://%s:%s", host, port)
}

func buildRabbitURL() string {
	user := envOr("RABBITMQ_USER", "tracko")
	password := envOr("RABBITMQ_PASSWORD", "tracko")
	host := envOr("RABBITMQ_HOST", "localhost")
	port := envOr("RABBITMQ_PORT", "5672")

	u := &url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   "/",
	}
	return u.String()
}

func envOr(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
