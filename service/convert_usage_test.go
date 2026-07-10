package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cloneUsage(u *dto.Usage) *dto.Usage {
	if u == nil {
		return nil
	}
	b, err := json.Marshal(u)
	if err != nil {
		panic(err)
	}
	var out dto.Usage
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return &out
}

func assertUsageDeepEqual(t *testing.T, want, got *dto.Usage) {
	t.Helper()
	wb, err := json.Marshal(want)
	require.NoError(t, err)
	gb, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, string(wb), string(gb))
}

func TestBuildClaudeUsageFromOpenAIUsage_Table(t *testing.T) {
	tests := []struct {
		name string
		in   *dto.Usage
		// expected
		input, read, create, out int
		nilUsage                 bool
		nestedNil                bool
		nested5m, nested1h       int
		checkNested              bool
	}{
		{
			name:  "H1 no cache",
			in:    &dto.Usage{PromptTokens: 1000, CompletionTokens: 50},
			input: 1000, read: 0, create: 0, out: 50,
		},
		{
			name: "H2 cache hit empty semantic",
			in: &dto.Usage{
				PromptTokens:     100000,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 90000,
				},
			},
			input: 10000, read: 90000, create: 0, out: 20,
		},
		{
			name: "H3 openai+source=anthropic symmetric",
			in: &dto.Usage{
				PromptTokens:     180,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: 50,
				},
				UsageSemantic: "openai",
				UsageSource:   "anthropic",
			},
			input: 100, read: 30, create: 50, out: 20,
		},
		{
			name: "H4 anthropic passthrough",
			in: &dto.Usage{
				PromptTokens:     100,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: 50,
				},
				UsageSemantic: "anthropic",
			},
			input: 100, read: 30, create: 50, out: 20,
		},
		{
			name: "H5 read>prompt normalize",
			in: &dto.Usage{
				PromptTokens: 10,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 100,
				},
			},
			input: 0, read: 10, create: 0,
		},
		{
			name: "H6 read+create>prompt openai anthropic source",
			in: &dto.Usage{
				PromptTokens: 100,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         80,
					CachedCreationTokens: 50,
				},
				UsageSemantic:               "openai",
				UsageSource:                 "anthropic",
				ClaudeCacheCreation5mTokens: 50,
				ClaudeCacheCreation1hTokens: 0,
			},
			input: 0, read: 80, create: 20,
			// raw 5m=50 > emittedCreate=20 → nested nil
			checkNested: true, nestedNil: true,
		},
		{
			name: "H7 negative cached",
			in: &dto.Usage{
				PromptTokens: 100,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: -5,
				},
			},
			input: 100, read: 0, create: 0,
		},
		{
			name:     "H8 nil",
			in:       nil,
			nilUsage: true,
		},
		{
			name: "H9 normal split s5+s1 < emittedCreate",
			in: &dto.Usage{
				PromptTokens: 180,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: 50,
				},
				UsageSemantic:               "openai",
				UsageSource:                 "anthropic",
				ClaudeCacheCreation5mTokens: 10,
				ClaudeCacheCreation1hTokens: 20,
			},
			input: 100, read: 30, create: 50,
			checkNested: true, nestedNil: false, nested5m: 30, nested1h: 20, // remainder 20 → 5m
		},
		{
			name: "H10 empty semantic with create field",
			in: &dto.Usage{
				PromptTokens: 150,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: 50,
				},
			},
			input: 120, read: 30, create: 50, // create not subtracted from prompt
		},
		{
			name: "H11 openai without anthropic source",
			in: &dto.Usage{
				PromptTokens:     100000,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 90000,
				},
				UsageSemantic: "openai",
			},
			input: 10000, read: 90000, create: 0, out: 20,
		},
		{
			name: "H12 overfull 5m/1h",
			in: &dto.Usage{
				PromptTokens: 100,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         0,
					CachedCreationTokens: 40,
				},
				UsageSemantic:               "openai",
				UsageSource:                 "anthropic",
				ClaudeCacheCreation5mTokens: 30,
				ClaudeCacheCreation1hTokens: 30, // 60 > 40
			},
			input: 60, read: 0, create: 40,
			checkNested: true, nestedNil: true,
		},
		{
			name: "H13 anthropic negative split",
			in: &dto.Usage{
				PromptTokens: 100,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedCreationTokens: 50,
				},
				UsageSemantic:               "anthropic",
				ClaudeCacheCreation5mTokens: -1,
				ClaudeCacheCreation1hTokens: 0,
			},
			input: 100, read: 0, create: 50,
			checkNested: true, nestedNil: true,
		},
		{
			name: "H14 normal s5+s1 < create remainder to 5m",
			in: &dto.Usage{
				PromptTokens: 150,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedCreationTokens: 50,
				},
				UsageSemantic:               "openai",
				UsageSource:                 "anthropic",
				ClaudeCacheCreation5mTokens: 10,
				ClaudeCacheCreation1hTokens: 20,
			},
			input: 100, read: 0, create: 50,
			checkNested: true, nestedNil: false, nested5m: 30, nested1h: 20,
		},
		{
			name: "H15 zero-split",
			in: &dto.Usage{
				PromptTokens: 50,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedCreationTokens: 50,
				},
				UsageSemantic:               "openai",
				UsageSource:                 "anthropic",
				ClaudeCacheCreation5mTokens: 0,
				ClaudeCacheCreation1hTokens: 0,
			},
			input: 0, read: 0, create: 50,
			checkNested: true, nestedNil: false, nested5m: 50, nested1h: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildClaudeUsageFromOpenAIUsage(tt.in)
			if tt.nilUsage {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.input, got.InputTokens, "input")
			assert.Equal(t, tt.read, got.CacheReadInputTokens, "read")
			assert.Equal(t, tt.create, got.CacheCreationInputTokens, "create")
			assert.Equal(t, tt.out, got.OutputTokens, "out")
			// OpenAI-total invariants when not anthropic passthrough
			if tt.in != nil && tt.in.UsageSemantic != "anthropic" {
				if tt.in.UsageSemantic == "openai" && tt.in.UsageSource == "anthropic" {
					assert.Equal(t, tt.in.PromptTokens, got.InputTokens+got.CacheReadInputTokens+got.CacheCreationInputTokens,
						"input+read+create == prompt")
				} else {
					assert.Equal(t, max(0, tt.in.PromptTokens), got.InputTokens+got.CacheReadInputTokens,
						"input+read == prompt")
				}
			}
			if tt.checkNested {
				if tt.nestedNil {
					assert.Nil(t, got.CacheCreation)
				} else {
					require.NotNil(t, got.CacheCreation)
					assert.Equal(t, tt.nested5m, got.CacheCreation.Ephemeral5mInputTokens)
					assert.Equal(t, tt.nested1h, got.CacheCreation.Ephemeral1hInputTokens)
					assert.Equal(t, got.CacheCreationInputTokens,
						got.CacheCreation.Ephemeral5mInputTokens+got.CacheCreation.Ephemeral1hInputTokens)
				}
			}
		})
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestBuildClaudeUsageFromOpenAIUsage_Immutability(t *testing.T) {
	in := &dto.Usage{
		PromptTokens:     100000,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         90000,
			CachedCreationTokens: 10,
		},
		ClaudeCacheCreation5mTokens: 5,
		ClaudeCacheCreation1hTokens: 0,
		UsageSemantic:               "openai",
		UsageSource:                 "anthropic",
	}
	before := cloneUsage(in)
	_ = buildClaudeUsageFromOpenAIUsage(in)
	assertUsageDeepEqual(t, before, in)
}

