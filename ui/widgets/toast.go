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

const (
	// Toast 默认尺寸
	toastMinWidth  = 300 // 最小宽度
	toastMaxWidth  = 500 // 最大宽度
	toastMinHeight = 50  // 最小高度

	// Toast 边距
	toastPaddingX = 20 // 左右内边距
	toastPaddingY = 12 // 上下内边距
	toastMargin   = 40 // 距离屏幕边缘的外边距
)

// ShowToast 显示一个简单的 Toast 提示
// message: 提示内容
// icon: 图标资源 (可选)
func ShowToast(parent fyne.Window, message string, icon fyne.Resource) {
	if parent == nil {
		return
	}

	// 创建标签（默认不开启换行，以便计算单行宽度）
	label := widget.NewLabel(message)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Bold: false}
	// label.Wrapping = fyne.TextWrapWord // 移除此处设置，在布局决策时动态设置

	var contentBox *fyne.Container
	if icon != nil {
		// 图标（限制最大尺寸，保持比例）
		img := canvas.NewImageFromResource(icon)
		img.FillMode = canvas.ImageFillContain // 保持比例，自适应尺寸
		img.SetMinSize(fyne.NewSize(24, 24))   // 限制大小为24x24

		// 使用 VBox(Spacer, Item, Spacer) 来实现垂直居中
		centeredImg := container.NewVBox(layout.NewSpacer(), img, layout.NewSpacer())
		centeredLabel := container.NewVBox(layout.NewSpacer(), label, layout.NewSpacer())

		// 计算内容首选宽度（假设不换行）
		// Label 默认如果不设置 Wrapping，MinSize 就是单行宽度
		singleLineWidth := label.MinSize().Width + 24 + theme.Padding()*2 // 24 is icon width

		if singleLineWidth < (toastMaxWidth - toastPaddingX*2) {
			// 如果单行能放下，使用 HBox + Center 布局，使图标和文字紧凑居中
			// 此时必须禁用自动换行，否则 HBox 会压缩 Label 宽度
			label.Wrapping = fyne.TextWrapOff

			hBox := container.NewHBox(centeredImg, centeredLabel)
			// container.NewCenter 会将 hBox 居中显示
			contentBox = container.NewCenter(hBox)
		} else {
			// 如果放不下，使用 Border 布局，让 Label 占据剩余空间并自动换行
			label.Wrapping = fyne.TextWrapWord

			contentBox = container.NewBorder(nil, nil, centeredImg, nil, centeredLabel)
		}

	} else {
		// 没有图标时，Label 充满容器
		// 同样使用 VBox 实现垂直居中
		contentBox = container.NewMax(container.NewVBox(layout.NewSpacer(), label, layout.NewSpacer()))
	}

	// 内容区域（带内边距）
	paddedContent := container.NewPadded(contentBox)

	// 背景
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	// bg.StrokeColor = theme.Color(theme.ColorNamePrimary)
	// bg.StrokeWidth = 2
	bg.CornerRadius = theme.InputRadiusSize()

	// 最终容器
	popupContainer := container.NewStack(bg, paddedContent)

	// 创建 Popup
	popup := widget.NewPopUp(popupContainer, parent.Canvas())

	// 计算尺寸和位置
	minSize := popupContainer.MinSize()

	// 应用尺寸约束
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

	toastSize := fyne.NewSize(width, height)

	// 确保标签有足够的宽度以居中显示
	// label.Resize 不是必需的，因为布局管理器会自动处理

	fyne.Do(func() {
		// 显示并调整尺寸
		popup.Show()
		popup.Resize(toastSize)

		// 居中显示（距离屏幕上边距toastMargin）
		winSize := parent.Canvas().Size()
		x := (winSize.Width - toastSize.Width) / 2
		y := float32(toastMargin) // 距离顶部固定距离
		popup.Move(fyne.NewPos(x, y))
	})
	// 自动隐藏并清理资源
	go func() {
		time.Sleep(2 * time.Second)
		// 必须捕获 popup 变量，或者直接使用，但决不能在外部置空它
		// 因为 fyne.Do 是异步的，可能在 popup = nil 之后才执行
		p := popup
		fyne.Do(func() {
			p.Hide()
		})
	}()
}
