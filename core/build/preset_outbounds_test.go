// SPEC 055 — preset.outbounds expand + merge tests.
package build

import (
	"encoding/json"
	"strings"
	"testing"

	"singbox-launcher/core/state/v6"
	"singbox-launcher/core/template"
)

// === Expand tests ===

func TestExpandPreset_OutboundsAdd(t *testing.T) {
	preset := &template.Preset{
		ID: "ru-inside",
		Outbounds: []template.PresetOutbound{
			{
				Tag:  "ru VPN 🇷🇺",
				Type: "selector",
				Options: map[string]interface{}{
					"default": "direct-out",
				},
				Filters:      map[string]interface{}{"tag": "/(🇷🇺)/i"},
				AddOutbounds: []string{"direct-out"},
			},
		},
	}
	frags, _, ok := ExpandPreset(preset, nil)
	if !ok {
		t.Fatalf("ExpandPreset failed")
	}
	if len(frags.Outbounds) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(frags.Outbounds))
	}
	ob := frags.Outbounds[0]
	if ob.Mode != "add" {
		t.Errorf("expected mode=add (default), got %q", ob.Mode)
	}
	if ob.Tag != "ru VPN 🇷🇺" {
		t.Errorf("expected tag preserved (not prefixed), got %q", ob.Tag)
	}
	// Body не должен содержать control fields.
	if _, has := ob.Body["mode"]; has {
		t.Error("expected mode stripped from Body")
	}
	if _, has := ob.Body["if"]; has {
		t.Error("expected if stripped from Body")
	}
}

func TestExpandPreset_OutboundsUpdateStripsType(t *testing.T) {
	preset := &template.Preset{
		ID: "p1",
		Outbounds: []template.PresetOutbound{
			{
				Mode: "update",
				Tag:  "proxy-out",
				Type: "shadowsocks", // forbidden — should be dropped at expand
				Filters: map[string]interface{}{
					"tag": "!/RU/i",
				},
			},
		},
	}
	frags, _, ok := ExpandPreset(preset, nil)
	if !ok {
		t.Fatalf("ExpandPreset failed")
	}
	ob := frags.Outbounds[0]
	if ob.Mode != "update" {
		t.Errorf("expected mode=update, got %q", ob.Mode)
	}
	if _, has := ob.Body["type"]; has {
		t.Error("expected type stripped from update Body")
	}
	if _, has := ob.Body["filters"]; !has {
		t.Error("expected filters preserved")
	}
}

func TestExpandPreset_OutboundsVarSubstitution(t *testing.T) {
	preset := &template.Preset{
		ID: "p1",
		Vars: []template.PresetVar{
			{Name: "default_node", Type: "text", Default: "node-1"},
		},
		Outbounds: []template.PresetOutbound{
			{
				Tag:  "my-selector",
				Type: "selector",
				Options: map[string]interface{}{
					"default": "@default_node",
				},
			},
		},
	}
	frags, _, ok := ExpandPreset(preset, nil)
	if !ok {
		t.Fatalf("ExpandPreset failed")
	}
	opts := frags.Outbounds[0].Body["options"].(map[string]interface{})
	if opts["default"] != "node-1" {
		t.Errorf("expected @default_node→'node-1', got %v", opts["default"])
	}
}

func TestExpandPreset_OutboundsIfFilter(t *testing.T) {
	preset := &template.Preset{
		ID: "p1",
		Vars: []template.PresetVar{
			{Name: "use_extra", Type: "bool", Default: "false"},
		},
		Outbounds: []template.PresetOutbound{
			{Tag: "always", Type: "selector"},
			{Tag: "conditional", Type: "selector", If: []string{"use_extra"}},
		},
	}
	// use_extra=false → conditional dropped
	frags, _, _ := ExpandPreset(preset, nil)
	if len(frags.Outbounds) != 1 || frags.Outbounds[0].Tag != "always" {
		t.Errorf("expected only 'always', got %+v", frags.Outbounds)
	}
	// use_extra=true → both
	frags, _, _ = ExpandPreset(preset, map[string]string{"use_extra": "true"})
	if len(frags.Outbounds) != 2 {
		t.Errorf("expected 2 outbounds, got %d", len(frags.Outbounds))
	}
}

// === Merge tests ===

func TestMergePresetsIntoOutbounds_Add(t *testing.T) {
	base := []byte(`[{"tag":"proxy-out","type":"selector"}]`)
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "ru-inside", Outbounds: []template.PresetOutbound{
				{Tag: "ru VPN 🇷🇺", Type: "selector"},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "ru-inside", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out, err := MergePresetsIntoOutbounds(base, ctx)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	var arr []map[string]interface{}
	json.Unmarshal(out, &arr)
	if len(arr) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(arr))
	}
	if arr[0]["tag"] != "proxy-out" || arr[1]["tag"] != "ru VPN 🇷🇺" {
		t.Errorf("unexpected order/tags: %+v", arr)
	}
}

