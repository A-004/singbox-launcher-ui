package presentation

import (
	"testing"

	"singbox-launcher/core/state"
	wizardmodels "singbox-launcher/ui/configurator/models"
)

// TestSyncDNSByOrderToState_RoundTrip — SPEC 062-F-N: build a state.DNS.Rules
// slice with interleaved kind=preset and kind=user rules in non-canonical
// order (user before preset, then another preset, then another user) →
// derive DNSRuleOrder + DNSUserRules via DNSRuleOrderFromStateRules →
// re-emit via SyncDNSByOrderToState → assert the resulting slice matches
// the original order (round-trip preservation).
func TestSyncDNSByOrderToState_RoundTrip(t *testing.T) {
	// Seed: state.DNS.Rules in mixed order.
	original := []state.DNSRule{
		{Kind: state.DNSRuleKindUser, Enabled: true, Body: map[string]interface{}{
			"domain": "mysite.ru", "server": "my-dns",
		}},
		{Kind: state.DNSRuleKindPreset, Ref: "russian", Enabled: true},
		{Kind: state.DNSRuleKindUser, Enabled: true, Body: map[string]interface{}{
			"domain_suffix": "ads.example.com", "server": "block-dns",
		}},
		{Kind: state.DNSRuleKindPreset, Ref: "ip-fallback", Enabled: false},
	}

	// PresetRefs as restored from state.Rules — must contain russian and ip-fallback.
	presetRefs := []*wizardmodels.PresetRefState{
		{Ref: "russian", Enabled: true},
		{Ref: "ip-fallback", Enabled: true}, // preset itself enabled in route; dns_rule toggle is separate
	}

	order, userRules := wizardmodels.DNSRuleOrderFromStateRules(original, presetRefs)
	if len(order) != 4 {
		t.Fatalf("expected 4 slots in order, got %d", len(order))
	}
	if len(userRules) != 2 {
		t.Fatalf("expected 2 user rules, got %d", len(userRules))
	}

	// Set DNSRuleEnabled on second preset so round-trip preserves the disabled toggle.
	{
		falseV := false
		presetRefs[1].DNSRuleEnabled = &falseV
	}

	// Re-emit through SyncDNSByOrderToState.
	out := wizardmodels.SyncDNSByOrderToState(
		order,
		presetRefs,
		userRules,
		nil, // dnsServers — empty; we only care about rules portion
		"",  // dnsRulesText — empty; order non-empty wins
		nil, nil,
	)

	if len(out.Rules) != 4 {
		t.Fatalf("expected 4 rules after re-emit, got %d", len(out.Rules))
	}
	// Slot 0: user (mysite.ru / my-dns)
	if out.Rules[0].Kind != state.DNSRuleKindUser {
		t.Errorf("rules[0].Kind = %v, want user", out.Rules[0].Kind)
	}
	if got, _ := out.Rules[0].Body["domain"].(string); got != "mysite.ru" {
		t.Errorf("rules[0].Body.domain = %v, want mysite.ru", out.Rules[0].Body["domain"])
	}
	if got, _ := out.Rules[0].Body["server"].(string); got != "my-dns" {
		t.Errorf("rules[0].Body.server = %v, want my-dns", out.Rules[0].Body["server"])
	}
	// Slot 1: preset russian, enabled
	if out.Rules[1].Kind != state.DNSRuleKindPreset {
		t.Errorf("rules[1].Kind = %v, want preset", out.Rules[1].Kind)
	}
	if out.Rules[1].Ref != "russian" {
		t.Errorf("rules[1].Ref = %v, want russian", out.Rules[1].Ref)
	}
	if !out.Rules[1].Enabled {
		t.Errorf("rules[1].Enabled = false, want true")
	}
	// Slot 2: user (ads.example.com / block-dns)
	if out.Rules[2].Kind != state.DNSRuleKindUser {
		t.Errorf("rules[2].Kind = %v, want user", out.Rules[2].Kind)
	}
	if got, _ := out.Rules[2].Body["server"].(string); got != "block-dns" {
		t.Errorf("rules[2].Body.server = %v, want block-dns", out.Rules[2].Body["server"])
	}
	// Slot 3: preset ip-fallback, disabled via DNSRuleEnabled toggle
	if out.Rules[3].Kind != state.DNSRuleKindPreset {
		t.Errorf("rules[3].Kind = %v, want preset", out.Rules[3].Kind)
	}
	if out.Rules[3].Ref != "ip-fallback" {
		t.Errorf("rules[3].Ref = %v, want ip-fallback", out.Rules[3].Ref)
	}
	if out.Rules[3].Enabled {
		t.Errorf("rules[3].Enabled = true, want false (DNSRuleEnabled override)")
	}
}

