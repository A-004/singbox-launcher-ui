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

// CyberAppleTheme gives the launcher a restrained black and white surface
// with a small violet accent and squared-off controls.
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
		return color.NRGBA{R: 0x08, G: 0x09, B: 0x0d, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xf6, G: 0xf7, B: 0xfb, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x16, G: 0x17, B: 0x1d, A: 0xe8}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x11, G: 0x12, B: 0x16, A: 0xb8}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x74, G: 0x77, B: 0x82, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x9a, G: 0x9d, B: 0xaa, A: 0xff}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return color.NRGBA{R: 0x9b, G: 0x6d, B: 0xff, A: 0xff}
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0x9b, G: 0x6d, B: 0xff, A: 0x66}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0f}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0x9b, G: 0x6d, B: 0xff, A: 0x2e}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x9b, G: 0x6d, B: 0xff, A: 0x3a}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x0f, G: 0x10, B: 0x16, A: 0xf2}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x14}
	case theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return color.NRGBA{R: 0x0d, G: 0x0e, B: 0x14, A: 0xf8}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x13, G: 0x14, B: 0x1c, A: 0xf4}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x38}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0a}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x88}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x32, G: 0xd5, B: 0x83, A: 0xff}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xff, G: 0xcc, B: 0x66, A: 0xff}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xff, G: 0x5c, B: 0x7a, A: 0xff}
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return color.NRGBA{R: 0x05, G: 0x06, B: 0x09, A: 0xff}
	default:
		return theme.DarkTheme().Color(name, variant)
	}
}

func (t *CyberAppleTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DarkTheme().Font(style)
}

func (t *CyberAppleTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(name)
}

func (t *CyberAppleTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius, theme.SizeNameScrollBarRadius:
		return 2
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameSeparatorThickness:
		return 0.5
	case theme.SizeNameText:
		return 14
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 17
	case theme.SizeNameInlineIcon:
		return 19
	default:
		return theme.DarkTheme().Size(name)
	}
}
