package configtypes

import (
	"strings"
	"testing"
)

// TestBuildSubscriptionUserAgent_HyphenatedToken guards the fix for the
// "panel serves JSON config instead of subscription list" bug: the product
// token must be the hyphenated "sing-box-launcher". A bare "singbox" substring
// makes substring-matching panels (Remnawave/Marzban-style) treat us as a
// non-sing-box client and return a full client-config JSON the launcher can't
// ingest. The hyphenated "sing-box" token is recognized as a real sing-box
// client → proper subscription list.
func TestBuildSubscriptionUserAgent_HyphenatedToken(t *testing.T) {
	ua := BuildSubscriptionUserAgent()

	if !strings.HasPrefix(ua, "sing-box-launcher/") {
		t.Errorf("UA must start with hyphenated product token, got %q", ua)
	}
	// The whole point: no bare "singbox" anywhere, but "sing-box" present.
	if strings.Contains(ua, "singbox") {
		t.Errorf("UA must not contain bare 'singbox' (panels mis-route it), got %q", ua)
	}
	if !strings.Contains(ua, "sing-box") {
		t.Errorf("UA must contain hyphenated 'sing-box' token, got %q", ua)
	}
}