func TestMergePresetsIntoOutbounds_UpdateFilters(t *testing.T) {
	// proxy-out has existing outbounds list; preset update filters !RU regex
	// should drop RU-tagged entries from outbounds. filters field itself must
	// NOT appear in final (sing-box 1.12+ rejects unknown field).
	base := []byte(`[{"tag":"proxy-out","type":"selector","options":{"default":"x"},"outbounds":["a-EU","b-🇷🇺-RU","c-US"]}]`)
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "ru-inside", Outbounds: []template.PresetOutbound{
				{Mode: "update", Tag: "proxy-out",
					Filters: map[string]interface{}{"tag": "!/🇷🇺/i"}},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "ru-inside", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out, _ := MergePresetsIntoOutbounds(base, ctx)
	var arr []map[string]interface{}
	json.Unmarshal(out, &arr)
	if len(arr) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(arr))
	}
	po := arr[0]
	if _, has := po["filters"]; has {
		t.Errorf("filters field MUST be stripped after resolve; got: %+v", po)
	}
	opts := po["options"].(map[string]interface{})
	if opts["default"] != "x" {
		t.Errorf("options.default lost: %+v", opts)
	}
	outs, _ := po["outbounds"].([]interface{})
	got := make([]string, 0, len(outs))
	for _, x := range outs {
		got = append(got, x.(string))
	}
	want := []string{"a-EU", "c-US"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("outbounds after !RU filter: got %v, want %v", got, want)
	}
}

func TestMergePresetsIntoOutbounds_UpdateAddOutboundsUnion(t *testing.T) {
	// addOutbounds in patch should append to target.outbounds (not stay as
	// separate addOutbounds field — sing-box doesn't understand it).
	base := []byte(`[{"tag":"proxy-out","type":"selector","outbounds":["a","b"]}]`)
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "p1", Outbounds: []template.PresetOutbound{
				{Mode: "update", Tag: "proxy-out", AddOutbounds: []string{"b", "c"}},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "p1", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out, _ := MergePresetsIntoOutbounds(base, ctx)
	var arr []map[string]interface{}
	json.Unmarshal(out, &arr)
	if _, has := arr[0]["addOutbounds"]; has {
		t.Errorf("addOutbounds field MUST be stripped after merge; got: %+v", arr[0])
	}
	outs, _ := arr[0]["outbounds"].([]interface{})
	got := make([]string, 0, len(outs))
	for _, x := range outs {
		got = append(got, x.(string))
	}
	want := []string{"a", "b", "c"} // existing + addOutbounds (union)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("union expected %v, got %v", want, got)
	}
}

