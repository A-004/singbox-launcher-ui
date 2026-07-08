package ui

import (
	"fyne.io/fyne/v2"
)

// AdaptiveDashboardLayout переключается между вертикальной (узкий) и
// двухколоночной (широкий) раскладкой для Core Dashboard.
//
// Порог переключения — 901px по ширине.
// Children ожидаются в порядке:
//
//	[0] leftPanel  — статус, трафик, версии, конфиг, subs toast
//	[1] rightPanel — power button, state, exit
//
// ВАЖНО: Layout НЕ оборачивает children в Scroll — это делает вызывающий
// код (один внешний Scroll на весь body).
type AdaptiveDashboardLayout struct {
	threshold float32
}

// NewAdaptiveDashboardLayout создаёт AdaptiveDashboardLayout с порогом 451px.
func NewAdaptiveDashboardLayout() *AdaptiveDashboardLayout {
	return &AdaptiveDashboardLayout{threshold: 451}
}

// MinSize возвращает минимальный размер как сумму высот всех children
// (вертикальная раскладка по умолчанию).
func (l *AdaptiveDashboardLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := float32(0)
	height := float32(0)
	for _, o := range objects {
		min := o.MinSize()
		if min.Width > width {
			width = min.Width
		}
		height += min.Height
	}
	// Добавляем зазоры между children
	if len(objects) > 1 {
		height += float32(len(objects)-1) * 6
	}
	return fyne.NewSize(width, height)
}

// Layout располагает children в зависимости от доступной ширины.
// В узком режиме (< threshold): children друг под другом (как VBox),
// каждый на всю ширину, с собственной высотой по MinSize.
// В широком режиме (>= threshold): две колонки на всю высоту.
func (l *AdaptiveDashboardLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		// Если меньше двух children — просто растягиваем первый на всю площадь
		for _, o := range objects {
			o.Move(fyne.NewPos(0, 0))
			o.Resize(size)
		}
		return
	}

	left, right := objects[0], objects[1]
	gap := float32(6)

	if size.Width < l.threshold {
		// === ВЕРТИКАЛЬНАЯ РАСКЛАДКА (узкий режим) ===
		// Дети друг под другом, каждый на всю ширину, высота по содержимому.
		// Внешний Scroll (добавленный в CreateCoreDashboardTab) обеспечивает
		// скролл если контент не влезает.
		y := float32(0)

		// [0] = power button — сверху
		left.Move(fyne.NewPos(0, y))
		left.Resize(fyne.NewSize(size.Width, left.MinSize().Height))
		y += left.MinSize().Height + gap

		// [1] = информация — снизу
		right.Move(fyne.NewPos(0, y))
		right.Resize(fyne.NewSize(size.Width, right.MinSize().Height))
	} else {
		// === ДВУХКОЛОНОЧНАЯ РАСКЛАДКА (широкий режим) ===
		// [1] = информация — слева, [0] = power button — справа
		colW := (size.Width - gap) / 2

		// Информация слева
		right.Move(fyne.NewPos(0, 0))
		right.Resize(fyne.NewSize(colW, size.Height))

		// Power button справа
		left.Move(fyne.NewPos(colW+gap, 0))
		left.Resize(fyne.NewSize(colW, size.Height))
	}
}
