package datastore

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

func TestSyncReconciliationConstraintsIsNoopForNonPostgres(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "reconciliation-*.db")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	database, err := initializeDatabase(&settings.DatabaseConfig{DatabaseType: settings.Sqlite3DbType, DatabasePath: file.Name()})
	require.NoError(t, err)
	assert.NoError(t, database.SyncReconciliationConstraints())
	assert.NoError(t, database.engineGroup.Close())
}
