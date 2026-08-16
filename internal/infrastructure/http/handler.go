package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"tracko/internal/application"
	"tracko/internal/domain"
)

type Handler struct {
	service application.TrackingService
}

func NewHandler(service application.TrackingService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[0] != "api" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch parts[1] {
	case "drivers":
		h.handleDrivers(w, r, parts)
	case "trips":
		h.handleTrips(w, r, parts)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) handleDrivers(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 4 || parts[2] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	driverID := parts[2]
	switch {
	case r.Method == http.MethodGet && parts[3] == "route":
		h.getDriverRoute(w, r, driverID)
	case r.Method == http.MethodGet && parts[3] == "trips":
		h.listDriverTrips(w, r, driverID)
	case r.Method == http.MethodGet && parts[3] == "location":
		h.getDriverLocation(w, r, driverID)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) handleTrips(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case r.Method == http.MethodPost && len(parts) == 2:
		h.createTrip(w, r)
	case len(parts) == 4 && parts[2] != "" && r.Method == http.MethodGet && parts[3] == "route":
		h.getTripRoute(w, r, parts[2])
	case len(parts) == 3 && parts[2] != "":
		tripID := parts[2]
		switch {
		case r.Method == http.MethodGet:
			h.getTrip(w, r, tripID)
		case r.Method == http.MethodPatch:
			h.updateTrip(w, r, tripID)
		default:
			writeError(w, http.StatusNotFound, "not found")
		}
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) createTrip(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateTripRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	trip, err := h.service.CreateTrip(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, trip)
}

func (h *Handler) getTrip(w http.ResponseWriter, r *http.Request, tripID string) {
	trip, err := h.service.GetTrip(r.Context(), tripID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, trip)
}

func (h *Handler) updateTrip(w http.ResponseWriter, r *http.Request, tripID string) {
	var req domain.UpdateTripRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	trip, err := h.service.UpdateTrip(r.Context(), tripID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, trip)
}

func (h *Handler) listDriverTrips(w http.ResponseWriter, r *http.Request, driverID string) {
	trips, err := h.service.ListTripsByDriver(r.Context(), driverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, trips)
}

func (h *Handler) getDriverLocation(w http.ResponseWriter, r *http.Request, driverID string) {
	loc, err := h.service.GetLastKnownLocation(r.Context(), driverID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "location not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, loc)
}

func HealthHandler(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func (h *Handler) getDriverRoute(w http.ResponseWriter, r *http.Request, driverID string) {
	from, err := parseTimeQuery(r, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid from: use RFC3339 (e.g. 2026-08-14T00:00:00Z)")
		return
	}
	to, err := parseTimeQuery(r, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid to: use RFC3339 (e.g. 2026-08-14T23:59:59Z)")
		return
	}

	route, err := h.service.GetRoute(r.Context(), driverID, from, to)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, route)
}

func (h *Handler) getTripRoute(w http.ResponseWriter, r *http.Request, tripID string) {
	route, err := h.service.GetTripRoute(r.Context(), tripID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return errors.New("invalid request body")
	}
	if len(body) == 0 {
		return errors.New("request body is required")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return errors.New("invalid json")
	}
	return nil
}

func parseTimeQuery(r *http.Request, key string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errors.New("invalid time")
	}
	return &parsed, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
