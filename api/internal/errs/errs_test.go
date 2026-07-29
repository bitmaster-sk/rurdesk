package errs

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromStatus_ConflictMapsToConflictSentinel(t *testing.T) {
	e := FromStatus(http.StatusConflict)
	require.Equal(t, "CONFLICT", e.Code)
	require.Equal(t, "error.conflict", e.TranslateKey)
}

func TestInUseSentinelsCarry409(t *testing.T) {
	require.Equal(t, http.StatusConflict, ErrStateInUse.HttpStatus())
	require.Equal(t, http.StatusConflict, ErrSeverityInUse.HttpStatus())
}

func TestInvalidMigrationTargetSentinelsCarry422(t *testing.T) {
	for _, e := range []*Error{ErrInvalidStateMigrationTarget, ErrInvalidSeverityMigrationTarget} {
		require.Equal(t, http.StatusUnprocessableEntity, e.HttpStatus())
		require.Equal(t, "INVALID_MIGRATION_TARGET", e.Code)
	}
	require.NotEqual(t, ErrInvalidStateMigrationTarget.TranslateKey, ErrInvalidSeverityMigrationTarget.TranslateKey)
}
