package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC8: Token.Update()'s Select allow-list must persist trace_enabled and
// trace_otlp_endpoint, including an explicit change to false / a new value.
func TestTokenUpdatePersistsTraceFields(t *testing.T) {
	truncateTables(t)

	token := &Token{
		UserId:            1,
		Key:               "k-trace-1",
		TraceEnabled:      true,
		TraceOtlpEndpoint: "http://x/v1/traces",
	}
	require.NoError(t, token.Insert())

	reloaded, err := GetTokenById(token.Id)
	require.NoError(t, err)
	assert.True(t, reloaded.TraceEnabled)
	assert.Equal(t, "http://x/v1/traces", reloaded.TraceOtlpEndpoint)

	reloaded.TraceEnabled = false
	reloaded.TraceOtlpEndpoint = "http://y/v1/traces"
	require.NoError(t, reloaded.Update())

	final, err := GetTokenById(token.Id)
	require.NoError(t, err)
	assert.False(t, final.TraceEnabled)
	assert.Equal(t, "http://y/v1/traces", final.TraceOtlpEndpoint)
}
