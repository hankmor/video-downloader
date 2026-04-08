package widgets

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// =====================================================
// IconWithTooltip - 带 tooltip 的图标
// =====================================================

// IconWithTooltip 带 tooltip 的图标
type IconWithTooltip struct {
	widget.Icon
	tooltipContext
	toolTipText string
}

// NewIconWithTooltip 创建带 tooltip 的图标
func NewIconWithTooltip(res fyne.Resource, tooltip string) *IconWithTooltip {
	icon := &IconWithTooltip{
		toolTipText: tooltip,
	}
	icon.ExtendBaseWidget(icon)
	icon.SetResource(res)
	return icon
}

// SetToolTip 设置 tooltip 文本
func (i *IconWithTooltip) SetToolTip(text string) {
	i.toolTipText = text
}

// MouseIn 鼠标进入
func (i *IconWithTooltip) MouseIn(e *desktop.MouseEvent) {
	if i.toolTipText == "" {
		return
	}

	i.absoluteMousePos = e.AbsolutePosition
	i.setPendingToolTip()
}

// MouseOut 鼠标离开
func (i *IconWithTooltip) MouseOut() {
	i.cancelToolTip()
}

// MouseMoved 鼠标移动
func (i *IconWithTooltip) MouseMoved(e *desktop.MouseEvent) {
	i.absoluteMousePos = e.AbsolutePosition
}

// setPendingToolTip 设置待显示的 tooltip
// 注意：这里复用了 tooltipContext 的字段，但需要重新实现方法，
// 因为 tooltipContext 只是数据结构，逻辑方法在 ButtonWithTooltip 中
func (i *IconWithTooltip) setPendingToolTip() {
	ctx, cancel := context.WithCancel(context.Background())
	i.pendingToolTipCtx, i.pendingToolTipCancel = ctx, cancel

	delay := NextToolTipDelayTime()
	go func() {
		<-time.After(delay)
		select {
		case <-ctx.Done():
			return
		default:
			fyne.Do(func() {
				i.cancelToolTip() // 清理旧的上下文
				i.showToolTip()
			})
		}
	}()
}

// showToolTip 显示 tooltip
func (i *IconWithTooltip) showToolTip() {
	c := fyne.CurrentApp().Driver().CanvasForObject(i)
	if c == nil {
		return
	}
	i.toolTipHandle = ShowToolTipAtMousePosition(c, i.absoluteMousePos, i.toolTipText)

	// 自动隐藏：1.5 秒后自动消失
	go func() {
		<-time.After(1500 * time.Millisecond) // 这里稍微改长一点或者保持一致
		fyne.Do(func() {
			if i.toolTipHandle != nil {
				HideToolTip(i.toolTipHandle)
				i.toolTipHandle = nil
			}
		})
	}()
}

// cancelToolTip 取消/隐藏 tooltip
func (i *IconWithTooltip) cancelToolTip() {
	if i.pendingToolTipCancel != nil {
		i.pendingToolTipCancel()
		i.pendingToolTipCancel = nil
		i.pendingToolTipCtx = nil
	}
	if i.toolTipHandle != nil {
		HideToolTip(i.toolTipHandle)
		i.toolTipHandle = nil
	}
}
