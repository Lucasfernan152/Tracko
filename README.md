# Tracko

Self-hosted live tracking service. You run it; your publishers send GPS; your clients read the HTTP API and WebSocket feed.

Tracko stores trip telemetry in PostGIS and accepts locations over MQTT and RabbitMQ.

## Quick start

```bash
cp .env.example .env
docker compose up --build
```

The API listens on `http://localhost:8080` (override with `TRACKO_HTTP_PORT` / `HTTP_PORT`).

Check it:

```bash
curl http://localhost:8080/health
```

## API docs (Swagger)

After the service is running:

- Swagger UI: [http://localhost:8080/swagger/](http://localhost:8080/swagger/)
- OpenAPI 3 spec: [http://localhost:8080/openapi.yaml](http://localhost:8080/openapi.yaml) (source: [`docs/openapi.yaml`](docs/openapi.yaml))

The UI is embedded in the binary (no CDN). **Try it out** calls this instance, so Postgres, Mosquitto, and RabbitMQ must be up (`docker compose up --build` or the local stack in [Run without Docker](#run-without-docker)). You can still read the spec file without starting anything.

## Use your own infrastructure

Set connection env vars and Tracko will not assume localhost defaults from inside Compose.

**External Postgres** (DSN wins over the individual pieces):

```env
POSTGRES_DSN=postgres://user:pass@192.168.1.10:5432/tracko?sslmode=disable
```

Apply [`migrations/001_init.sql`](migrations/001_init.sql) on that database (PostGIS required). You can still start only the brokers and the API:

```bash
docker compose up --build tracko mosquitto rabbitmq
```

Compose will also start the bundled Postgres container because Tracko waits for it. Point `POSTGRES_DSN` at your server and ignore the local one, or drop the `postgres` service in an override file.

**External MQTT / RabbitMQ:**

```env
MQTT_URL=tcp://mqtt.internal:1883
RABBITMQ_URL=amqp://tracko:tracko@rabbit.internal:5672/
```

Or set `MQTT_HOST` / `RABBITMQ_HOST` (and ports/credentials) instead of the full URL.

## Environment

Two layers: process config (what Tracko connects to and listens on) and Compose publish ports (what the host exposes).

### Process

| Variable | Default | Role |
|---|---|---|
| `POSTGRES_DSN` | built from pieces | Full DSN; wins if set |
| `POSTGRES_HOST` / `PORT` / `USER` / `PASSWORD` / `DB` / `SSLMODE` | `localhost` `5432` `postgres` `admin` `tracko` `disable` | Build the DSN (`postgres` / `mosquitto` / `rabbitmq` inside Compose) |
| `MQTT_URL` | built from pieces | `tcp://host:port` |
| `MQTT_HOST` / `MQTT_PORT` | `localhost` `1883` | MQTT broker |
| `MQTT_TOPIC` | `trips/+/location` | Subscribe filter |
| `RABBITMQ_URL` | built from pieces | `amqp://user:pass@host:port/` |
| `RABBITMQ_HOST` / `PORT` / `USER` / `PASSWORD` | `localhost` `5672` `tracko` `tracko` | AMQP broker |
| `RABBITMQ_QUEUE` / `EXCHANGE` / `ROUTING_KEY` | `vehicle_telemetry_queue` `amq.topic` `vehicles.*.location` | Queue binding |
| `HTTP_ADDR` | `:{HTTP_PORT}` | API listen address |
| `HTTP_PORT` | `8080` | API port when `HTTP_ADDR` is empty |
| `WS_PATH` | `/ws/tracking` | WebSocket path |
| `WS_ADDR` | empty | If set (e.g. `:8081`), WebSocket listens on a separate port |

Local `go run ./cmd/server` uses the `localhost` defaults. Compose overrides hosts to service names unless you set them in `.env`.

### Compose host ports

| Variable | Default | Publishes |
|---|---|---|
| `TRACKO_HTTP_PORT` | `8080` | API (and WebSocket if `WS_ADDR` is empty) |
| `TRACKO_WS_PORT` | unset | Optional; add a port mapping if `WS_ADDR` is set |
| `MQTT_HOST_PORT` | `1883` | Mosquitto |
| `RABBITMQ_AMQP_PORT` | `5672` | AMQP |
| `RABBITMQ_MGMT_PORT` | `15672` | RabbitMQ management UI |
| `POSTGRES_HOST_PORT` | `5432` | Postgres |

Example: API on 9090, MQTT on 1884, your own Postgres:

```env
HTTP_PORT=9090
TRACKO_HTTP_PORT=9090
MQTT_HOST_PORT=1884
POSTGRES_DSN=postgres://user:pass@192.168.1.10:5432/tracko?sslmode=disable
```

To expose WebSocket on another host port, set `WS_ADDR=:8081` and add this to the `tracko` service `ports:` list:

```yaml
- "${TRACKO_WS_PORT:-8081}:8081"
```

## Trip lifecycle

1. Create a trip: `POST /api/trips`
2. Assign a driver: `PATCH /api/trips/{id}` with `{"driver_id":"..."}`
3. Start it: `PATCH /api/trips/{id}` with `{"status":"in_progress"}`
4. Publish GPS to MQTT or RabbitMQ
5. Complete or cancel: `PATCH` `completed` / `cancelled`

Telemetry is accepted only while the trip is `in_progress`.

## Publish location

### MQTT

Topic: `trips/{trip_id}/location` (or the `MQTT_TOPIC` filter you configured).

```json
{
  "trip_id": "trip-...",
  "driver_id": "driver-123",
  "latitude": -34.6037,
  "longitude": -58.3816,
  "speed": 28,
  "heading": 90,
  "timestamp": "2026-08-16T14:00:00Z"
}
```

`trip_id` can be omitted if it is already in the topic. If `timestamp` is omitted, the server stores `now` UTC.

```bash
mosquitto_pub -h localhost -t "trips/TRIP_ID/location" -m '{"latitude":-34.6037,"longitude":-58.3816,"speed":28,"heading":90}'
```

### RabbitMQ

Routing key: `vehicles.{id}.location` on `amq.topic` (override with `RABBITMQ_ROUTING_KEY` / `RABBITMQ_EXCHANGE`). Same JSON body.

### Demo publisher

[`scripts/simulate_ba_route.sh`](scripts/simulate_ba_route.sh) creates a trip, starts it, and publishes a Buenos Aires path over MQTT.

```bash
./scripts/simulate_ba_route.sh --api-base http://localhost:8080 --broker localhost --port 1883
```

Requires `curl` and `mosquitto_pub`.

## HTTP API

See [API docs (Swagger)](#api-docs-swagger) for the interactive reference.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Postgres ping |
| `POST` | `/api/trips` | Create trip (`{"metadata":{...}}`) |
| `GET` | `/api/trips/{id}` | Get trip |
| `PATCH` | `/api/trips/{id}` | Assign driver / change status / merge metadata |
| `GET` | `/api/trips/{id}/route` | Points for that trip |
| `GET` | `/api/drivers/{id}/trips` | Trips for a driver |
| `GET` | `/api/drivers/{id}/route` | Points (`?from=` / `?to=` RFC3339) |
| `GET` | `/api/drivers/{id}/location` | Last known point |

Live feed: `ws://localhost:8080/ws/tracking` (or `WS_PATH` / `WS_ADDR`).

RabbitMQ management UI (bundled stack): `http://localhost:15672` (`tracko` / `tracko` by default).

## Run without Docker

You need PostGIS, a Mosquitto broker, and RabbitMQ. Apply `migrations/001_init.sql`, then:

```bash
go run ./cmd/server
```

## License

MIT. See [LICENSE](LICENSE).
