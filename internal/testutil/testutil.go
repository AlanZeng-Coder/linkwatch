package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/AlanZeng-Coder/linkwatch/internal/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func SetupTestDB(t *testing.T) *storage.SQLiteStorage {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	s := storage.NewSQLiteStorage(db)
	require.NoError(t, s.Init(context.Background()))
	return s
}