func TestResponseOpenAI2Claude_UsageMapping(t *testing.T) {
	in := dto.OpenAITextResponse{
		Id:    "chatcmpl_usage",
		Model: "test-model",
		Usage: dto.Usage{
			PromptTokens:     100000,
			CompletionTokens: 20,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: 90000,
			},
		},
		Choices: []dto.OpenAITextResponseChoice{
			{Message: dto.Message{Role: "assistant"}, FinishReason: "stop"},
		},
	}
	// set text content
	in.Choices[0].Message.SetStringContent("hi")

	before := cloneUsage(&in.Usage)
	resp := ResponseOpenAI2Claude(&in, nil)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Usage)
	assert.Equal(t, 10000, resp.Usage.InputTokens)
	assert.Equal(t, 90000, resp.Usage.CacheReadInputTokens)
	assert.Equal(t, 0, resp.Usage.CacheCreationInputTokens)
	assert.Equal(t, 20, resp.Usage.OutputTokens)
	assert.Equal(t, 100000, resp.Usage.InputTokens+resp.Usage.CacheReadInputTokens)
	assertUsageDeepEqual(t, before, &in.Usage)
}

func TestStreamResponseOpenAI2Claude_FinalMessageDeltaUsage(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
		// not first chunk → skip message_start branch
		SendResponseCount: 2,
	}
	finish := "stop"
	usage := &dto.Usage{
		PromptTokens:     100000,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 90000,
		},
	}
	before := cloneUsage(usage)
	// usage-only final chunk (empty choices)
	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_stream",
		Model:   "test-model",
		Choices: nil,
		Usage:   usage,
	}
	// set finish reason via info for stop_reason
	info.FinishReason = finish

	resps := StreamResponseOpenAI2Claude(chunk, info)
	require.NotEmpty(t, resps)
	var delta *dto.ClaudeResponse
	for _, r := range resps {
		if r.Type == "message_delta" {
			delta = r
			break
		}
	}
	require.NotNil(t, delta, "expected message_delta")
	require.NotNil(t, delta.Usage)
	assert.Equal(t, 10000, delta.Usage.InputTokens)
	assert.Equal(t, 90000, delta.Usage.CacheReadInputTokens)
	assert.Equal(t, 20, delta.Usage.OutputTokens)
	assert.Equal(t, 100000, delta.Usage.InputTokens+delta.Usage.CacheReadInputTokens)
	assertUsageDeepEqual(t, before, usage)
}

func TestBuildClaudeUsageFromOpenAIUsage_AnthropicSemanticNoSubtract(t *testing.T) {
	// P5
	in := &dto.Usage{
		PromptTokens: 100,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 50,
		},
		UsageSemantic: "anthropic",
	}
	got := buildClaudeUsageFromOpenAIUsage(in)
	require.NotNil(t, got)
	assert.Equal(t, 100, got.InputTokens)
	assert.Equal(t, 30, got.CacheReadInputTokens)
	assert.Equal(t, 50, got.CacheCreationInputTokens)
}
