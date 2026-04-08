package widgets

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// =====================================================
// ButtonWithTooltip - 带 tooltip 的按钮
// 使用 tooltip 层管理，支持主窗口和 dialog/popup
// =====================================================

// tooltipContext 用于管理 tooltip 状态
type tooltipContext struct {
	toolTipHandle        *ToolTipHandle
	absoluteMousePos     fyne.Position
	pendingToolTipCtx    context.Context
	pendingToolTipCancel context.CancelFunc
}

// ButtonWithTooltip 带 tooltip 的按钮
type ButtonWithTooltip struct {
	widget.Button
	tooltipContext
	toolTipText string
}

// NewButtonWithTooltip 创建带 tooltip 的按钮
func NewButtonWithTooltip(text string, icon fyne.Resource, tapped func(), tooltip string) *ButtonWithTooltip {
	btn := &ButtonWithTooltip{
		toolTipText: tooltip,
	}
	btn.ExtendBaseWidget(btn)
	btn.Text = text
	btn.Icon = icon
	btn.OnTapped = tapped
	btn.Importance = widget.LowImportance
	return btn
}

// SetOnTapped 设置点击回调
func (b *ButtonWithTooltip) SetOnTapped(f func()) {
	b.Button.OnTapped = f
}

// SetToolTip 设置 tooltip 文本
func (b *ButtonWithTooltip) SetToolTip(text string) {
	b.toolTipText = text
}

// MouseIn 鼠标进入
func (b *ButtonWithTooltip) MouseIn(e *desktop.MouseEvent) {
	b.Button.MouseIn(e)
	if b.toolTipText == "" {
		return
	}

	b.absoluteMousePos = e.AbsolutePosition
	b.setPendingToolTip()
}

// MouseOut 鼠标离开
func (b *ButtonWithTooltip) MouseOut() {
	b.Button.MouseOut()
	b.cancelToolTip()
}

// MouseMoved 鼠标移动
func (b *ButtonWithTooltip) MouseMoved(e *desktop.MouseEvent) {
	b.Button.MouseMoved(e)
	b.absoluteMousePos = e.AbsolutePosition
}

// Tapped 点击
func (b *ButtonWithTooltip) Tapped(e *fyne.PointEvent) {
	b.cancelToolTip()
	b.Button.Tapped(e)
}

// MouseDown 鼠标按下时隐藏 tooltip
func (b *ButtonWithTooltip) MouseDown(e *desktop.MouseEvent) {
	b.cancelToolTip()
	// 调用父类的 MouseDown 如果有的话
	if btn, ok := interface{}(&b.Button).(desktop.Mouseable); ok {
		btn.MouseDown(e)
	}
}

// MouseUp 鼠标释放
func (b *ButtonWithTooltip) MouseUp(e *desktop.MouseEvent) {
	// 调用父类的 MouseUp 如果有的话
	if btn, ok := interface{}(&b.Button).(desktop.Mouseable); ok {
		btn.MouseUp(e)
	}
}

// setPendingToolTip 设置待显示的 tooltip
func (b *ButtonWithTooltip) setPendingToolTip() {
	ctx, cancel := context.WithCancel(context.Background())
	b.pendingToolTipCtx, b.pendingToolTipCancel = ctx, cancel

	delay := NextToolTipDelayTime()
	go func() {
		<-time.After(delay)
		select {
		case <-ctx.Done():
			return
		default:
			fyne.Do(func() {
				b.cancelToolTip() // 清理旧的上下文
				b.showToolTip()
			})
		}
	}()
}

// showToolTip 显示 tooltip
func (b *ButtonWithTooltip) showToolTip() {
	c := fyne.CurrentApp().Driver().CanvasForObject(b)
	if c == nil {
		return
	}
	b.toolTipHandle = ShowToolTipAtMousePosition(c, b.absoluteMousePos, b.toolTipText)

	// 自动隐藏：1.5 秒后自动消失
	go func() {
		<-time.After(1000 * time.Millisecond)
		fyne.Do(func() {
			if b.toolTipHandle != nil {
				HideToolTip(b.toolTipHandle)
				b.toolTipHandle = nil
			}
		})
	}()
}

// cancelToolTip 取消/隐藏 tooltip
func (b *ButtonWithTooltip) cancelToolTip() {
	if b.pendingToolTipCancel != nil {
		b.pendingToolTipCancel()
		b.pendingToolTipCancel = nil
		b.pendingToolTipCtx = nil
	}
	if b.toolTipHandle != nil {
		HideToolTip(b.toolTipHandle)
		b.toolTipHandle = nil
	}
}