func TestMergePresetsIntoOutbounds_UpdateMissingTargetSkipped(t *testing.T) {
	base := []byte(`[{"tag":"proxy-out","type":"selector"}]`)
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "p1", Outbounds: []template.PresetOutbound{
				{Mode: "update", Tag: "nonexistent",
					Filters: map[string]interface{}{"tag": "/x/"}},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "p1", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out, _ := MergePresetsIntoOutbounds(base, ctx)
	var arr []map[string]interface{}
	json.Unmarshal(out, &arr)
	if len(arr) != 1 || arr[0]["tag"] != "proxy-out" {
		t.Errorf("base outbound should be untouched: %+v", arr)
	}
}

func TestMergePresetsIntoOutbounds_AddCollisionFirstWins(t *testing.T) {
	// Two presets both add "shared" — first (by RuleOrder) wins.
	base := []byte(`[]`)
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "p1", Outbounds: []template.PresetOutbound{
				{Tag: "shared", Type: "selector",
					Options: map[string]interface{}{"default": "first"}},
			}},
			{ID: "p2", Outbounds: []template.PresetOutbound{
				{Tag: "shared", Type: "urltest",
					Options: map[string]interface{}{"default": "second"}},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "p1", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
			{Kind: v6.RuleKindPreset, Ref: "p2", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out, _ := MergePresetsIntoOutbounds(base, ctx)
	var arr []map[string]interface{}
	json.Unmarshal(out, &arr)
	if len(arr) != 1 {
		t.Fatalf("expected 1 outbound (collision dedup), got %d", len(arr))
	}
	if arr[0]["type"] != "selector" {
		t.Errorf("expected first-wins (selector), got: %v", arr[0])
	}
}

func TestMergePresetsIntoOutbounds_DisabledPresetIgnored(t *testing.T) {
	base := []byte(`[{"tag":"proxy-out","type":"selector"}]`)
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "p1", Outbounds: []template.PresetOutbound{
				{Tag: "new", Type: "selector"},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "p1", Enabled: false,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out, _ := MergePresetsIntoOutbounds(base, ctx)
	var arr []map[string]interface{}
	json.Unmarshal(out, &arr)
	if len(arr) != 1 {
		t.Errorf("disabled preset should not add outbounds, got %+v", arr)
	}
}

// === Dangling cleanup tests ===

func TestCleanDanglingOutboundRefInRule_KeepsValid(t *testing.T) {
	rule := map[string]interface{}{"outbound": "proxy-out", "domain": "x"}
	emitted := map[string]bool{"proxy-out": true}
	out := cleanDanglingOutboundRefInRule(rule, emitted, "")
	if out["outbound"] != "proxy-out" {
		t.Errorf("valid outbound should be kept: %+v", out)
	}
}

func TestCleanDanglingOutboundRefInRule_FallbackToFinal(t *testing.T) {
	rule := map[string]interface{}{"outbound": "ghost", "domain": "x"}
	emitted := map[string]bool{"proxy-out": true}
	out := cleanDanglingOutboundRefInRule(rule, emitted, "proxy-out")
	if out["outbound"] != "proxy-out" {
		t.Errorf("dangling ref should fallback to final 'proxy-out', got: %+v", out)
	}
}

func TestCleanDanglingOutboundRefInRule_DropWhenNoFinal(t *testing.T) {
	rule := map[string]interface{}{"outbound": "ghost", "domain": "x"}
	emitted := map[string]bool{}
	out := cleanDanglingOutboundRefInRule(rule, emitted, "")
	if out != nil {
		t.Errorf("expected nil (drop), got: %+v", out)
	}
}

func TestCleanDanglingOutboundRefInRule_SentinelsPreserved(t *testing.T) {
	for _, sent := range []string{"reject", "drop"} {
		rule := map[string]interface{}{"outbound": sent, "domain": "x"}
		out := cleanDanglingOutboundRefInRule(rule, map[string]bool{}, "fallback")
		if out["outbound"] != sent {
			t.Errorf("sentinel %q should be preserved, got: %+v", sent, out)
		}
	}
}

// === SPEC 055 phase 2: ApplyPresetUpdatesToGeneratedOutbounds ===

func TestApplyPresetUpdatesToGeneratedOutbounds_PatchesFilters(t *testing.T) {
	// Generated cache has parser-emitted proxy-out without filters.
	cache := []string{
		"\t{\"tag\":\"proxy-out\",\"type\":\"selector\",\"outbounds\":[\"a\",\"b\"]},",
		"\t{\"tag\":\"direct-out\",\"type\":\"direct\"}",
	}
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "russian", Outbounds: []template.PresetOutbound{
				{Mode: "update", Tag: "proxy-out",
					Filters: map[string]interface{}{"tag": "!/RU/i"}},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "russian", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	// Filter !/RU/i applied to existing outbounds ["a", "b"] — both match
	// (neither contains "RU"), so both should remain. filters field MUST be
	// stripped after resolution (sing-box 1.12+ unknown field).
	cache[0] = "\t{\"tag\":\"proxy-out\",\"type\":\"selector\",\"outbounds\":[\"a-EU\",\"b-RU\",\"c-US\"]},"
	out := ApplyPresetUpdatesToGeneratedOutbounds(cache, ctx)
	if len(out) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(out))
	}
	var m map[string]interface{}
	clean := strings.TrimRight(strings.TrimSpace(out[0]), ",")
	if err := json.Unmarshal([]byte(clean), &m); err != nil {
		t.Fatalf("parse patched: %v", err)
	}
	if _, has := m["filters"]; has {
		t.Errorf("filters field MUST be stripped after resolve: %+v", m)
	}
	outs, _ := m["outbounds"].([]interface{})
	got := make([]string, 0, len(outs))
	for _, x := range outs {
		got = append(got, x.(string))
	}
	want := []string{"a-EU", "c-US"} // b-RU dropped by !/RU/i
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("filtered outbounds: got %v, want %v", got, want)
	}
	// Second entry (direct-out) — без изменений.
	if !strings.Contains(out[1], "\"direct\"") {
		t.Errorf("direct-out should be unchanged: %q", out[1])
	}
}

func TestApplyPresetUpdatesToGeneratedOutbounds_NoUpdatesNoop(t *testing.T) {
	cache := []string{"\t{\"tag\":\"proxy-out\",\"type\":\"selector\"},"}
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "russian", Outbounds: []template.PresetOutbound{
				{Tag: "ru VPN", Type: "selector"}, // mode=add, не update
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "russian", Enabled: true,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out := ApplyPresetUpdatesToGeneratedOutbounds(cache, ctx)
	if out[0] != cache[0] {
		t.Errorf("no-op expected (no update patches), got change: %q", out[0])
	}
}

func TestApplyPresetUpdatesToGeneratedOutbounds_DisabledPresetIgnored(t *testing.T) {
	cache := []string{"\t{\"tag\":\"proxy-out\",\"type\":\"selector\"},"}
	ctx := PresetMergeContext{
		Presets: []template.Preset{
			{ID: "russian", Outbounds: []template.PresetOutbound{
				{Mode: "update", Tag: "proxy-out",
					Filters: map[string]interface{}{"tag": "!/RU/i"}},
			}},
		},
		RulesV6: []v6.Rule{
			{Kind: v6.RuleKindPreset, Ref: "russian", Enabled: false,
				Body: mustMarshal(t, v6.PresetBody{})},
		},
	}
	out := ApplyPresetUpdatesToGeneratedOutbounds(cache, ctx)
	if out[0] != cache[0] {
		t.Errorf("disabled preset must not patch: %q", out[0])
	}
}

// === Helper ===

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
