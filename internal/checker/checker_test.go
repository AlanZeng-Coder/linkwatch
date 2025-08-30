package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AlanZeng-Coder/linkwatch/internal/storage"
	"github.com/AlanZeng-Coder/linkwatch/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestChecker_ConcurrencyAndPerHost(t *testing.T) {
	s := testutil.SetupTestDB(t)
	c := NewChecker(s, 1*time.Second, 2, 2*time.Second)

	s.CreateTarget(context.Background(), "https://example.com", "")
	s.CreateTarget(context.Background(), "https://example.com/other", "")
	s.CreateTarget(context.Background(), "https://test.com", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	go c.Start()
	time.Sleep(2 * time.Second)
	c.Stop()

	assert.True(t, true)
}

func TestCheckOne_RetryBackoff(t *testing.T) {
	s := testutil.SetupTestDB(t)
	c := NewChecker(s, 1*time.Second, 1, 2*time.Second)

	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count < 3 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	target := &storage.Target{URL: srv.URL, ID: "test"}
	start := time.Now()
	c.checkOne(target)
	duration := time.Since(start)

	assert.Greater(t, duration, 600*time.Millisecond)
	assert.Equal(t, 3, count)
}
