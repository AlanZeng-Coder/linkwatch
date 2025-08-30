package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) *SQLiteStorage {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	s := NewSQLiteStorage(db)
	if err := s.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestListTargets_Pagination(t *testing.T) {
	s := setupTestDB(t)
	defer s.Close()

	urls := []string{"https://a.com", "https://b.com", "https://c.com"}
	for _, u := range urls {
		_, _, err := s.CreateTarget(context.Background(), u, "")
		assert.NoError(t, err)
		time.Sleep(1000 * time.Millisecond)
	}

	items, next, err := s.ListTargets(context.Background(), "", 1, "")
	assert.NoError(t, err)
	assert.Len(t, items, 1)

	items2, next2, err := s.ListTargets(context.Background(), "", 1, next)
	assert.NoError(t, err)
	assert.Len(t, items2, 1)

	items3, next3, err := s.ListTargets(context.Background(), "", 1, next2)
	assert.NoError(t, err)
	assert.Len(t, items3, 1)
	assert.Empty(t, next3)

	allURLs := []string{items[0].URL, items2[0].URL, items3[0].URL}
	assert.ElementsMatch(t, urls, allURLs)

	itemsHost, _, err := s.ListTargets(context.Background(), "a.com", 10, "")
	assert.NoError(t, err)
	assert.Len(t, itemsHost, 1)
	assert.Equal(t, "https://a.com", itemsHost[0].URL)
}

func TestCreateTarget_Idempotency(t *testing.T) {
	s := setupTestDB(t)
	defer s.Close()

	url := "https://example.com"
	key := "test-key"

	target, isNew, err := s.CreateTarget(context.Background(), url, key)
	assert.NoError(t, err)
	assert.True(t, isNew)
	assert.NotEmpty(t, target.ID)
	assert.Equal(t, url, target.URL)

	target2, isNew2, err := s.CreateTarget(context.Background(), url, key)
	assert.NoError(t, err)
	assert.False(t, isNew2)
	assert.Equal(t, target.ID, target2.ID)

	target3, isNew3, err := s.CreateTarget(context.Background(), url, "")
	assert.NoError(t, err)
	assert.False(t, isNew3)
	assert.Equal(t, target.ID, target3.ID)
}
func TestGetCheckResults(t *testing.T) {
	s := setupTestDB(t)
	defer s.Close()

	target, _, err := s.CreateTarget(context.Background(), "https://test.com", "")
	assert.NoError(t, err)

	results := []*CheckResult{
		{CheckedAt: time.Now().Add(-30 * time.Second), StatusCode: 200, LatencyMs: 100, Error: ""},
		{CheckedAt: time.Now().Add(-10 * time.Second), StatusCode: 404, LatencyMs: 50, Error: "not found"},
	}
	for _, r := range results {
		err := s.SaveCheckResult(context.Background(), target.ID, r)
		assert.NoError(t, err)
	}

	fetched, err := s.GetCheckResults(context.Background(), target.ID, time.Time{}, 10)
	assert.NoError(t, err)
	assert.Len(t, fetched, 2)
	assert.Equal(t, 404, fetched[0].StatusCode)
	assert.Equal(t, 200, fetched[1].StatusCode)

	since := time.Now().Add(-20 * time.Second)
	fetchedSince, err := s.GetCheckResults(context.Background(), target.ID, since, 10)
	assert.NoError(t, err)
	assert.Len(t, fetchedSince, 1)
	assert.Equal(t, 404, fetchedSince[0].StatusCode)
}