// TestSyncDNSByOrderToState_EmptyOrderFallsBackToText — defensive: legacy
// state with no DNSRuleOrder still works.
func TestSyncDNSByOrderToState_EmptyOrderFallsBackToText(t *testing.T) {
	text := `{"rules":[{"domain":"example.com","server":"my-dns"}]}`
	out := wizardmodels.SyncDNSByOrderToState(
		nil, // order empty → fallback
		nil, // presetRefs
		nil, // userRules
		nil,
		text,
		nil, nil,
	)
	if len(out.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(out.Rules))
	}
	if out.Rules[0].Kind != state.DNSRuleKindUser {
		t.Errorf("rules[0].Kind = %v, want user", out.Rules[0].Kind)
	}
	if got, _ := out.Rules[0].Body["domain"].(string); got != "example.com" {
		t.Errorf("rules[0].Body.domain = %v", out.Rules[0].Body["domain"])
	}
}

// TestSyncDNSByOrderToState_SkipsDisabledPresetRoute — if preset-ref itself
// is !Enabled (route disabled), the dns_rule slot is silently dropped from
// state.DNS.Rules (mirrors SyncDNSOptionsWithActivePresets invariant).
func TestSyncDNSByOrderToState_SkipsDisabledPresetRoute(t *testing.T) {
	presetRefs := []*wizardmodels.PresetRefState{
		{Ref: "russian", Enabled: false}, // route disabled
	}
	order := []wizardmodels.DNSRuleSlot{
		{Kind: wizardmodels.DNSSlotKindPresetRef, Index: 0},
	}
	out := wizardmodels.SyncDNSByOrderToState(order, presetRefs, nil, nil, "", nil, nil)
	if len(out.Rules) != 0 {
		t.Errorf("expected 0 rules (route disabled), got %d", len(out.Rules))
	}
}

// TestDNSRuleOrderFromStateRules_PreservesSliceOrder — sanity that load builds
// the same order as the state slice.
func TestDNSRuleOrderFromStateRules_PreservesSliceOrder(t *testing.T) {
	rules := []state.DNSRule{
		{Kind: state.DNSRuleKindPreset, Ref: "p2", Enabled: true},
		{Kind: state.DNSRuleKindUser, Enabled: true, Body: map[string]interface{}{"domain": "a"}},
		{Kind: state.DNSRuleKindPreset, Ref: "p1", Enabled: true},
	}
	presetRefs := []*wizardmodels.PresetRefState{
		{Ref: "p1"},
		{Ref: "p2"},
	}
	order, userRules := wizardmodels.DNSRuleOrderFromStateRules(rules, presetRefs)
	if len(order) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(order))
	}
	if order[0].Kind != wizardmodels.DNSSlotKindPresetRef || order[0].Index != 1 {
		t.Errorf("order[0] = %+v, want {PresetRef, 1}", order[0])
	}
	if order[1].Kind != wizardmodels.DNSSlotKindUser || order[1].Index != 0 {
		t.Errorf("order[1] = %+v, want {User, 0}", order[1])
	}
	if order[2].Kind != wizardmodels.DNSSlotKindPresetRef || order[2].Index != 0 {
		t.Errorf("order[2] = %+v, want {PresetRef, 0}", order[2])
	}
	if len(userRules) != 1 {
		t.Fatalf("expected 1 user rule, got %d", len(userRules))
	}
	if got, _ := userRules[0].Body["domain"].(string); got != "a" {
		t.Errorf("userRules[0].Body.domain = %v, want a", userRules[0].Body["domain"])
	}
}
