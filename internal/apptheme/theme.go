package apptheme

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// UserTheme represents a subset of theme colors that can be overridden via JSON.
type UserTheme struct {
	Background          *string `json:"Background,omitempty"`
	Foreground          *string `json:"Foreground,omitempty"`
	Primary             *string `json:"Primary,omitempty"`
	Button              *string `json:"Button,omitempty"`
	DisabledButton      *string `json:"DisabledButton,omitempty"`
	Disabled            *string `json:"Disabled,omitempty"`
	PlaceHolder         *string `json:"PlaceHolder,omitempty"`
	Focus               *string `json:"Focus,omitempty"`
	Hover               *string `json:"Hover,omitempty"`
	Pressed             *string `json:"Pressed,omitempty"`
	Selection           *string `json:"Selection,omitempty"`
	InputBackground     *string `json:"InputBackground,omitempty"`
	InputBorder         *string `json:"InputBorder,omitempty"`
	Separator           *string `json:"Separator,omitempty"`
	HeaderBackground    *string `json:"HeaderBackground,omitempty"`
	MenuBackground      *string `json:"MenuBackground,omitempty"`
	OverlayBackground   *string `json:"OverlayBackground,omitempty"`
	ScrollBar           *string `json:"ScrollBar,omitempty"`
	ScrollBarBackground *string `json:"ScrollBarBackground,omitempty"`
	Shadow              *string `json:"Shadow,omitempty"`
	Success             *string `json:"Success,omitempty"`
	Warning             *string `json:"Warning,omitempty"`
	Error               *string `json:"Error,omitempty"`
}

// CyberAppleTheme implements Apple Human Interface Guidelines:
// - Strict black-and-white palette (iOS System colors)
// - Generous whitespace (padding 12–20)
// - Squircle corners (8–12px radius)
// - SF System font (via Fyne default)
// - Minimal hierarchy: pure contrast, no colored accents
type CyberAppleTheme struct {
	userTheme UserTheme
}

var _ fyne.Theme = (*CyberAppleTheme)(nil)

func NewCyberAppleTheme() fyne.Theme {
	t := &CyberAppleTheme{}
	t.loadUserTheme()
	return t
}

// loadUserTheme tries to read user_theme.json relative to the executable.
func (t *CyberAppleTheme) loadUserTheme() {
	ex, err := os.Executable()
	if err != nil {
		return
	}
	themePath := filepath.Join(filepath.Dir(ex), "user_theme.json")
	data, err := os.ReadFile(themePath)
	if err != nil {
		return // file doesn't exist — use defaults
	}
	var ut UserTheme
	if err := json.Unmarshal(data, &ut); err != nil {
		return // malformed — use defaults
	}
	t.userTheme = ut
}

// hexToColor converts "#RRGGBB" or "#RRGGBBAA" to color.NRGBA.
func hexToColor(hex string) color.NRGBA {
	if len(hex) == 0 || hex[0] != '#' {
		return color.NRGBA{}
	}
	hex = hex[1:] // strip '#'
	var r, g, b, a uint8
	a = 0xff
	switch len(hex) {
	case 6:
		_, _ = scanHex(hex, &r, &g, &b)
	case 8:
		_, _ = scanHex(hex, &r, &g, &b, &a)
	}
	return color.NRGBA{R: r, G: g, B: b, A: a}
}

func scanHex(s string, vals ...*uint8) (int, error) {
	for i, v := range vals {
		if 2*i+2 > len(s) {
			break
		}
		n := 0
		for j := 0; j < 2; j++ {
			n <<= 4
			c := s[2*i+j]
			switch {
			case c >= '0' && c <= '9':
				n += int(c - '0')
			case c >= 'a' && c <= 'f':
				n += int(c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				n += int(c - 'A' + 10)
			}
		}
		*v = uint8(n)
	}
	return len(vals), nil
}

func (t *CyberAppleTheme) userColor(name fyne.ThemeColorName) *color.NRGBA {
	hex := func(s *string) *color.NRGBA {
		if s == nil || *s == "" {
			return nil
		}
		c := hexToColor(*s)
		return &c
	}

	switch name {
	case theme.ColorNameBackground:
		return hex(t.userTheme.Background)
	case theme.ColorNameForeground:
		return hex(t.userTheme.Foreground)
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return hex(t.userTheme.Primary)
	case theme.ColorNameButton:
		return hex(t.userTheme.Button)
	case theme.ColorNameDisabledButton:
		return hex(t.userTheme.DisabledButton)
	case theme.ColorNameDisabled:
		return hex(t.userTheme.Disabled)
	case theme.ColorNamePlaceHolder:
		return hex(t.userTheme.PlaceHolder)
	case theme.ColorNameFocus:
		return hex(t.userTheme.Focus)
	case theme.ColorNameHover:
		return hex(t.userTheme.Hover)
	case theme.ColorNamePressed:
		return hex(t.userTheme.Pressed)
	case theme.ColorNameSelection:
		return hex(t.userTheme.Selection)
	case theme.ColorNameInputBackground:
		return hex(t.userTheme.InputBackground)
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		if c := hex(t.userTheme.InputBorder); c != nil {
			return c
		}
		return hex(t.userTheme.Separator)
	case theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		if c := hex(t.userTheme.HeaderBackground); c != nil {
			return c
		}
		return hex(t.userTheme.MenuBackground)
	case theme.ColorNameOverlayBackground:
		return hex(t.userTheme.OverlayBackground)
	case theme.ColorNameScrollBar:
		return hex(t.userTheme.ScrollBar)
	case theme.ColorNameScrollBarBackground:
		return hex(t.userTheme.ScrollBarBackground)
	case theme.ColorNameShadow:
		return hex(t.userTheme.Shadow)
	case theme.ColorNameSuccess:
		return hex(t.userTheme.Success)
	case theme.ColorNameWarning:
		return hex(t.userTheme.Warning)
	case theme.ColorNameError:
		return hex(t.userTheme.Error)
	}
	return nil
}

func (t *CyberAppleTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// User theme JSON has priority over hardcoded defaults.
	if c := t.userColor(name); c != nil {
		return *c
	}

	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x1c, G: 0x1c, B: 0x1e, A: 0xff}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x2c, G: 0x2c, B: 0x2e, A: 0xff}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x63, G: 0x63, B: 0x66, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x8e, G: 0x8e, B: 0x93, A: 0xff}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x40}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x10}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x1e}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x2e}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x1c, G: 0x1c, B: 0x1e, A: 0xff}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{R: 0x38, G: 0x38, B: 0x3a, A: 0xff}
	case theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return color.NRGBA{R: 0x11, G: 0x11, B: 0x13, A: 0xff}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x11, G: 0x11, B: 0x13, A: 0xf5}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x3c}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0a}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xcc}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	default:
		return theme.DarkTheme().Color(name, variant)
	}
}

func (t *CyberAppleTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTextFont()
}

func (t *CyberAppleTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(name)
}

func (t *CyberAppleTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 10
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 8
	case theme.SizeNameScrollBarRadius:
		return 4
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameSeparatorThickness:
		return 0.2
	case theme.SizeNameText:
		return 13
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameInlineIcon:
		return 18
	default:
		return theme.DarkTheme().Size(name)
	}
}
