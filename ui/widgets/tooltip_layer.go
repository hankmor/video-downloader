package widgets

import (
	"errors"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// =====================================================
// Tooltip 层管理 - 参考 fyne-tooltip 库
// 支持窗口和 PopUp 的独立 tooltip 层
// =====================================================

const (
	initialToolTipDelay             = 750 * time.Millisecond
	subsequentToolTipDelay          = 300 * time.Millisecond
	subsequentToolTipDelayValidTime = 1500 * time.Millisecond

	// tooltip 位置偏移
	belowMouseDist  = 12 // 鼠标下方距离
	aboveMouseDist  = 8  // 如果下方空间不足，显示在上方
	maxToolTipWidth = 400
)

var (
	toolTipLayers             = make(map[fyne.Canvas]*ToolTipLayer)
	toolTipMu                 sync.RWMutex
	lastToolTipShownUnixMilli int64
)

// ToolTipHandle 用于追踪 tooltip
type ToolTipHandle struct {
	canvas  fyne.Canvas
	overlay fyne.CanvasObject
}

// ToolTipLayer tooltip 层
type ToolTipLayer struct {
	Container fyne.Container
	overlays  map[fyne.CanvasObject]*ToolTipLayer
}

// NextToolTipDelayTime 返回下一个 tooltip 的延迟时间
func NextToolTipDelayTime() time.Duration {
	if time.Now().UnixMilli()-lastToolTipShownUnixMilli < subsequentToolTipDelayValidTime.Milliseconds() {
		return subsequentToolTipDelay
	}
	return initialToolTipDelay
}

// AddTooltipLayer 为窗口内容添加 tooltip 层
func AddTooltipLayer(content fyne.CanvasObject, c fyne.Canvas) fyne.CanvasObject {
	layer := &ToolTipLayer{}

	toolTipMu.Lock()
	toolTipLayers[c] = layer
	toolTipMu.Unlock()

	return container.NewStack(content, &layer.Container)
}

// DestroyTooltipLayer 销毁窗口的 tooltip 层
func DestroyTooltipLayer(c fyne.Canvas) {
	toolTipMu.Lock()
	delete(toolTipLayers, c)
	toolTipMu.Unlock()
}

// AddPopUpTooltipLayer 为 PopUp 添加 tooltip 层
func AddPopUpTooltipLayer(popUp *widget.PopUp) {
	toolTipMu.Lock()
	defer toolTipMu.Unlock()

	ct := toolTipLayers[popUp.Canvas]
	if ct == nil {
		fyne.LogError("", errors.New("no tool tip layer created for parent canvas"))
		return
	}

	layer := &ToolTipLayer{}
	if ct.overlays == nil {
		ct.overlays = make(map[fyne.CanvasObject]*ToolTipLayer)
	}
	ct.overlays[popUp] = layer

	// 将 tooltip 层添加到 PopUp 的内容中
	popUp.Content = container.NewStack(popUp.Content, &layer.Container)
}

// DestroyPopUpTooltipLayer 销毁 PopUp 的 tooltip 层
func DestroyPopUpTooltipLayer(popUp *widget.PopUp) {
	toolTipMu.Lock()
	defer toolTipMu.Unlock()

	ct := toolTipLayers[popUp.Canvas]
	if ct != nil {
		delete(ct.overlays, popUp)
	}
}

// ShowToolTipAtMousePosition 在鼠标位置显示 tooltip
func ShowToolTipAtMousePosition(c fyne.Canvas, pos fyne.Position, text string) *ToolTipHandle {
	if c == nil {
		fyne.LogError("", errors.New("no canvas associated with tool tip widget"))
		return nil
	}

	lastToolTipShownUnixMilli = time.Now().UnixMilli()
	overlay := c.Overlays().Top()
	handle := &ToolTipHandle{canvas: c, overlay: overlay}

	// 创建 tooltip
	t := createToolTip(text)

	// 检查是否在 overlay 中
	if overlay != nil {
		// 在 overlay 中（dialog/popup），检查是否有注册的层
		tl := findToolTipLayer(handle, false)
		if tl != nil && tl != toolTipLayers[c] {
			// 有注册的 overlay 层，使用它
			tl.Container.Objects = []fyne.CanvasObject{t}

			var zeroPos fyne.Position
			if pop, ok := overlay.(*widget.PopUp); ok && pop != nil {
				zeroPos = pop.Content.Position()
			}
			sizeAndPositionToolTip(zeroPos, pos.Subtract(zeroPos), t, c)
			tl.Container.Refresh()
			return handle
		}

		// 没有注册的 overlay 层，使用非模态 PopUp 包装 tooltip
		// 非模态 PopUp 允许点击穿透到底层
		tooltipPopUp := widget.NewPopUp(t, c)
		// 计算 tooltip 位置：在鼠标下方 belowMouseDist 处
		tooltipPos := fyne.NewPos(pos.X, pos.Y+belowMouseDist)
		// 边界检查
		canvasSize := c.Size()
		tooltipSize := t.MinSize()
		// 如果会超出底部，显示在鼠标上方
		if tooltipPos.Y+tooltipSize.Height > canvasSize.Height {
			tooltipPos.Y = pos.Y - tooltipSize.Height - aboveMouseDist
		}
		// 如果会超出右边界，向左移动
		if tooltipPos.X+tooltipSize.Width > canvasSize.Width {
			tooltipPos.X = canvasSize.Width - tooltipSize.Width - 4
		}
		tooltipPopUp.ShowAtPosition(tooltipPos)
		handle.overlay = tooltipPopUp // 记录 PopUp 以便隐藏
		return handle
	}

	// 在主窗口中，使用 tooltip 层
	tl := findToolTipLayer(handle, true)
	if tl == nil {
		return nil
	}

	tl.Container.Objects = []fyne.CanvasObject{t}
	zeroPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(&tl.Container)
	sizeAndPositionToolTip(zeroPos, pos.Subtract(zeroPos), t, c)
	tl.Container.Refresh()
	return handle
}

// HideToolTip 隐藏 tooltip
func HideToolTip(handle *ToolTipHandle) {
	if handle == nil {
		return
	}

	// 检查是否是 PopUp 包装的 tooltip
	if handle.overlay != nil {
		if pop, ok := handle.overlay.(*widget.PopUp); ok {
			pop.Hide()
		} else if handle.canvas != nil {
			// 尝试从 overlay 中移除
			handle.canvas.Overlays().Remove(handle.overlay)
		}
	}

	// 尝试从 tooltip 层中移除
	tl := findToolTipLayer(handle, false)
	if tl != nil {
		tl.Container.Objects = nil
		tl.Container.Refresh()
	}
}

// findToolTipLayer 查找 tooltip 层
// 对于 overlay 中的组件，fallback 到使用主窗口的 tooltip 层
func findToolTipLayer(handle *ToolTipHandle, logErr bool) *ToolTipLayer {
	toolTipMu.RLock()
	defer toolTipMu.RUnlock()

	tl := toolTipLayers[handle.canvas]
	if tl == nil {
		if logErr {
			fyne.LogError("", errors.New("no tool tip layer created for window canvas"))
		}
		return nil
	}

	// 对于 overlay 中的组件，检查是否有注册的层
	// 如果没有，fallback 到主窗口层（tooltip 会渲染在主窗口层但仍然可见）
	if handle.overlay != nil {
		if overlayLayer := tl.overlays[handle.overlay]; overlayLayer != nil {
			return overlayLayer
		}
		// 没有注册的 overlay 层，fallback 到主窗口层
		// 这样 tooltip 仍然可以显示，只是在 overlay 下面（但通常用户会看到）
	}

	return tl
}

// sizeAndPositionToolTip 计算并设置 tooltip 的大小和位置
func sizeAndPositionToolTip(zeroPos, relPos fyne.Position, t fyne.CanvasObject, c fyne.Canvas) {
	canvasSize := c.Size()
	canvasPad := theme.Padding()
	tooltipSize := t.MinSize()

	// 计算宽度
	w := fyne.Min(tooltipSize.Width, fyne.Min(canvasSize.Width-canvasPad*2, maxToolTipWidth))
	t.Resize(fyne.NewSize(w, tooltipSize.Height))
	tooltipSize = t.Size()

	// 水平位置：如果会超出右边界，向左移动
	if rightEdge := relPos.X + zeroPos.X + tooltipSize.Width; rightEdge > canvasSize.Width-canvasPad {
		relPos.X -= rightEdge - canvasSize.Width + canvasPad
	}
	// 确保不超出左边界
	if relPos.X+zeroPos.X < canvasPad {
		relPos.X = canvasPad - zeroPos.X
	}

	// 垂直位置：优先显示在鼠标下方
	if bottomEdge := relPos.Y + zeroPos.Y + tooltipSize.Height + belowMouseDist; bottomEdge > canvasSize.Height-canvasPad {
		// 下方空间不足，显示在上方
		relPos.Y -= tooltipSize.Height + aboveMouseDist
	} else {
		relPos.Y += belowMouseDist
	}

	t.Move(relPos)
}

// createToolTip 创建 tooltip 组件
func createToolTip(text string) fyne.CanvasObject {
	textObj := canvas.NewText(text, theme.Color(theme.ColorNameForeground))
	textObj.TextSize = 11
	textObj.Alignment = fyne.TextAlignCenter

	bgRect := canvas.NewRectangle(theme.Color(theme.ColorNameOverlayBackground))
	bgRect.CornerRadius = 6
	bgRect.StrokeColor = theme.Color(theme.ColorNameInputBorder)
	bgRect.StrokeWidth = 1

	textMinSize := textObj.MinSize()
	paddingH := float32(12)
	paddingV := float32(6)

	tooltipWidth := textMinSize.Width + paddingH*2
	tooltipHeight := textMinSize.Height + paddingV*2

	bgRect.Resize(fyne.NewSize(tooltipWidth, tooltipHeight))

	tooltipContainer := container.NewStack(bgRect, container.NewCenter(textObj))
	tooltipContainer.Resize(fyne.NewSize(tooltipWidth, tooltipHeight))

	return tooltipContainer
}
