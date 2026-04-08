package widgets

import (
	"net/url"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/hankmor/vdd/ui/icons"
)

var urlPattern = regexp.MustCompile(`(https?://[^\s]+|www\.[^\s]+)`)

// minWidthContainer 最小宽度容器
type minWidthContainer struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	minWidth float32
}

// NewMinWidthContainer 创建最小宽度容器
func NewMinWidthContainer(content fyne.CanvasObject, minWidth float32) *minWidthContainer {
	m := &minWidthContainer{
		content:  content,
		minWidth: minWidth,
	}
	m.ExtendBaseWidget(m)
	return m
}

// MinSize 返回最小尺寸
func (m *minWidthContainer) MinSize() fyne.Size {
	size := m.content.MinSize()
	if size.Width < m.minWidth {
		size.Width = m.minWidth
	}
	return size
}

// CreateRenderer 创建渲染器
func (m *minWidthContainer) CreateRenderer() fyne.WidgetRenderer {
	return &minWidthRenderer{
		container: m,
		content:   m.content,
	}
}

// minWidthRenderer 渲染器
type minWidthRenderer struct {
	container *minWidthContainer
	content   fyne.CanvasObject
}

func (r *minWidthRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *minWidthRenderer) MinSize() fyne.Size {
	return r.container.MinSize()
}

func (r *minWidthRenderer) Refresh() {
	r.content.Refresh()
}

func (r *minWidthRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *minWidthRenderer) Destroy() {}

func ShowDialog(title string, content fyne.CanvasObject, parent fyne.Window) {
	// 创建对话框变量，以便在按钮回调中使用
	var d dialog.Dialog

	// 创建确认按钮（带 OK 图标），点击后关闭对话框
	confirmBtn := widget.NewButtonWithIcon("", icons.ThemedOkIcon, func() {
		if d != nil {
			d.Hide()
		}
	})

	// 创建内容容器，添加适当的间距
	innerContent := container.NewVBox(
		container.NewPadded(content),                         // 添加内边距
		container.NewPadded(container.NewCenter(confirmBtn)), // 按钮居中并添加内边距
	)

	// 创建包装容器，设置最小宽度（400px）
	minWidth := float32(400)
	container := NewMinWidthContainer(innerContent, minWidth)

	// 创建自定义对话框（第三个参数传入空字符串，因为我们已经在内容中包含了按钮）
	d = dialog.NewCustom(title, "", container, parent)

	// 清空默认按钮，因为我们已经在内容中包含了自定义按钮
	if customDlg, ok := d.(*dialog.CustomDialog); ok {
		customDlg.SetButtons([]fyne.CanvasObject{})
	}

	d.Show()
}

// ShowInformation 显示提示框（只有一个确认按钮，带 OK 图标）
// title: 对话框标题
// message: 提示消息（支持自动检测 URL 并转换为可点击链接）
// parent: 父窗口
func ShowInformation(title, message string, parent fyne.Window, showLink ...bool) {
	// 检测消息中的 URL 并创建内容
	var messageContent fyne.CanvasObject

	if len(showLink) > 0 && showLink[0] {
		// 使用正则表达式检测 URL（http://, https://, www. 开头）
		matches := urlPattern.FindStringSubmatch(message)

		if len(matches) > 0 {
			// 找到 URL，将消息拆分为文本和链接
			urlStr := matches[1]
			// 如果 URL 没有协议，添加 https://
			if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
				urlStr = "https://" + urlStr
			}

			// 解析 URL
			u, err := url.Parse(urlStr)
			if err == nil {
				// 获取 URL 前的文本
				urlIndex := strings.Index(message, matches[1])
				textBefore := message[:urlIndex]
				textAfter := message[urlIndex+len(matches[1]):]

				// 创建文本标签（居中对齐）
				var contentItems []fyne.CanvasObject
				if strings.TrimSpace(textBefore) != "" {
					textLabel := widget.NewLabelWithStyle(strings.TrimSpace(textBefore), fyne.TextAlignCenter, fyne.TextStyle{})
					textLabel.Wrapping = fyne.TextWrapWord
					contentItems = append(contentItems, textLabel)
				}

				// 创建超链接（居中）
				link := widget.NewHyperlink(urlStr, u)
				contentItems = append(contentItems, container.NewCenter(link))

				if strings.TrimSpace(textAfter) != "" {
					textLabel := widget.NewLabelWithStyle(strings.TrimSpace(textAfter), fyne.TextAlignCenter, fyne.TextStyle{})
					textLabel.Wrapping = fyne.TextWrapWord
					contentItems = append(contentItems, textLabel)
				}

				messageContent = container.NewVBox(contentItems...)
			} else {
				// URL 解析失败，使用普通文本（居中对齐）
				messageLabel := widget.NewLabelWithStyle(message, fyne.TextAlignCenter, fyne.TextStyle{})
				messageLabel.Wrapping = fyne.TextWrapWord
				messageContent = messageLabel
			}
		}
	} else {
		// 没有找到 URL，使用普通文本（居中对齐）
		messageLabel := widget.NewLabelWithStyle(message, fyne.TextAlignCenter, fyne.TextStyle{})
		messageLabel.Wrapping = fyne.TextWrapWord
		messageContent = messageLabel
	}

	// 创建对话框变量，以便在按钮回调中使用
	var d dialog.Dialog

	// 创建确认按钮（带 OK 图标），点击后关闭对话框
	confirmBtn := widget.NewButtonWithIcon("", icons.ThemedOkIcon, func() {
		if d != nil {
			d.Hide()
		}
	})
	confirmBtn.Importance = widget.LowImportance

	// 创建内容容器，添加适当的间距
	innerContent := container.NewVBox(
		container.NewPadded(messageContent),                  // 添加内边距
		container.NewPadded(container.NewCenter(confirmBtn)), // 按钮居中并添加内边距
	)

	// 创建包装容器，设置最小宽度（400px）
	minWidth := float32(400)
	content := NewMinWidthContainer(innerContent, minWidth)

	// 创建自定义对话框（第三个参数传入空字符串，因为我们已经在内容中包含了按钮）
	d = dialog.NewCustom(title, "", content, parent)

	// 清空默认按钮，因为我们已经在内容中包含了自定义按钮
	if customDlg, ok := d.(*dialog.CustomDialog); ok {
		customDlg.SetButtons([]fyne.CanvasObject{})
	}

	d.Show()
}

