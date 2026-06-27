package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core"
	"singbox-launcher/core/events"
	"singbox-launcher/internal/locale"
	"singbox-launcher/ui/components"
)

// tabDef describes one tab in the custom tab bar.
type tabDef struct {
	Label   string
	Content fyne.CanvasObject
}

// App manages the UI structure and tabs.
//
// Uses custom tab buttons with square outline on the active tab instead
// of Fyne's default AppTabs (which have a thick underline indicator).
type App struct {
	window  fyne.Window
	core    *core.AppController
	content fyne.CanvasObject
	overlay *components.ClickRedirect

	tabs          []tabDef
	activeTab     int
	tabBtns       []*widget.Button
	tabContentBox *fyne.Container

	// Clash API tab index (for updateClashAPITabState)
	clashAPIIndex int
}

// NewApp creates a new App instance
func NewApp(window fyne.Window, controller *core.AppController) *App {
	app := &App{
		window: window,
		core:   controller,
	}

	// Build tab definitions
	coreLabel := stripLeadingEmoji(locale.T("app.tab.core"))
	serversLabel := locale.T("app.tab.servers")
	diagLabel := locale.T("app.tab.diagnostics")
	settingsLabel := locale.T("app.tab.settings")
	helpLabel := locale.T("app.tab.help")

	app.tabs = []tabDef{
		{Label: coreLabel, Content: CreateCoreDashboardTab(controller)},
		{Label: serversLabel, Content: CreateClashAPITab(controller)},
		{Label: diagLabel, Content: CreateDiagnosticsTab(controller)},
		{Label: settingsLabel, Content: widget.NewLabel("")},
		{Label: helpLabel, Content: CreateHelpTab(controller)},
	}
	app.clashAPIIndex = 1

	// Settings opens a separate window via callback, not inline content.
	// Store a no-op content; the on-tap handler does the window open.

	// Build tab buttons
	app.tabBtns = make([]*widget.Button, len(app.tabs))
	for i, td := range app.tabs {
		idx := i
		btn := widget.NewButton(td.Label, func() {
			if idx == 3 { // Settings tab index
				OpenSettingsWindow(controller)
				return
			}
			app.selectTab(idx)
			if idx == app.clashAPIIndex {
				if controller.UIService != nil && controller.UIService.RefreshAPIFunc != nil {
					controller.UIService.RefreshAPIFunc()
				}
			}
		})
		btn.Importance = widget.LowImportance
		app.tabBtns[i] = btn
	}

	// Tab content: start with the first tab's content visible
	app.activeTab = 0
	app.tabContentBox = container.NewStack(app.tabs[0].Content)

	// Apply active styling
	app.updateTabStyles()

	// Build bottom navigation bar (tab buttons)
	tabBarItems := make([]fyne.CanvasObject, 0, len(app.tabBtns))
	for _, btn := range app.tabBtns {
		tabBarItems = append(tabBarItems, btn)
	}

	// Bottom navigation — Apple dark style (49px height)
	bottomBg := canvas.NewRectangle(AppleBottomBg)
	bottomBg.SetMinSize(fyne.NewSize(0, 49))

	bottomSep := canvas.NewRectangle(AppleSeparator)
	bottomSep.SetMinSize(fyne.NewSize(0, 0.5))

	bottomBar := container.NewVBox(
		bottomSep,
		container.NewCenter(container.NewHBox(tabBarItems...)),
	)
	bottomWrap := container.NewStack(bottomBg, bottomBar)

	// Apple-style titlebar at top
	titlebar := AppleTitlebar("xerotrace (singbox-launcher)", 40)

	// Main layout: titlebar top, bottom nav, content in center
	mainContent := container.NewBorder(
		titlebar,  // top
		bottomWrap, // bottom
		nil, nil,
		app.tabContentBox, // center (fills remaining space)
	)

	// Set window background
	bg := canvas.NewRectangle(AppleWindowBg)

	app.content = container.NewStack(bg, mainContent)

	// Core tab status refresh
	refreshCoreTabIcon := func() {
		var icon string
		if controller.RunningState != nil && controller.RunningState.IsRunning() {
			icon = ">"
		} else {
			icon = "||"
		}
		app.tabBtns[0].SetText(icon + " " + coreLabel)
	}
	refreshCoreTabIcon()

	// Wire up event callbacks
	originalUpdateCoreStatusFunc := controller.UIService.UpdateCoreStatusFunc
	controller.UIService.UpdateCoreStatusFunc = func() {
		if originalUpdateCoreStatusFunc != nil {
			originalUpdateCoreStatusFunc()
		}
		fyne.Do(func() {
			app.updateClashAPITabState()
		})
	}

	if controller.EventBus != nil {
		controller.EventBus.Subscribe(events.VpnStateChanged, func(_ events.Event) {
			fyne.Do(refreshCoreTabIcon)
		})
	}

	OnOverrideChanged(func() {
		fyne.Do(app.updateClashAPITabState)
	})

	app.updateClashAPITabState()
	refreshCoreTabIcon()

	InitWizardOverlay(app, controller)

	app.registerShortcuts()

	return app
}

// selectTab switches to the given tab index and updates button styles.
func (a *App) selectTab(idx int) {
	if idx < 0 || idx >= len(a.tabs) || idx == a.activeTab {
		return
	}
	// Replace content in stack
	a.tabContentBox.Objects = []fyne.CanvasObject{a.tabs[idx].Content}
	a.tabContentBox.Refresh()
	a.activeTab = idx
	a.updateTabStyles()
}

// updateTabStyles applies square outline to the active tab button.
func (a *App) updateTabStyles() {
	for i, btn := range a.tabBtns {
		if i == a.activeTab {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
}

func (a *App) registerShortcuts() {
	if a.window == nil || a.window.Canvas() == nil {
		return
	}
	reconnect := &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierShortcutDefault}
	a.window.Canvas().AddShortcut(reconnect, func(fyne.Shortcut) {
		core.KillSingBoxForRestart()
	})
	updateSubs := &desktop.CustomShortcut{KeyName: fyne.KeyU, Modifier: fyne.KeyModifierShortcutDefault}
	a.window.Canvas().AddShortcut(updateSubs, func(fyne.Shortcut) {
		core.RunParserProcess()
	})
	pingAll := &desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: fyne.KeyModifierShortcutDefault}
	a.window.Canvas().AddShortcut(pingAll, func(fyne.Shortcut) {
		if a.core != nil && a.core.UIService != nil && a.core.UIService.AutoPingAfterConnectFunc != nil {
			a.core.UIService.AutoPingAfterConnectFunc()
		}
	})
}

// GetContent returns the root content for the main window.
func (a *App) GetContent() fyne.CanvasObject {
	return a.content
}

// updateClashAPITabState — SPEC 064: Servers tab always enabled.
func (a *App) updateClashAPITabState() {
	// All tabs are always enabled in our custom bar — no-op.
}

// stripLeadingEmoji removes any leading emoji + space from a label.
func stripLeadingEmoji(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[i+1:]
		}
	}
	return s
}
