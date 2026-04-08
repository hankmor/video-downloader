package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ThinProgressBar 自定义细进度条
type ThinProgressBar struct {
	widget.BaseWidget
	Value float64
}

// NewThinProgressBar 创建新的细进度条
func NewThinProgressBar() *ThinProgressBar {
	p := &ThinProgressBar{}
	p.ExtendBaseWidget(p)
	return p
}

// SetValue 设置进度 (0.0 - 1.0)
func (p *ThinProgressBar) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	if p.Value == v {
		return
	}
	p.Value = v
	p.Refresh()
}

// CreateRenderer 实现 Widget 接口
func (p *ThinProgressBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameDisabled))
	bar := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))

	// 圆角效果
	bg.CornerRadius = 3
	bar.CornerRadius = 3

	return &thinProgressRenderer{
		p:       p,
		bg:      bg,
		bar:     bar,
		objects: []fyne.CanvasObject{bg, bar},
	}
}

type thinProgressRenderer struct {
	p       *ThinProgressBar
	bg      *canvas.Rectangle
	bar     *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *thinProgressRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	barWidth := float32(r.p.Value) * size.Width
	r.bar.Resize(fyne.NewSize(barWidth, size.Height))
	r.bar.Move(fyne.NewPos(0, 0))
}

func (r *thinProgressRenderer) MinSize() fyne.Size {
	// 设置高度
	return fyne.NewSize(10, 6)
}

func (r *thinProgressRenderer) Refresh() {
	r.bg.FillColor = theme.Color(theme.ColorNameDisabled)
	r.bg.Refresh()

	r.bar.FillColor = theme.Color(theme.ColorNamePrimary)
	r.bar.Refresh()

	// 触发重新布局以更新宽度
	r.Layout(r.p.Size())
}

func (r *thinProgressRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *thinProgressRenderer) Destroy() {}
