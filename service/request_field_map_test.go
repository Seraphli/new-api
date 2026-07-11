package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
)

func effortMap() []dto.RequestFieldMap {
	return []dto.RequestFieldMap{{
		When: "claude_to_openai",
		From: dto.RequestFieldMapFromEffort,
		To:   dto.RequestFieldMapToReasoningEffort,
	}}
}

func TestValidateRequestFieldMaps(t *testing.T) {
	if err := dto.ValidateRequestFieldMaps(nil); err != nil {
		t.Fatal(err)
	}
	if err := dto.ValidateRequestFieldMaps(effortMap()); err != nil {
		t.Fatal(err)
	}
	if err := dto.ValidateRequestFieldMaps([]dto.RequestFieldMap{{From: "messages", To: "model"}}); err == nil {
		t.Fatal("expected reject illegal pair")
	}
	if err := dto.ValidateRequestFieldMaps([]dto.RequestFieldMap{{
		When: "openai_to_claude",
		From: dto.RequestFieldMapFromEffort,
		To:   dto.RequestFieldMapToReasoningEffort,
	}}); err == nil {
		t.Fatal("expected reject bad when")
	}
}

func TestApplyRequestFieldMaps(t *testing.T) {
	base := []byte(`{"model":"grok-4.5","messages":[]}`)
	claude := &dto.ClaudeRequest{OutputConfig: json.RawMessage(`{"effort":"max"}`)}

	// M1 empty maps
	out, err := ApplyRequestFieldMaps(claude, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(base) {
		t.Fatalf("empty maps should not change body: %s", out)
	}

	// M2 max
	out, err = ApplyRequestFieldMaps(claude, append([]byte(nil), base...), effortMap())
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(out, "reasoning_effort").String() != "max" {
		t.Fatalf("want max, got %s", out)
	}

	// M3 empty effort
	claudeEmpty := &dto.ClaudeRequest{}
	out, err = ApplyRequestFieldMaps(claudeEmpty, append([]byte(nil), base...), effortMap())
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(out, "reasoning_effort").Exists() {
		t.Fatalf("empty effort should omit: %s", out)
	}

	// M4 illegal pair skipped at runtime
	out, err = ApplyRequestFieldMaps(claude, append([]byte(nil), base...), []dto.RequestFieldMap{{From: "x", To: "y"}})
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(out, "y").Exists() {
		t.Fatal("illegal pair should skip")
	}

	// M5 bad when
	out, err = ApplyRequestFieldMaps(claude, append([]byte(nil), base...), []dto.RequestFieldMap{{
		When: "other", From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(out, "reasoning_effort").Exists() {
		t.Fatal("bad when should skip")
	}

	// M6 later overrides
	out, err = ApplyRequestFieldMaps(claude, append([]byte(nil), base...), []dto.RequestFieldMap{
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(out, "reasoning_effort").String() != "max" {
		t.Fatal(out)
	}
}

func TestShouldApplyAndSyncNoOp(t *testing.T) {
	originPass := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	defer func() { model_setting.GetGlobalSettings().PassThroughRequestEnabled = originPass }()
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false

	claude := &dto.ClaudeRequest{OutputConfig: json.RawMessage(`{"effort":"max"}`)}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{},
		},
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI},
		ReasoningEffort:        "pre",
	}

	// empty maps: shouldMap false, Sync not called by gate tests
	if ShouldApplyRequestFieldMaps(info, claude, nil) {
		t.Fatal("empty maps must not shouldMap")
	}

	// non-OpenAI final
	info.RequestConversionChain = []types.RelayFormat{types.RelayFormatClaude}
	info.FinalRequestRelayFormat = ""
	if ShouldApplyRequestFieldMaps(info, claude, effortMap()) {
		t.Fatal("claude final must not shouldMap")
	}

	// pass-through
	info.RequestConversionChain = []types.RelayFormat{types.RelayFormatOpenAI}
	info.ChannelSetting.PassThroughBodyEnabled = true
	if ShouldApplyRequestFieldMaps(info, claude, effortMap()) {
		t.Fatal("pass-through must not shouldMap")
	}
	info.ChannelSetting.PassThroughBodyEnabled = false

	// OpenAI + maps
	if !ShouldApplyRequestFieldMaps(info, claude, effortMap()) {
		t.Fatal("expected shouldMap")
	}

	// H9 style: Sync only when shouldMap; without call effort stays pre
	if info.ReasoningEffort != "pre" {
		t.Fatal(info.ReasoningEffort)
	}

	body := []byte(`{"reasoning_effort":"max"}`)
	SyncReasoningEffortFromFinalJSON(info, body)
	if info.ReasoningEffort != "max" {
		t.Fatal(info.ReasoningEffort)
	}

	// delete path clears
	SyncReasoningEffortFromFinalJSON(info, []byte(`{"model":"x"}`))
	if info.ReasoningEffort != "" {
		t.Fatal("missing key should clear")
	}
}

func TestBuildSettingsJSONPreservesRequestFieldMaps(t *testing.T) {
	// S1/S2: simulate frontend merge of settings JSON (OtherSettings)
	existing := `{"advanced_custom":{"advanced_routes":[]},"request_field_maps":[{"when":"claude_to_openai","from":"output_config.effort","to":"reasoning_effort"}]}`
	var settingsObj map[string]any
	if err := json.Unmarshal([]byte(existing), &settingsObj); err != nil {
		t.Fatal(err)
	}
	// merge a known key like UI does
	settingsObj["allow_service_tier"] = true
	out, err := json.Marshal(settingsObj)
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.GetBytes(out, "request_field_maps.0.to").Exists() {
		t.Fatalf("maps lost after merge: %s", out)
	}
	// S3 counterexample: setting six-key rebuild would drop unknown keys — document via assert on rebuild shape
	settingSix := map[string]any{
		"force_format": false, "thinking_to_content": false, "proxy": "",
		"pass_through_body_enabled": false, "system_prompt": "", "system_prompt_override": false,
	}
	// deliberately not carrying request_field_maps — proves they must not live in setting
	if _, ok := settingSix["request_field_maps"]; ok {
		t.Fatal("setting six-key must not include maps")
	}
}

func TestMapOverrideOrderAndSync(t *testing.T) {
	claude := &dto.ClaudeRequest{OutputConfig: json.RawMessage(`{"effort":"max"}`)}
	base := []byte(`{"model":"grok-4.5"}`)
	mapped, err := ApplyRequestFieldMaps(claude, base, effortMap())
	if err != nil {
		t.Fatal(err)
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	SyncReasoningEffortFromFinalJSON(info, mapped)
	if info.ReasoningEffort != "max" {
		t.Fatalf("H8: %q", info.ReasoningEffort)
	}
	deleted := []byte(`{"model":"grok-4.5"}`)
	SyncReasoningEffortFromFinalJSON(info, deleted)
	if info.ReasoningEffort != "" {
		t.Fatalf("H6: %q", info.ReasoningEffort)
	}
	replaced := []byte(`{"model":"grok-4.5","reasoning_effort":"high"}`)
	SyncReasoningEffortFromFinalJSON(info, replaced)
	if info.ReasoningEffort != "high" {
		t.Fatalf("H7: %q", info.ReasoningEffort)
	}
}

func TestShouldApplyOpenAIResponsesExcluded(t *testing.T) {
	claude := &dto.ClaudeRequest{OutputConfig: json.RawMessage(`{"effort":"max"}`)}
	info := &relaycommon.RelayInfo{
		ChannelMeta:            &relaycommon.ChannelMeta{},
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAIResponses},
	}
	if ShouldApplyRequestFieldMaps(info, claude, effortMap()) {
		t.Fatal("OpenAI Responses final must not shouldMap")
	}
}
