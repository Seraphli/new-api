package service

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ShouldApplyRequestFieldMaps is the single gate for Apply and Sync (plan r3 shouldMap).
func ShouldApplyRequestFieldMaps(info *relaycommon.RelayInfo, claude *dto.ClaudeRequest, maps []dto.RequestFieldMap) bool {
	if info == nil || claude == nil || len(maps) == 0 {
		return false
	}
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		return false
	}
	if info.GetFinalRequestRelayFormat() != types.RelayFormatOpenAI {
		return false
	}
	return true
}

// ApplyRequestFieldMaps writes allowlisted Claude fields onto OpenAI JSON.
// Does not set info.ReasoningEffort (Sync after override).
func ApplyRequestFieldMaps(claude *dto.ClaudeRequest, openAIJSON []byte, maps []dto.RequestFieldMap) ([]byte, error) {
	if claude == nil || len(maps) == 0 {
		return openAIJSON, nil
	}
	out := openAIJSON
	for _, m := range maps {
		when := strings.TrimSpace(m.When)
		if when != "" && when != dto.RequestFieldMapWhenClaudeToOpenAI {
			continue
		}
		from := strings.TrimSpace(m.From)
		to := strings.TrimSpace(m.To)
		if !dto.IsAllowedRequestFieldMapPair(from, to) {
			continue
		}
		var val string
		switch from {
		case dto.RequestFieldMapFromEffort:
			val = claude.GetEfforts()
		case dto.RequestFieldMapFromServiceTier:
			val = strings.TrimSpace(claude.ServiceTier)
		default:
			continue
		}
		if val == "" {
			continue
		}
		var err error
		out, err = sjson.SetBytes(out, to, val)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SyncReasoningEffortFromFinalJSON sets info.ReasoningEffort from final body.
// Caller must ensure shouldMap is true.
func SyncReasoningEffortFromFinalJSON(info *relaycommon.RelayInfo, jsonData []byte) {
	if info == nil {
		return
	}
	if gjson.GetBytes(jsonData, dto.RequestFieldMapToReasoningEffort).Exists() {
		info.ReasoningEffort = gjson.GetBytes(jsonData, dto.RequestFieldMapToReasoningEffort).String()
		return
	}
	info.ReasoningEffort = ""
}
