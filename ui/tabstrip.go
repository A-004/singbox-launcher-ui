package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// customTabstrip creates a minimal tab bar with square-outlined active tab
// and thin separators between tabs, replacing Fyne's default AppTabs
// (thick underline indicator). Returns (tabBar, contentStack, selectFunc).
//
// Usage:
//
//	bar, stack, select := customTabstrip(items, 0)
//	content := container.NewBorder(bar, nil, nil, nil, stack)
//	select(initial) // show initial tab
func customTabstrip(labels []string, contents []fyne.CanvasObject, onChanged func(int)) (fyne.CanvasObject, *fyne.Container, func(int)) {
	if len(labels) == 0 {
		return widget.NewLabel(""), container.NewStack(), func(int) {}
	}

	activeIdx := 0
	btns := make([]*widget.Button, len(labels))
	contentStack := container.NewStack(contents[0])

	for i := range labels {
		idx := i
		btn := widget.NewButton(labels[i], func() {
			if idx == activeIdx {
				return
			}
			activeIdx = idx
			contentStack.Objects = []fyne.CanvasObject{contents[idx]}
			contentStack.Refresh()
			updateStyles(btns, idx)
			if onChanged != nil {
				onChanged(idx)
			}
		})
		btn.Importance = widget.LowImportance
		btns[i] = btn
	}

	updateStyles(btns, 0)

	// Build bar with thin separators
	items := make([]fyne.CanvasObject, 0, len(btns)*2-1)
	for i, b := range btns {
		if i > 0 {
			sep := canvas.NewRectangle(color.NRGBA{R: 0x38, G: 0x38, B: 0x3a, A: 0xff})
			sep.SetMinSize(fyne.NewSize(1, 20))
			items = append(items, sep)
		}
		items = append(items, b)
	}

	bar := container.NewCenter(container.NewHBox(items...))

	selectFn := func(idx int) {
		if idx < 0 || idx >= len(contents) || idx == activeIdx {
			return
		}
		activeIdx = idx
		contentStack.Objects = []fyne.CanvasObject{contents[idx]}
		contentStack.Refresh()
		updateStyles(btns, idx)
	}

	return bar, contentStack, selectFn
}

// updateStyles applies active/inactive styling to tab buttons.
func updateStyles(btns []*widget.Button, active int) {
	for i, btn := range btns {
		if i == active {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
}
