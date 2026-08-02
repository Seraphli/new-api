package oaichat

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildClaudeUsageFromOpenAIUsage_CacheDoubleCount pins the OpenAI->Claude usage
// mapping so a Claude client summing input_tokens + cache_read_input_tokens never
// double-counts cache reads (fork fix ff61ca88), while preserving upstream's
// CacheWriteTokens>0 accounting and the "no cache-create subtraction without
// provenance" rule. Inputs carry no BillingUsage, so the provenance early-return in
// buildClaudeUsageFromOpenAIUsage is not taken and the token computation runs.
func TestBuildClaudeUsageFromOpenAIUsage_CacheDoubleCount(t *testing.T) {
	tests := []struct {
		name       string
		in         *dto.Usage
		wantInput  int
		wantRead   int
		wantCreate int
		wantOutput int
	}{
		{
			// TC1: plain OpenAI cache read, CacheWriteTokens == 0 -> input excludes read.
			// input = prompt - read = 100000 - 90000 = 10000
			name: "TC1_cache_read_no_write",
			in: &dto.Usage{
				PromptTokens:     100000,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 90000,
				},
			},
			wantInput: 10000, wantRead: 90000, wantCreate: 0, wantOutput: 20,
		},
		{
			// TC2: CacheWriteTokens > 0 provenance -> create subtracted exactly once.
			// cacheCreationTotal = max(CachedCreationTokens=0, CacheWriteTokens=50) = 50
			// input = prompt - read - create = 180 - 30 - 50 = 100
			name: "TC2_cache_write_present",
			in: &dto.Usage{
				PromptTokens:     180,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:     30,
					CacheWriteTokens: 50,
				},
			},
			wantInput: 100, wantRead: 30, wantCreate: 50, wantOutput: 20,
		},
		{
			// TC2b: CachedCreationTokens > 0 but CacheWriteTokens == 0 -> create is NOT
			// subtracted (no provenance it is inside prompt_tokens); read still subtracted.
			// cacheCreationTotal = max(CachedCreationTokens=20, CacheWriteTokens=0) = 20 (reported)
			// input = prompt - read = 180 - 30 = 150
			name: "TC2b_cached_creation_no_write",
			in: &dto.Usage{
				PromptTokens:     180,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: 20,
				},
			},
			wantInput: 150, wantRead: 30, wantCreate: 20, wantOutput: 20,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildClaudeUsageFromOpenAIUsage(tt.in)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantInput, got.InputTokens, "InputTokens")
			assert.Equal(t, tt.wantRead, got.CacheReadInputTokens, "CacheReadInputTokens")
			assert.Equal(t, tt.wantCreate, got.CacheCreationInputTokens, "CacheCreationInputTokens")
			assert.Equal(t, tt.wantOutput, got.OutputTokens, "OutputTokens")
		})
	}
}
