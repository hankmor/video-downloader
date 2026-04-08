package windows

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var licenseWindow fyne.Window

// licenseCallbacks 保留兼容，不再触发授权状态变更。
var licenseCallbacks []func()

func RegisterLicenseCallback(callback func()) {
	licenseCallbacks = append(licenseCallbacks, callback)
}

// ShowLicenseWindow 开源版提示：无需激活。
func ShowLicenseWindow(app fyne.App) {
	if licenseWindow != nil {
		licenseWindow.Show()
		licenseWindow.RequestFocus()
		return
	}

	w := app.NewWindow("开源版说明")
	licenseWindow = w
	w.SetOnClosed(func() {
		licenseWindow = nil
	})
	w.Resize(fyne.NewSize(460, 220))
	w.CenterOnScreen()

	content := container.NewVBox(
		widget.NewLabelWithStyle("VDD 开源版", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("当前版本已移除激活与授权校验逻辑。"),
		widget.NewLabel("所有功能默认可用，无需输入激活码。"),
		widget.NewSeparator(),
		container.NewCenter(widget.NewButton("关闭", func() {
			w.Close()
		})),
	)

	w.SetContent(container.NewPadded(content))
	w.Show()
}
