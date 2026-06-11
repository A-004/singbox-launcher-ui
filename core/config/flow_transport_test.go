package config

import (
	"strings"
	"testing"

	"singbox-launcher/core/config/subscription"
)

// VLESS flow (xtls-rprx-vision) is valid only over bare TLS/Reality. When a
// v2ray transport is present sing-box rejects the combo, so the generator must
// drop flow. Conversely, flow over bare Reality (no transport) must survive.
func TestGenerateNodeJSON_FlowDroppedWithTransport(t *testing.T) {
	cases := []struct {
		name     string
		uri      string
		wantFlow bool
	}{
		{
			name:     "flow + xhttp transport → flow dropped",
			uri:      "vless://a0ee37a5-1844-4087-bc5c-1db6f416d38c@h.test:443?encryption=none&flow=xtls-rprx-vision&type=xhttp&path=%2Fx&host=h.test&security=reality&sni=h.test&pbk=BBBB&sid=64f4#t",
			wantFlow: false,
		},
		{
			name:     "flow + ws transport → flow dropped",
			uri:      "vless://a0ee37a5-1844-4087-bc5c-1db6f416d38c@h.test:443?encryption=none&flow=xtls-rprx-vision&type=ws&path=%2Fw&security=tls&sni=h.test#t",
			wantFlow: false,
		},
		{
			name:     "flow + bare Reality (no transport) → flow kept",
			uri:      "vless://a0ee37a5-1844-4087-bc5c-1db6f416d38c@h.test:443?encryption=none&flow=xtls-rprx-vision&type=tcp&security=reality&sni=h.test&pbk=BBBB&sid=64f4#t",
			wantFlow: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node, err := subscription.ParseNode(c.uri, nil)
			if err != nil || node == nil {
				t.Fatalf("parse: %v", err)
			}
			js, err := GenerateNodeJSON(node)
			if err != nil {
				t.Fatalf("gen: %v", err)
			}
			hasFlow := strings.Contains(js, `"flow"`)
			if hasFlow != c.wantFlow {
				t.Errorf("flow present = %v, want %v\nJSON: %s", hasFlow, c.wantFlow, js)
			}
		})
	}
}

// Guard: outboundHasTransport recognizes a real transport but not an empty/absent one.
func TestOutboundHasTransport(t *testing.T) {
	if outboundHasTransport(nil) {
		t.Error("nil outbound has no transport")
	}
	if outboundHasTransport(map[string]interface{}{"type": "vless"}) {
		t.Error("no transport key → false")
	}
	if outboundHasTransport(map[string]interface{}{"transport": map[string]interface{}{}}) {
		t.Error("empty transport map → false")
	}
	if !outboundHasTransport(map[string]interface{}{"transport": map[string]interface{}{"type": "xhttp"}}) {
		t.Error("xhttp transport → true")
	}
}
