// File rule_identity_test.go — SPEC 063 unit tests for StableRuleID + sanitizeIDPart.
package state

import (
	"encoding/json"
	"testing"
)

func inlineBodyJSON(name string) json.RawMessage {
	b, _ := json.Marshal(InlineBody{Name: name, Match: map[string]interface{}{"port": []int{443}}, Outbound: "direct-out"})
	return b
}

func srsBodyJSON(name, url string) json.RawMessage {
	b, _ := json.Marshal(SrsBody{Name: name, SrsURL: url, Outbound: "reject"})
	return b
}

// TestStableRuleID_AllKinds — для каждого kind возвращает корректную identity.
func TestStableRuleID_AllKinds(t *testing.T) {
	cases := []struct {
		name string
		rule Rule
		want string
	}{
		{
			name: "preset returns Ref",
			rule: Rule{Kind: RuleKindPreset, Ref: "ru-direct", Body: json.RawMessage(`{"vars":{}}`)},
			want: "ru-direct",
		},
		{
			name: "inline returns sanitized name",
			rule: Rule{Kind: RuleKindInline, Body: inlineBodyJSON("Firefox VPN")},
			want: "Firefox-VPN",
		},
		{
			name: "srs returns sanitized name",
			rule: Rule{Kind: RuleKindSrs, Body: srsBodyJSON("Custom block list", "https://x/y.srs")},
			want: "Custom-block-list",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StableRuleID(tc.rule); got != tc.want {
				t.Errorf("StableRuleID: got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestStableRuleID_EdgeCases — empty/unknown/undecodable → "unnamed".
func TestStableRuleID_EdgeCases(t *testing.T) {
	cases := []struct {
		name string
		rule Rule
		want string
	}{
		{
			name: "inline empty name",
			rule: Rule{Kind: RuleKindInline, Body: inlineBodyJSON("")},
			want: "unnamed",
		},
		{
			name: "srs empty name",
			rule: Rule{Kind: RuleKindSrs, Body: srsBodyJSON("", "https://x")},
			want: "unnamed",
		},
		{
			name: "unknown kind",
			rule: Rule{Kind: "geosite", Body: json.RawMessage(`{}`)},
			want: "unnamed",
		},
		{
			name: "undecodable body",
			rule: Rule{Kind: RuleKindInline, Body: json.RawMessage(`{ not json`)},
			want: "unnamed",
		},
		{
			name: "unicode/special chars sanitized to ASCII",
			rule: Rule{Kind: RuleKindInline, Body: inlineBodyJSON("Firefox через VPN!")},
			want: "Firefox--VPN", // не-ASCII strip, "!" strip, пробелы → "-"
		},
		{
			name: "all special chars → fallback rule",
			rule: Rule{Kind: RuleKindInline, Body: inlineBodyJSON("!@#")},
			want: "rule",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StableRuleID(tc.rule); got != tc.want {
				t.Errorf("StableRuleID: got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSanitizeIDPart — sanity для нижнего helper.
func TestSanitizeIDPart(t *testing.T) {
	cases := map[string]string{
		"Hello World":            "Hello-World",
		"Firefox через VPN":      "Firefox--VPN",
		"":                       "rule",
		"name with !@# symbols!": "name-with--symbols",
		"abc_DEF-123":            "abc_DEF-123",
		"!@#$%^":                 "rule",
	}
	for in, want := range cases {
		if got := sanitizeIDPart(in); got != want {
			t.Errorf("sanitizeIDPart(%q): got %q, want %q", in, got, want)
		}
	}
}
