package widgets

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ClickableToast 可点击的 Toast 提示
type ClickableToast struct {
	message  string
	onClick  func()
	popup    *widget.PopUp
	window   fyne.Window
	duration time.Duration
}

// NewClickableToast 创建可点击的 Toast
func NewClickableToast(message string, onClick func()) *ClickableToast {
	return &ClickableToast{
		message:  message,
		onClick:  onClick,
		duration: 4 * time.Second, // 默认4秒自动消失
	}
}

// ShowToast 显示 Toast
func (t *ClickableToast) ShowToast(window fyne.Window) {
	if window == nil {
		return
	}
	t.window = window

	// 创建标签
	label := widget.NewLabel(t.message)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Bold: false}
	label.Wrapping = fyne.TextWrapWord

	// 创建可点击的按钮（透明）
	btn := widget.NewButton("", func() {
		if t.onClick != nil {
			t.onClick()
		}
		t.Hide()
	})
	btn.Importance = widget.LowImportance

	// 内容区域
	content := container.NewStack(
		btn,
		container.NewPadded(
			container.NewMax(
				container.NewVBox(
					layout.NewSpacer(),
					label,
					layout.NewSpacer(),
				),
			),
		),
	)

	// 背景
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	bg.CornerRadius = theme.InputRadiusSize()

	// 最终容器
	popupContainer := container.NewStack(bg, content)

	// 创建 Popup
	t.popup = widget.NewPopUp(popupContainer, window.Canvas())

	// 计算尺寸和位置
	minSize := popupContainer.MinSize()
	width := minSize.Width + toastPaddingX*2
	if width < toastMinWidth {
		width = toastMinWidth
	}
	if width > toastMaxWidth {
		width = toastMaxWidth
	}

	height := minSize.Height + toastPaddingY*2
	if height < toastMinHeight {
		height = toastMinHeight
	}

	canvasSize := window.Canvas().Size()
	x := (canvasSize.Width - width) / 2
	y := canvasSize.Height - height - toastMargin

	t.popup.Resize(fyne.NewSize(width, height))
	t.popup.Move(fyne.NewPos(x, y))
	t.popup.Show()

	// 自动隐藏
	go func() {
		time.Sleep(t.duration)
		fyne.Do(func() {
			t.Hide()
		})
	}()
}

// Hide 隐藏 Toast
func (t *ClickableToast) Hide() {
	if t.popup != nil {
		t.popup.Hide()
		t.popup = nil
	}
}
