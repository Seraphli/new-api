package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateDB_CreatesInviteCodeTable verifies migrateDB() (re)creates the
// invite_codes table via its AutoMigrate call. TestMain already AutoMigrates
// &InviteCode{} before any test runs, so the table is dropped first to make
// sure the assertion actually exercises migrateDB() and not the TestMain setup.
func TestMigrateDB_CreatesInviteCodeTable(t *testing.T) {
	require.NoError(t, DB.Migrator().DropTable(&InviteCode{}))
	assert.False(t, DB.Migrator().HasTable(&InviteCode{}))

	require.NoError(t, migrateDB())

	assert.True(t, DB.Migrator().HasTable(&InviteCode{}))
}
