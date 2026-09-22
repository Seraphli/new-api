package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

// TestUsageFromOpenAI_CacheReadOverflow pins the clamp added for the case where an
// upstream reports CachedTokens >= PromptTokens: cache read must clamp to prompt
// tokens and input tokens must never go negative.
func TestUsageFromOpenAI_CacheReadOverflow(t *testing.T) {
	in := &dto.Usage{
		PromptTokens: 10,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 20,
		},
	}
	got := UsageFromOpenAI(in)
	require.NotNil(t, got)
	require.Equal(t, 0, got.InputTokens)
	require.Equal(t, 10, got.CacheReadInputTokens)
}

// TestUsageFromOpenAI_TC3_golden_replacement_no_write pins the OpenAI->Claude
// usage conversion for a cache read with no cache write, reusing the exact
// input/output values from the retired openai_to_claude.golden.json fixture
// (dropped upstream at rc.37).
func TestUsageFromOpenAI_TC3_golden_replacement_no_write(t *testing.T) {
	in := &dto.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 3,
		},
	}
	got := UsageFromOpenAI(in)
	require.NotNil(t, got)
	require.Equal(t, 7, got.InputTokens)
	require.Equal(t, 3, got.CacheReadInputTokens)
	require.Equal(t, 0, got.CacheCreationInputTokens)
	require.Equal(t, 5, got.OutputTokens)
}