func ShowConfirmDialog(title, message string, onConfirm func(), parent fyne.Window) dialog.Dialog {
	// 创建消息内容（居中对齐）
	messageLabel := widget.NewLabelWithStyle(message, fyne.TextAlignCenter, fyne.TextStyle{})
	messageLabel.Wrapping = fyne.TextWrapWord

	// 创建对话框变量，以便在按钮回调中使用
	var d dialog.Dialog

	// 创建 OK 按钮（带图标），点击后执行回调并关闭对话框
	okBtn := widget.NewButtonWithIcon("", icons.ThemedOkIcon, func() {
		if onConfirm != nil {
			onConfirm()
		}
		if d != nil {
			d.Hide()
		}
	})

	// 创建 Cancel 按钮（带图标），点击后直接关闭对话框
	cancelBtn := widget.NewButtonWithIcon("", icons.ThemedCancelIcon, func() {
		if d != nil {
			d.Hide()
		}
	})

	// 创建按钮容器，按钮右对齐
	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		okBtn,
		cancelBtn,
		layout.NewSpacer(),
	)

	// 创建内容容器，添加适当的间距
	innerContent := container.NewVBox(
		container.NewPadded(messageLabel),    // 添加内边距
		container.NewPadded(buttonContainer), // 按钮容器并添加内边距
	)

	// 创建包装容器，设置最小宽度（400px）
	minWidth := float32(400)
	content := NewMinWidthContainer(innerContent, minWidth)

	// 创建自定义对话框（第三个参数传入空字符串，因为我们已经在内容中包含了按钮）
	d = dialog.NewCustom(title, "", content, parent)

	// 清空默认按钮，因为我们已经在内容中包含了自定义按钮
	if customDlg, ok := d.(*dialog.CustomDialog); ok {
		customDlg.SetButtons([]fyne.CanvasObject{})
	}
	d.Show()
	return d
}

// ShowConfirmWithContent 显示带自定义内容的确认框（有 OK 和 Cancel 按钮）
// title: 对话框标题
// content: 自定义内容（可以包含多个组件，如 Label、Checkbox 等）
// onConfirm: 点击 OK 按钮时的回调函数
// parent: 父窗口
func ShowConfirmWithContent(title string, content fyne.CanvasObject, onConfirm func(), parent fyne.Window) {
	// 创建对话框变量，以便在按钮回调中使用
	var d dialog.Dialog

	// 创建 OK 按钮（带图标），点击后执行回调并关闭对话框
	okBtn := widget.NewButtonWithIcon("", icons.ThemedOkIcon, func() {
		if onConfirm != nil {
			onConfirm()
		}
		if d != nil {
			d.Hide()
		}
	})

	// 创建 Cancel 按钮（带图标），点击后直接关闭对话框
	cancelBtn := widget.NewButtonWithIcon("", icons.ThemedCancelIcon, func() {
		if d != nil {
			d.Hide()
		}
	})
	okBtn.Importance = widget.LowImportance
	cancelBtn.Importance = widget.LowImportance

	// 创建按钮容器，按钮右对齐
	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		okBtn,
		cancelBtn,
		layout.NewSpacer(),
	)

	// 创建内容容器，添加适当的间距
	innerContent := container.NewVBox(
		container.NewPadded(content),         // 自定义内容，添加内边距
		container.NewPadded(buttonContainer), // 按钮容器并添加内边距
	)

	// 创建包装容器，设置最小宽度（400px）
	minWidth := float32(400)
	wrappedContent := NewMinWidthContainer(innerContent, minWidth)

	// 创建自定义对话框（第三个参数传入空字符串，因为我们已经在内容中包含了按钮）
	d = dialog.NewCustom(title, "", wrappedContent, parent)

	// 清空默认按钮，因为我们已经在内容中包含了自定义按钮
	if customDlg, ok := d.(*dialog.CustomDialog); ok {
		customDlg.SetButtons([]fyne.CanvasObject{})
	}

	d.Show()
}

// ShowError 显示错误提示
func ShowError(title string, err error, parent fyne.Window) {
	if err == nil {
		return
	}
	ShowInformation(title, err.Error(), parent)
}
