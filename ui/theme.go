// Package ui — Apple-inspired design system for singbox-launcher.
//
// Provides color constants, typography helpers, and component builders
// that follow the Human Interface Guidelines (macOS/iOS design language).
package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ──────────────────────────────────────────────
// Apple Dark Color Palette (Dark Mode)
// ──────────────────────────────────────────────

var (
	AppleWindowBg      = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF} // #000000
	AppleCardBg        = color.NRGBA{R: 0x1C, G: 0x1C, B: 0x1E, A: 0xFF} // #1C1C1E
	AppleCardBorder    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x14} // rgba(255,255,255,0.08)
	AppleTextPrimary   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // #FFFFFF
	AppleTextSecondary = color.NRGBA{R: 0x8E, G: 0x8E, B: 0x93, A: 0xFF} // #8E8E93
	AppleTextTertiary  = color.NRGBA{R: 0x6E, G: 0x6E, B: 0x73, A: 0xFF} // #6E6E73
	AppleBlue          = color.NRGBA{R: 0x00, G: 0x7A, B: 0xFF, A: 0xFF} // #007AFF
	AppleGreen         = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0xFF} // #34C759
	AppleOrange        = color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF} // #FF9F0A
	AppleRed           = color.NRGBA{R: 0xFF, G: 0x3B, B: 0x30, A: 0xFF} // #FF3B30
	AppleGray          = color.NRGBA{R: 0x8E, G: 0x8E, B: 0x93, A: 0xFF} // #8E8E93
	AppleClose         = color.NRGBA{R: 0xFF, G: 0x5F, B: 0x57, A: 0xFF} // #FF5F57
	AppleMinimize      = color.NRGBA{R: 0xFE, G: 0xBC, B: 0x2E, A: 0xFF} // #FEBC2E
	AppleMaximize      = color.NRGBA{R: 0x28, G: 0xC8, B: 0x40, A: 0xFF} // #28C840
	AppleChevron       = color.NRGBA{R: 0x8E, G: 0x8E, B: 0x93, A: 0xFF} // #8E8E93 (dark)
	AppleSeparator     = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0F} // rgba(255,255,255,0.06)
	ApplePowerOff      = color.NRGBA{R: 0x2C, G: 0x2C, B: 0x2E, A: 0xFF} // #2C2C2E
	ApplePowerOn       = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0xFF} // #34C759
	ApplePowerAura     = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0x4D} // rgba(52,199,89,0.3)
	AppleActiveGreen   = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0x14} // rgba(52,199,89,0.08)
	AppleActiveText    = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0xFF} // #34C759

	// Titlebar / bottom nav — dark blur
	AppleTitleBg  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xE0} // rgba(0,0,0,0.88)
	AppleBottomBg = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xE6} // rgba(0,0,0,0.90)
)

// ──────────────────────────────────────────────
// Card / Container Helpers
// ──────────────────────────────────────────────

// NewAppleCard creates an Apple-style card with white background,
// 0.5px border, and 16px border-radius. No shadow.
func NewAppleCard(content fyne.CanvasObject) *fyne.Container {
	bg := canvas.NewRectangle(AppleCardBg)
	bg.CornerRadius = 16

	border := canvas.NewRectangle(color.Transparent)
	border.CornerRadius = 16
	border.StrokeWidth = 0.5
	border.StrokeColor = AppleCardBorder

	inner := container.NewPadded(content)
	return container.NewStack(bg, border, inner)
}

// NewAppleCardSmall creates a smaller card with 10px radius.
func NewAppleCardSmall(content fyne.CanvasObject) *fyne.Container {
	bg := canvas.NewRectangle(AppleCardBg)
	bg.CornerRadius = 10

	border := canvas.NewRectangle(color.Transparent)
	border.CornerRadius = 10
	border.StrokeWidth = 0.5
	border.StrokeColor = AppleCardBorder

	inner := container.NewPadded(content)
	return container.NewStack(bg, border, inner)
}

// NewAppleCapsLabel creates an uppercase label (11px / weight 500).
func NewAppleCapsLabel(text string) *canvas.Text {
	t := canvas.NewText(text, AppleTextTertiary)
	t.TextSize = 11
	return t
}

// NewAppleCaptionLabel creates a caption label (13px / weight 400), secondary color.
func NewAppleCaptionLabel(text string) *canvas.Text {
	t := canvas.NewText(text, AppleTextSecondary)
	t.TextSize = 13
	return t
}

// NewAppleSeparator creates a 0.5px separator line.
func NewAppleSeparator() fyne.CanvasObject {
	sep := canvas.NewRectangle(AppleSeparator)
	sep.SetMinSize(fyne.NewSize(0, 1))
	return sep
}

// AppleTitlebar creates a titlebar with centered title and thin bottom border.
func AppleTitlebar(title string, height float32) fyne.CanvasObject {
	bg := canvas.NewRectangle(AppleTitleBg)

	titleTxt := canvas.NewText(title, AppleTextPrimary)
	titleTxt.TextSize = 13

	bar := container.NewBorder(nil, nil, nil, nil,
		container.NewCenter(titleTxt),
	)

	// Thin bottom border
	bottomLine := canvas.NewRectangle(AppleSeparator)

	return container.NewStack(
		bg,
		container.NewBorder(nil, bottomLine, nil, nil, bar),
	)
}

// ApplePowerButton is a circular 88×88 power button with ON/OFF states.
// ON: green fill with aura border ring. OFF: gray fill.
// Implements fyne.Tappable + desktop.Hoverable for click handling.
type ApplePowerButton struct {
	widget.BaseWidget
	on       bool
	hovered  bool
	onTapped func()
}

