package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/tidwall/gjson"
)

func TestRequestFieldMapsValidateAndCanonicalizePersistHelpers(t *testing.T) {
	legal := []dto.RequestFieldMap{
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
		{From: dto.RequestFieldMapFromServiceTier, To: dto.RequestFieldMapToServiceTier},
	}
	enTrue := true
	if err := dto.ValidateRequestFieldMapsConfig(dto.ChannelOtherSettings{
		RequestFieldMapsEnabled: &enTrue,
		RequestFieldMaps:        legal,
	}); err != nil {
		t.Fatal(err)
	}
	if err := dto.ValidateRequestFieldMaps([]dto.RequestFieldMap{{From: "messages", To: "model"}}); err == nil {
		t.Fatal("expected illegal pair")
	}
	if err := dto.ValidateRequestFieldMaps([]dto.RequestFieldMap{
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
	}); err == nil {
		t.Fatal("expected duplicate to")
	}
	if err := dto.ValidateRequestFieldMapsConfig(dto.ChannelOtherSettings{
		RequestFieldMapsEnabled: &enTrue,
		RequestFieldMaps:        nil,
	}); err == nil {
		t.Fatal("expected fail true+empty")
	}

	ch := &Channel{OtherSettings: `{"__sentinel_unknown__":"keep-me","request_field_maps":[{"from":"output_config.effort","to":"reasoning_effort"},{"from":"service_tier","to":"service_tier"}]}`}
	if err := ch.ValidateSettings(); err != nil {
		t.Fatal(err)
	}
	if err := ch.CanonicalizeRequestFieldMapsInOtherSettings(); err != nil {
		t.Fatal(err)
	}
	if gjson.Get(ch.OtherSettings, "request_field_maps.0.when").String() != dto.RequestFieldMapWhenClaudeToOpenAI {
		t.Fatalf("when0: %s", ch.OtherSettings)
	}
	if gjson.Get(ch.OtherSettings, "__sentinel_unknown__").String() != "keep-me" {
		t.Fatalf("sentinel lost: %s", ch.OtherSettings)
	}
}

func TestRequestFieldMapsEnabledPresenceRoundTrip(t *testing.T) {
	absent := `{"request_field_maps":[{"when":"claude_to_openai","from":"output_config.effort","to":"reasoning_effort"}],"__sentinel_unknown__":"keep"}`
	var s dto.ChannelOtherSettings
	if err := common.UnmarshalJsonStr(absent, &s); err != nil {
		t.Fatal(err)
	}
	if s.RequestFieldMapsEnabled != nil {
		t.Fatal("expected nil enabled")
	}
	if !dto.EffectiveRequestFieldMapsEnabled(s) {
		t.Fatal("absent+maps should be true")
	}
	out, err := dto.CanonicalizeOtherSettingsJSON(absent)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(out, "request_field_maps_enabled").Exists() {
		t.Fatalf("enabled key must stay absent: %s", out)
	}
	if gjson.Get(out, "__sentinel_unknown__").String() != "keep" {
		t.Fatal(out)
	}

	falseJSON := `{"request_field_maps_enabled":false,"request_field_maps":[{"when":"claude_to_openai","from":"output_config.effort","to":"reasoning_effort"}],"__sentinel_unknown__":"keep"}`
	var s2 dto.ChannelOtherSettings
	if err := common.UnmarshalJsonStr(falseJSON, &s2); err != nil {
		t.Fatal(err)
	}
	if s2.RequestFieldMapsEnabled == nil || *s2.RequestFieldMapsEnabled {
		t.Fatal("expected explicit false")
	}
	if dto.EffectiveRequestFieldMapsEnabled(s2) {
		t.Fatal("explicit false must disable")
	}
	out2, err := dto.CanonicalizeOtherSettingsJSON(falseJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.Get(out2, "request_field_maps_enabled").Exists() || gjson.Get(out2, "request_field_maps_enabled").Bool() {
		t.Fatalf("want false key present: %s", out2)
	}
	if gjson.Get(out2, "__sentinel_unknown__").String() != "keep" {
		t.Fatal(out2)
	}
	var s3 dto.ChannelOtherSettings
	if err := common.UnmarshalJsonStr(out2, &s3); err != nil {
		t.Fatal(err)
	}
	if s3.RequestFieldMapsEnabled == nil || *s3.RequestFieldMapsEnabled || dto.EffectiveRequestFieldMapsEnabled(s3) {
		t.Fatal("after canonicalize still explicit false")
	}

	ch := &Channel{OtherSettings: falseJSON}
	if err := ch.ValidateSettings(); err != nil {
		t.Fatal(err)
	}
	if err := ch.CanonicalizeRequestFieldMapsInOtherSettings(); err != nil {
		t.Fatal(err)
	}
	if !gjson.Get(ch.OtherSettings, "request_field_maps_enabled").Exists() || gjson.Get(ch.OtherSettings, "request_field_maps_enabled").Bool() {
		t.Fatalf("channel path lost false: %s", ch.OtherSettings)
	}
}
