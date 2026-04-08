package windows

import (
	"fmt"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/hankmor/vdd/core/consts"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/updater"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/ui/widgets"
)

// ShowAboutWindow 显示关于对话框
func ShowAboutWindow(app fyne.App, parent fyne.Window) {
	checkUpdateBtn := widget.NewButtonWithIcon("", icons.ThemedRefreshIcon, func() {
		checkUpdate(app, parent)
	})

	status := widget.NewLabelWithStyle(
		"开源版：已移除激活与授权限制，全部功能默认可用",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle(consts.AppName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(container.NewHBox(
			widget.NewLabelWithStyle("Version "+consts.AppVersion, fyne.TextAlignCenter, fyne.TextStyle{}),
			checkUpdateBtn,
		)),
		status,
	)

	content.Add(widget.NewSeparator())
	copyright := widget.NewRichText(&widget.TextSegment{
		Text: fmt.Sprintf("Copyright © %d-%d VDD", 2025, time.Now().Year()),
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNameDisabled,
			Alignment: fyne.TextAlignCenter,
			SizeName:  theme.SizeNameCaptionText,
		},
	})
	content.Add(container.NewCenter(copyright))

	disclaimer := widget.NewRichText(&widget.TextSegment{
		Text: "仅供个人学习研究使用，请遵守相关法律法规及平台服务条款",
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNameDisabled,
			Alignment: fyne.TextAlignCenter,
			SizeName:  theme.SizeNameCaptionText,
		},
	})
	content.Add(container.NewCenter(disclaimer))
	content.Add(widget.NewSeparator())

	widgets.ShowDialog("关于", content, parent)
}

func checkUpdate(app fyne.App, parent fyne.Window) {
	pb := widgets.NewThinProgressBar()
	progressContent := container.NewStack(pb)
	progressDialog := dialog.NewCustom("检查更新", "取消", progressContent, parent)
	progressDialog.Show()

	progressFunc := func(progress float64) {
		fyne.Do(func() {
			pb.SetValue(progress)
		})
	}

	go func() {
		info, err := updater.CheckForUpdates(consts.AppVersion, "hankmor/video-downloader", progressFunc)

		fyne.Do(func() {
			progressDialog.Hide()

			if err != nil {
				logger.Errorf("检查更新失败: %v", err)
				dialog.ShowError(fmt.Errorf("检查更新失败, 请稍后重试"), parent)
				return
			}

			if info != nil {
				widgets.ShowConfirmDialog(
					"发现新版本",
					fmt.Sprintf("最新版本: %s, 是否前往下载页面？", info.TagName),
					func() {
						u, _ := url.Parse(info.HTMLURL)
						widgets.SafeOpenURL(app, u)
					},
					parent,
				)
			} else {
				widgets.ShowInformation("检查更新", "当前已是最新版本 ("+consts.AppVersion+")", parent)
			}
		})
	}()
}
