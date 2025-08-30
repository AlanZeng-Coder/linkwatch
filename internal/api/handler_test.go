package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AlanZeng-Coder/linkwatch/internal/testutil"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestCanonicalizeURL(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
		err      bool
	}{
		{"https://EXAMPLE.com/", "https://example.com", false},
		{"HTTP://example.com:80/path/", "http://example.com/path", false},
		{"https://example.com:443", "https://example.com", false},
		{"ftp://invalid.com", "", true},
		{"https://example.com#fragment", "https://example.com", false},
	}

	for _, tt := range tests {
		res, err := canonicalizeURL(tt.raw)
		if tt.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, res)
		}
	}
}

func TestPostTarget(t *testing.T) {
	s := testutil.SetupTestDB(t)
	h := NewHandler(s)

	body := bytes.NewBufferString(`{"url": "https://example.com"}`)
	req := httptest.NewRequest("POST", "/v1/targets", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostTarget(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "https://example.com", resp["url"])

	body.Reset()
	body.WriteString(`{"url": "https://example.com"}`)
	req = httptest.NewRequest("POST", "/v1/targets", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	h.PostTarget(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListTargets(t *testing.T) {
	s := testutil.SetupTestDB(t)
	h := NewHandler(s)

	s.CreateTarget(context.Background(), "https://example.com", "")
	s.CreateTarget(context.Background(), "https://test.com", "")

	req := httptest.NewRequest("GET", "/v1/targets?limit=1", nil)
	w := httptest.NewRecorder()

	h.ListTargets(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	items := resp["items"].([]interface{})
	assert.Len(t, items, 1)
	assert.NotEmpty(t, resp["next_page_token"])
}
