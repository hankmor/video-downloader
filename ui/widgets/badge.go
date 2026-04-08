package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// Badge 徽章组件
type Badge struct {
	Container *fyne.Container
	bg        *canvas.Rectangle
	text      *canvas.Text
}

func NewBadge(label string, bgCol color.Color) *Badge {
	b := &Badge{}

	b.text = canvas.NewText(label, color.White)
	b.text.TextSize = 10
	b.text.TextStyle = fyne.TextStyle{Bold: true}
	b.text.Alignment = fyne.TextAlignCenter

	// 计算文字大小并添加 padding
	textSize := b.text.MinSize()
	// Padding: 左右 = 6, 上下 = 2
	padH := float32(6 * 2)
	padV := float32(3 * 2)
	
	b.bg = canvas.NewRectangle(bgCol)
	b.bg.CornerRadius = 4 
	// 强制设置背景的最小尺寸，Stack 容器会采用最大的最小尺寸
	b.bg.SetMinSize(fyne.NewSize(textSize.Width+padH, textSize.Height+padV))

	// 使用自定义布局严格控制高度
	l := &BadgeLayout{
		text: b.text,
		bg:   b.bg,
		padH: 6 * 2,
		padV: 4 * 2,
	}

	b.Container = container.New(l, b.bg, b.text)
	return b
}

// BadgeLayout 自定义布局，用于强制高度等于 文字大小 + padding
type BadgeLayout struct {
	text *canvas.Text
	bg   *canvas.Rectangle
	padH float32
	padV float32
}

func (l *BadgeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	// 背景充满整个空间
	l.bg.Move(fyne.NewPos(0, 0))
	l.bg.Resize(size)

	// 文字居中
	textSize := l.text.MinSize()
	textX := (size.Width - textSize.Width) / 2
	textY := (size.Height - textSize.Height) / 2
	l.text.Move(fyne.NewPos(textX, textY))
	l.text.Resize(textSize)
}

func (l *BadgeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	textSize := l.text.MinSize()
	// 高度 = 文字原本高度 (text.TextSize) + Padding，忽略行高带来的额外高度
	// textSize.Height 包含了行距，我们想要严格紧凑的高度。
	// 使用 l.text.TextSize 作为高度基准。
	targetHeight := float32(l.text.TextSize) + l.padV
	
	return fyne.NewSize(textSize.Width+l.padH, targetHeight)
}

func (b *Badge) SetText(text string) {
	b.text.Text = text
	b.text.Refresh()
	if b.Container != nil {
		b.Container.Refresh()
	}
}

func (b *Badge) SetColor(c color.Color) {
	b.bg.FillColor = c
	b.bg.Refresh()
}

func (b *Badge) SetSize(s fyne.Size) {
	b.bg.SetMinSize(s)
	// CRITICAL: Also set Container's MinSize so layout system allocates space
	if b.Container != nil {
		b.Container.Resize(s)
		b.Container.Refresh()
	}
}

func (b *Badge) Show() {
	b.Container.Show()
}

func (b *Badge) Hide() {
	b.Container.Hide()
}

// Helper colors - 使用柔和且易区分的颜色方案
var (
	Color8K    = color.RGBA{R: 102, G: 51, B: 153, A: 255}  // 深紫色 - 最高级
	Color4K    = color.RGBA{R: 184, G: 134, B: 11, A: 210}  // 深金色 - 高级
	Color2K    = color.RGBA{R: 184, G: 115, B: 51, A: 255}  // 深铜色 - 中等
	Color1080P = color.RGBA{R: 34, G: 139, B: 34, A: 255}   // 深绿色 - 标准
	Color720P  = color.RGBA{R: 25, G: 25, B: 112, A: 255}   // 深蓝色 - 基础
)

func GetResolutionColor(dim int) (string, color.Color) {
	if dim >= 4320 {
		return "8K", Color8K
	}
	if dim >= 2160 {
		return "4K", Color4K
	}
	if dim >= 1440 {
		return "2K", Color2K
	}
	if dim >= 1080 {
		return "1080P", Color1080P
	}
	if dim >= 720 {
		return "720P", Color720P
	}
	return "", nil
}
