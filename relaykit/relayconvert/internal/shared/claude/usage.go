package claude

import "github.com/QuantumNous/new-api/relaykit/dto"

func UsageFromOpenAI(usage *dto.Usage) *dto.ClaudeUsage {
	if usage == nil {
		return nil
	}
	// An existing sidecar snapshots the original provider usage; carry it
	// across this bridge unchanged regardless of its dialect. Only synthesize
	// an OpenAI snapshot when no sidecar exists yet.
	existingBillingUsage := dto.CloneBillingUsage(usage.BillingUsage)
	if existingBillingUsage != nil && existingBillingUsage.ClaudeUsage != nil &&
		(existingBillingUsage.Source == dto.BillingUsageSourceClaudeMessages || existingBillingUsage.Semantic == dto.BillingUsageSemanticAnthropic) {
		result := existingBillingUsage.ClaudeUsage
		result.BillingUsage = dto.CloneBillingUsage(usage.BillingUsage)
		return result
	}
	billingUsage := existingBillingUsage
	if billingUsage == nil {
		billingUsage = dto.NewOpenAIChatBillingUsage(usage)
	}
	cacheCreation5m, cacheCreation1h := NormalizeCacheCreationSplit(
		usage.PromptTokensDetails.CachedCreationTokens,
		usage.ClaudeCacheCreation5mTokens,
		usage.ClaudeCacheCreation1hTokens,
	)
	cacheCreationTokens := usage.PromptTokensDetails.CacheCreationTokensTotal()
	cacheReadTokens := usage.PromptTokensDetails.CachedTokens
	inputTokens := usage.PromptTokens
	if usage.UsageSemantic != dto.BillingUsageSemanticAnthropic {
		// OpenAI-style prompt/input totals include cache reads and writes, while
		// Claude reports both separately from input_tokens. Clamp read to [0, prompt]
		// to defend against upstream reimplementations reporting cached_tokens >
		// prompt_tokens.
		if cacheReadTokens < 0 {
			cacheReadTokens = 0
		}
		if cacheReadTokens > usage.PromptTokens {
			cacheReadTokens = usage.PromptTokens
		}
		inputTokens = usage.PromptTokens - cacheReadTokens
		if usage.PromptTokensDetails.CacheWriteTokens > 0 {
			// Subtract cache CREATE only with provenance that it is part of prompt_tokens:
			// the inverse path buildOpenAIStyleUsageFromClaudeUsage sets CacheWriteTokens =
			// cacheCreationTokens and rebuilds prompt = input + read + create, so
			// CacheWriteTokens > 0 means create is genuinely counted inside prompt_tokens.
			inputTokens -= cacheCreationTokens
		}
		if inputTokens < 0 {
			inputTokens = 0
		}
	}
	result := &dto.ClaudeUsage{
		InputTokens:              inputTokens,
		OutputTokens:             usage.CompletionTokens,
		CacheCreationInputTokens: cacheCreationTokens,
		CacheReadInputTokens:     cacheReadTokens,
		BillingUsage:             billingUsage,
	}
	if cacheCreation5m > 0 || cacheCreation1h > 0 {
		result.CacheCreation = &dto.ClaudeCacheCreationUsage{
			Ephemeral5mInputTokens: cacheCreation5m,
			Ephemeral1hInputTokens: cacheCreation1h,
		}
	}
	return result
}
