package widgets

import "fyne.io/fyne/v2"

// compactVBoxLayout 自定义紧凑垂直布局
type compactVBoxLayout struct {
	spacing float32
}

func NewCompactVBoxLayout(spacing float32) fyne.Layout {
	return &compactVBoxLayout{spacing: spacing}
}

func (l *compactVBoxLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, child := range objects {
		if !child.Visible() {
			continue
		}

		childHeight := child.MinSize().Height
		child.Resize(fyne.NewSize(size.Width, childHeight))
		child.Move(fyne.NewPos(0, y))

		y += childHeight + l.spacing
	}
}

func (l *compactVBoxLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	visibleCount := 0
	for _, child := range objects {
		if !child.Visible() {
			continue
		}
		childMin := child.MinSize()
		w = fyne.Max(w, childMin.Width)
		h += childMin.Height
		visibleCount++
	}

	if visibleCount > 1 {
		h += float32(visibleCount-1) * l.spacing
	}

	return fyne.NewSize(w, h)
}
