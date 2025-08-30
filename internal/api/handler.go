package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AlanZeng-Coder/linkwatch/internal/storage"
)

type Handler struct {
	storage storage.Storage
}

func NewHandler(s storage.Storage) *Handler {
	return &Handler{storage: s}
}

func (h *Handler) PostTarget(w http.ResponseWriter, r *http.Request) {
	var body struct{ URL string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	canonicalURL, err := canonicalizeURL(body.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	idempKey := r.Header.Get("Idempotency-Key")

	target, isNew, err := h.storage.CreateTarget(r.Context(), canonicalURL, idempKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if isNew {
		w.WriteHeader(http.StatusCreated) // 201
	} else {
		w.WriteHeader(http.StatusOK) // 200
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"id": target.ID, "url": target.URL, "created_at": target.CreatedAt.Format(time.RFC3339)})
}

func canonicalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("invalid scheme")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		hostParts := strings.Split(u.Host, ":")
		u.Host = hostParts[0]
	}
	u.Path = strings.TrimRight(u.Path, "/")

	u.Fragment = ""
	return u.String(), nil
}

func (h *Handler) ListTargets(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	token := r.URL.Query().Get("page_token")

	items, next, err := h.storage.ListTargets(r.Context(), host, limit, token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var respItems []map[string]string
	for _, item := range items {
		respItems = append(respItems, map[string]string{"id": item.ID, "url": item.URL})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": respItems, "next_page_token": next})
}

func (h *Handler) GetResults(w http.ResponseWriter, r *http.Request, targetID string) {
	sinceStr := r.URL.Query().Get("since")
	var since time.Time
	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			http.Error(w, "invalid since", http.StatusBadRequest)
			return
		}
	}
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	results, err := h.storage.GetCheckResults(r.Context(), targetID, since, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var respItems []map[string]interface{}
	for _, res := range results {
		item := map[string]interface{}{
			"checked_at":  res.CheckedAt.Format(time.RFC3339),
			"status_code": res.StatusCode,
			"latency_ms":  res.LatencyMs,
		}
		if res.Error != "" {
			item["error"] = res.Error
		} else {
			item["error"] = nil
		}
		respItems = append(respItems, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": respItems})
}
