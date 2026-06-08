package tabs

import (
	"image/color"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	corestate "singbox-launcher/core/state"
	"singbox-launcher/internal/urlsafe"
	"singbox-launcher/ui/icons"
)

// supportLinkForMeta builds the per-source support-link affordance shown under
// a subscription row: an icon + the URL text.
//
//   - URL = meta.SupportURL if present, else meta.ProfileWebPageURL, else none.
//   - Icon = blue Telegram plane for Telegram links (t.me / tg://), otherwise a
//     generic link icon.
//   - Safe schemes (http/https/tg, per urlsafe) render as a clickable hyperlink
//     that opens in the browser/Telegram; an unsafe-but-present URL is shown as
//     plain text (no OpenURL); nothing present → nil (caller omits the line).
func supportLinkForMeta(meta *corestate.SubscriptionMeta) fyne.CanvasObject {
	if meta == nil {
		return nil
	}
	raw := strings.TrimSpace(meta.SupportURL)
	if raw == "" {
		raw = strings.TrimSpace(meta.ProfileWebPageURL)
	}
	if raw == "" {
		return nil
	}

	iconRes := icons.Link
	if isTelegramURL(raw) {
		iconRes = icons.Telegram
	}
	img := canvas.NewImageFromResource(iconRes)
	img.FillMode = canvas.ImageFillContain
	sz := theme.CaptionTextSize() + 3
	img.SetMinSize(fyne.NewSize(sz, sz))

	display := supportLinkDisplayText(raw)

	var textObj fyne.CanvasObject
	if urlsafe.IsSafeAnnounceURL(raw) {
		if u, err := url.Parse(strings.TrimSpace(raw)); err == nil {
			link := widget.NewHyperlink(display, u)
			link.Truncation = fyne.TextTruncateEllipsis
			textObj = link
		}
	}
	if textObj == nil {
		// Present but unsafe scheme (javascript:, file:, …) → plain text only.
		lbl := canvas.NewText(display, theme.PlaceHolderColor())
		lbl.TextSize = theme.CaptionTextSize()
		textObj = lbl
	}

	// Indent to align with the subtitle meta line (48px gutter ≈ checkbox col).
	leftPad := canvas.NewRectangle(color.Transparent)
	leftPad.SetMinSize(fyne.NewSize(44, sz))
	return container.NewBorder(nil, nil, container.NewHBox(leftPad, img), nil, textObj)
}

// isTelegramURL reports whether raw is a Telegram link: scheme tg:// or a
// t.me / telegram.* host.
func isTelegramURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Scheme, "tg") {
		return true
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "t.me", "telegram.me", "telegram.org", "telegram.dog":
		return true
	}
	return strings.HasSuffix(host, ".t.me")
}

// supportLinkDisplayText strips the scheme + trailing slash for a compact label
// (e.g. "https://t.me/foo/" → "t.me/foo").
func supportLinkDisplayText(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	return s
}
