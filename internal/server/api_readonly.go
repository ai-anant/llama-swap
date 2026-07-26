package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mostlygeek/llama-swap/internal/router"
)

// Read-only API v1 — serves raw metrics data for mobile/analytics clients.
// All computation is done client-side; the server just returns the data.

// handleAPIV1Metrics serves raw metrics with optional filtering.
// Query params:
//
//	?since=<RFC3339>  — only entries after this timestamp
//	?until=<RFC3339>  — only entries before this timestamp
//	?model=<name>     — filter by exact model name
//	?limit=<n>        — max entries (default 1000, max 10000)
//	?offset=<n>       — skip first N entries
func (s *Server) handleAPIV1Metrics(w http.ResponseWriter, r *http.Request) {
	all := s.metrics.getMetrics()

	// Apply filters
	since, until, model := parseMetricsQuery(r)

	var filtered []ActivityLogEntry
	for _, m := range all {
		if since != nil && m.Timestamp.Before(*since) {
			continue
		}
		if until != nil && m.Timestamp.After(*until) {
			continue
		}
		if model != "" && !strings.EqualFold(m.Model, model) {
			continue
		}
		filtered = append(filtered, m)
	}

	// Pagination
	limit := 1000
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
			if limit > 10000 {
				limit = 10000
			}
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v > 0 {
			offset = v
		}
	}

	if offset >= len(filtered) {
		filtered = nil
	} else {
		end := offset + limit
		if end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[offset:end]
	}

	if filtered == nil {
		filtered = []ActivityLogEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  filtered,
		"total": len(s.metrics.getMetrics()), // unfiltered total
	})
}

// handleAPIV1MetricsByID serves a single metric entry by its sequential ID.
func (s *Server) handleAPIV1MetricsByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		router.SendResponse(w, r, http.StatusBadRequest, "invalid metric ID")
		return
	}

	all := s.metrics.getMetrics()
	for _, m := range all {
		if m.ID == id {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			_ = json.NewEncoder(w).Encode(m)
			return
		}
	}

	router.SendResponse(w, r, http.StatusNotFound, "metric not found")
}

// handleAPIV1Models lists unique model names that appear in metrics.
func (s *Server) handleAPIV1Models(w http.ResponseWriter, r *http.Request) {
	all := s.metrics.getMetrics()
	seen := make(map[string]struct{}, 32)
	for _, m := range all {
		if m.Model != "" {
			seen[m.Model] = struct{}{}
		}
	}

	models := make([]string, 0, len(seen))
	for name := range seen {
		models = append(models, name)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  models,
		"total": len(models),
	})
}

// handleAPIV1Health is a lightweight read-only health check.
func (s *Server) handleAPIV1Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// parseMetricsQuery extracts common filter params from the request.
func parseMetricsQuery(r *http.Request) (since, until *time.Time, model string) {
	q := r.URL.Query()

	if s := q.Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = &t
		}
	}
	if u := q.Get("until"); u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			until = &t
		}
	}
	model = q.Get("model")
	return
}