var _ fyne.Tappable = (*ApplePowerButton)(nil)
var _ desktop.Hoverable = (*ApplePowerButton)(nil)

// Tapped implements fyne.Tappable.
func (b *ApplePowerButton) Tapped(*fyne.PointEvent) {
	if b.onTapped != nil {
		b.onTapped()
	}
}

// MouseIn implements desktop.Hoverable.
func (b *ApplePowerButton) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}

// MouseOut implements desktop.Hoverable.
func (b *ApplePowerButton) MouseOut() {
	b.hovered = false
	b.Refresh()
}

// MouseMoved implements desktop.Hoverable.
func (b *ApplePowerButton) MouseMoved(*desktop.MouseEvent) {}

// Cursor implements desktop.Cursorable.
func (b *ApplePowerButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

// NewApplePowerButton creates a new Apple-style power button.
func NewApplePowerButton(on bool, onTapped func()) *ApplePowerButton {
	btn := &ApplePowerButton{on: on, onTapped: onTapped}
	btn.ExtendBaseWidget(btn)
	return btn
}

// SetOn updates the button on/off state.
func (b *ApplePowerButton) SetOn(on bool) {
	if b.on != on {
		b.on = on
		b.Refresh()
	}
}

// Toggle switches the power state.
func (b *ApplePowerButton) Toggle() {
	b.on = !b.on
	b.Refresh()
}

// CreateRenderer implements fyne.Widget.
func (b *ApplePowerButton) CreateRenderer() fyne.WidgetRenderer {
	r := &applePowerButtonRenderer{btn: b}

	r.auraRing = canvas.NewCircle(color.Transparent)
	r.auraRing.Hidden = true

	r.bgCircle = canvas.NewCircle(ApplePowerOff)

	r.hoverCircle = canvas.NewCircle(color.Transparent)
	r.hoverCircle.Hidden = true

	r.shieldTxt = canvas.NewText("⏻", color.White)
	r.shieldTxt.TextSize = 32

	r.objects = []fyne.CanvasObject{r.auraRing, r.bgCircle, r.hoverCircle, r.shieldTxt}
	return r
}

// MinSize returns the fixed 88×88 size.
func (b *ApplePowerButton) MinSize() fyne.Size {
	return fyne.NewSize(88, 88)
}

type applePowerButtonRenderer struct {
	btn         *ApplePowerButton
	objects     []fyne.CanvasObject
	bgCircle    *canvas.Circle
	auraRing    *canvas.Circle
	hoverCircle *canvas.Circle
	shieldTxt   *canvas.Text
}

func (r *applePowerButtonRenderer) Layout(size fyne.Size) {
	cx, cy := size.Width/2, size.Height/2
	rad := float32(42)
	r.bgCircle.Position1 = fyne.NewPos(cx-rad, cy-rad)
	r.bgCircle.Position2 = fyne.NewPos(cx+rad, cy+rad)

	auraR := rad + 4
	r.auraRing.Position1 = fyne.NewPos(cx-auraR, cy-auraR)
	r.auraRing.Position2 = fyne.NewPos(cx+auraR, cy+auraR)

	r.hoverCircle.Position1 = fyne.NewPos(cx-rad, cy-rad)
	r.hoverCircle.Position2 = fyne.NewPos(cx+rad, cy+rad)

	if r.shieldTxt != nil {
		ts := r.shieldTxt.MinSize()
		r.shieldTxt.Move(fyne.NewPos(cx-ts.Width/2, cy-ts.Height/2-2))
	}
}

func (r *applePowerButtonRenderer) MinSize() fyne.Size { return fyne.NewSize(88, 88) }

func (r *applePowerButtonRenderer) Refresh() {
	if r.bgCircle == nil {
		return
	}
	if r.btn.on {
		r.bgCircle.FillColor = ApplePowerOn
		r.auraRing.FillColor = color.Transparent
		r.auraRing.StrokeColor = ApplePowerAura
		r.auraRing.StrokeWidth = 2
		r.auraRing.Hidden = false
	} else {
		r.bgCircle.FillColor = ApplePowerOff
		r.auraRing.Hidden = true
	}
	if r.btn.hovered {
		// Subtle dim overlay when hovering
		r.hoverCircle.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x12}
		r.hoverCircle.Hidden = false
	} else {
		r.hoverCircle.Hidden = true
	}
	canvas.Refresh(r.bgCircle)
	canvas.Refresh(r.auraRing)
	canvas.Refresh(r.hoverCircle)
}

func (r *applePowerButtonRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *applePowerButtonRenderer) Destroy()                     {}

// NewStatCard creates a small stat display card (iOS Health style).
func NewStatCard(label, value, unit string) fyne.CanvasObject {
	labelT := canvas.NewText(label, AppleTextSecondary)
	labelT.TextSize = 11

	valueT := canvas.NewText(value, AppleTextPrimary)
	valueT.TextSize = 20

	unitT := canvas.NewText(unit, AppleTextSecondary)
	unitT.TextSize = 12

	valRow := container.NewHBox(valueT, unitT)
	content := container.NewVBox(labelT, valRow)
	padded := container.NewPadded(content)

	bg := canvas.NewRectangle(AppleCardBg)
	bg.CornerRadius = 10
	border := canvas.NewRectangle(color.Transparent)
	border.CornerRadius = 10
	border.StrokeWidth = 0.5
	border.StrokeColor = AppleCardBorder

	return container.NewStack(bg, border, padded)
}
