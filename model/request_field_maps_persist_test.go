package model

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/tidwall/gjson"
)

// B6/B7: Validate + canonicalize behavior on OtherSettings (persist path uses same helpers).
func TestRequestFieldMapsValidateAndCanonicalizePersistHelpers(t *testing.T) {
	// B6 legal multi-row
	legal := []dto.RequestFieldMap{
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
		{From: dto.RequestFieldMapFromServiceTier, To: dto.RequestFieldMapToServiceTier},
	}
	if err := dto.ValidateRequestFieldMaps(legal); err != nil {
		t.Fatal(err)
	}
	// B6 illegal pair
	if err := dto.ValidateRequestFieldMaps([]dto.RequestFieldMap{{From: "messages", To: "model"}}); err == nil {
		t.Fatal("expected illegal pair")
	}
	// B6 duplicate to
	if err := dto.ValidateRequestFieldMaps([]dto.RequestFieldMap{
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
		{From: dto.RequestFieldMapFromEffort, To: dto.RequestFieldMapToReasoningEffort},
	}); err == nil {
		t.Fatal("expected duplicate to")
	}

	// B7 empty when + sentinel
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
	if gjson.Get(ch.OtherSettings, "request_field_maps.1.when").String() != dto.RequestFieldMapWhenClaudeToOpenAI {
		t.Fatalf("when1: %s", ch.OtherSettings)
	}
	if gjson.Get(ch.OtherSettings, "__sentinel_unknown__").String() != "keep-me" {
		t.Fatalf("sentinel lost: %s", ch.OtherSettings)
	}
}
