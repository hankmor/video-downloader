package windows

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/ui/helper"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/ui/themes"
	"github.com/hankmor/vdd/ui/widgets"
	"github.com/hankmor/vdd/utils"

	"github.com/hankmor/vdd/core/download" // Added import for download package
	// Added import for theme package
)

var settingsWindow fyne.Window

// ShowSettingsWindow 显示设置窗口
func ShowSettingsWindow(app fyne.App, parent fyne.Window, clipboardMonitor *ClipboardMonitor, downloader *download.Downloader) {
	// 如果窗口已存在，则激活它
	if settingsWindow != nil {
		settingsWindow.Show()
		settingsWindow.RequestFocus()
		return
	}
	w := app.NewWindow("设置")
	settingsWindow = w
	w.SetOnClosed(func() {
		settingsWindow = nil
		w.Close()
	})
	w.Resize(fyne.NewSize(700, 500))
	w.CenterOnScreen()

	cfg := config.Get()

	// 1. 下载目录设置
	downloadDirEntry := widget.NewEntry()
	downloadDirEntry.SetPlaceHolder("选择下载目录...")
	downloadDirEntry.SetText(cfg.DownloadDir)

	browseBtn := widget.NewButtonWithIcon("", icons.ThemedSearchIcon, func() {
		helper.ShowFolderOpen(w, "选择下载目录", func(dir string, err error) {
			if err == nil && dir != "" {
				fyne.Do(func() {
					downloadDirEntry.SetText(dir)
				})
			}
		})
	})

	openBtn := widget.NewButtonWithIcon("", icons.ThemedOpenFolderIcon, func() {
		utils.OpenFolder(downloadDirEntry.Text)
	})

	dirContainer := container.NewBorder(nil, nil, nil, container.NewHBox(openBtn, browseBtn), downloadDirEntry)

	// 2. 并发数设置
	concurrentSelect := widget.NewSelect([]string{"1", "2", "3", "4", "5", "10", "20"}, func(s string) {
	})
	// 设置当前值
	currentConcurrent := "3"
	if cfg.MaxConcurrent > 0 && cfg.MaxConcurrent <= config.MaxConcurrent {
		currentConcurrent = fmt.Sprintf("%d", cfg.MaxConcurrent)
	}
	concurrentSelect.SetSelected(currentConcurrent)

	// 3. 自动下载设置
	autoDownloadCheck := widget.NewCheck("自动下载推荐格式", nil)
	autoDownloadCheck.Checked = cfg.AutoDownload

	// 3.1 自动扫描订阅
	autoScanCheck := widget.NewCheck("启动时扫描订阅", nil)
	autoScanCheck.Checked = cfg.AutoScanSubscriptions

	// 3.2 后台自动扫描 (分钟)
	backgroundScanCheck := widget.NewCheck("启用后台定时更新", nil)
	backgroundScanCheck.Checked = cfg.AutoBackgroundScan

	scanIntervalEntry := widget.NewEntry()
	scanIntervalEntry.SetPlaceHolder("10")
	if cfg.BackgroundScanInterval > 0 {
		scanIntervalEntry.SetText(fmt.Sprintf("%d", cfg.BackgroundScanInterval))
	} else {
		scanIntervalEntry.SetText("10")
	}
	
	// 验证只能输入数字
	scanIntervalEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		if _, err := strconv.Atoi(s); err != nil {
			scanIntervalEntry.SetText(fmt.Sprintf("%d", cfg.BackgroundScanInterval))
		}
	}

	// 联动：只有启用时才能输入间隔
	if !backgroundScanCheck.Checked {
		scanIntervalEntry.Disable()
	}
	backgroundScanCheck.OnChanged = func(checked bool) {
		if checked {
			scanIntervalEntry.Enable()
		} else {
			scanIntervalEntry.Disable()
		}
	}

	scanIntervalContainer := container.NewBorder(nil, nil, widget.NewLabel("后台更新间隔(分钟):"), nil, scanIntervalEntry)

	// 4.1 深色/浅色样式微调
	// 浅色样式
	lightStyleSelect := widget.NewSelect([]string{"polar", "latte"}, nil)
	lightStyleSelect.SetSelected(cfg.LightStyle)

	// 深色样式
	darkStyleSelect := widget.NewSelect([]string{"titanium", "cyberpunk"}, nil)
	darkStyleSelect.SetSelected(cfg.DarkStyle)

	// 辅助函数：根据主模式更新样式选择器的可用性
	updateStyleState := func(mode string) {
		switch mode {
		case "light":
			lightStyleSelect.Enable()
			darkStyleSelect.Disable()
		case "dark":
			lightStyleSelect.Disable()
			darkStyleSelect.Enable()
		default: // auto
			lightStyleSelect.Enable()
			darkStyleSelect.Enable()
		}
	}

	// 4. 主题设置
	themeSelect := widget.NewSelect([]string{"auto", "light", "dark"}, func(s string) {
		updateStyleState(s)
	})

	// 根据配置值设置当前选项
	switch cfg.Theme {
	case "light":
		themeSelect.SetSelected("light")
	case "dark":
		themeSelect.SetSelected("dark")
	default: // auto
		themeSelect.SetSelected("auto")
	}
	// 初始化状态
	updateStyleState(themeSelect.Selected)

	// 将其放入一个 HBox 或 Grid
	// styleContainer := container.NewGridWithColumns(2,
	// 	widget.NewFormItem("浅色样式", lightStyleSelect).Widget, // Hacky manual composition if NewFormItem doesn't return widget directly... Wait, NewFormItem returns *FormItem which is struct.
	// 	// Let's use Form Layout properly:
	// )
	// Fyne Form lays out vertically. We can add more items to the main form.

	// 5. 字幕设置
	subtitleCheck := widget.NewCheck("下载字幕 (中/英)", nil)
	subtitleCheck.Checked = cfg.Subtitle

	// 6. 剪贴板设置
	clipboardCheck := widget.NewCheck("监听剪贴板 (自动填入链接)", nil)
	clipboardCheck.Checked = cfg.ClipboardMonitor

	// 7. FFmpeg 路径 (高级)
	ffmpegEntry := widget.NewEntry()
	ffmpegEntry.SetText(cfg.FFmpegPath)
	ffmpegEntry.SetPlaceHolder("留空使用系统默认版本")
	ffmpegHelp := createHelpButton("FFmpeg 配置说明",
		`精简版需要配置 FFmpeg 用于合并分离的视频和音频流，以及格式转换。
如果您的系统已安装 FFmpeg 并添加系统环境变量，此处可留空。
否则请指定 ffmpeg 可执行文件的完整路径。

注意：完整版已经集成 ffmpeg，无需额外配置。`, w)
	ffmpegContainer := container.NewBorder(nil, nil, nil, ffmpegHelp, ffmpegEntry)

	// 7. 代理设置 (高级)
	proxyEntry := widget.NewEntry()
	proxyEntry.SetText(cfg.ProxyURL)
	proxyEntry.SetPlaceHolder("http://127.0.0.1:7890 或 socks5://...")
	proxyHelp := createHelpButton("代理配置说明",
		`代理设置用于绕过网络限制下载 YouTube 等视频。
格式示例：
1. http://127.0.0.1:7890
2. socks5://127.0.0.1:1080
请查看您的代理软件设置以获取具体代理链接地址。`, w)
	proxyContainer := container.NewBorder(nil, nil, nil, proxyHelp, proxyEntry)

	// 8. Cookie 设置
	browserCookieCheck := widget.NewCheck("优先从浏览器获取 Cookie (推荐)", nil)
	browserCookieCheck.Checked = cfg.EnableBrowserCookie

	browsers := []string{"chrome", "firefox", "edge", "safari", "chromium", "brave", "vivaldi", "opera"}
	browserSelect := widget.NewSelect(browsers, nil)
	browserSelect.PlaceHolder = "选择浏览器..."
	browserSelect.SetSelected(cfg.BrowserName)
	if !cfg.EnableBrowserCookie {
		browserSelect.Disable()
	}

	browserCookieCheck.OnChanged = func(checked bool) {
		if checked {
			browserSelect.Enable()
		} else {
			browserSelect.Disable()
		}
	}

	cookiesHelp := createHelpButton("Cookie 配置说明",
		`启用后，VDD 将尝试从指定浏览器读取登录状态。
无需手动导出 cookies.txt 文件。
支持 Chrome, Edge, Firefox, Safari 等主流浏览器。

注意：
1. 首次使用可能需要系统授权 (macOS KeyChain)。
2. 请确保在浏览器中已登录视频网站账号。`, w)

	cookiesContainer := container.NewBorder(nil, nil, nil, cookiesHelp,
		container.NewVBox(browserCookieCheck, browserSelect))

	// 辅助函数：给控件增加 Padding 以增加行高
	wrapPadded := func(o fyne.CanvasObject) fyne.CanvasObject {
		// 垂直方向增加更多 Padding，水平方向保持默认或适度
		// 使用 NewVBox(NewPadded) 可以撑大高度
		return container.NewPadded(o)
	}

	// 1. 下载目录设置
	// ... (DownloadDir logic)
	// 在 Form 中使用

	// 9. 文件名格式设置
	nameOptions := []string{
		"视频标题.ext (默认)",
		"视频标题 - 发布者.ext",
		"发布者 - 视频标题.ext",
		"发布日期 - 视频标题.ext",
	}
	nameSelect := widget.NewSelect(nameOptions, nil)
	switch cfg.FilenameFormat {
	case "title_uploader":
		nameSelect.SetSelected(nameOptions[1])
	case "uploader_title":
		nameSelect.SetSelected(nameOptions[2])
	case "date_title":
		nameSelect.SetSelected(nameOptions[3])
	default:
		nameSelect.SetSelected(nameOptions[0])
	}

	// 10. 分辨率策略设置
	resOptions := []string{
		"最高画质 (默认)",
		"节省空间 (720p)",
	}
	resSelect := widget.NewSelect(resOptions, nil)
	if cfg.ResolutionStrategy == "saver" {
		resSelect.SetSelected(resOptions[1])
	} else {
		resSelect.SetSelected(resOptions[0])
	}

	resContainer := container.NewBorder(nil, nil, nil, createHelpButton("分辨率策略说明",
		"最高画质 (默认):\n下载源提供的最高清晰度视频 (如 4K/8K)，但文件体积较大。\n\n节省空间 (720p):\n均衡模式，一般不超过 720p，既保证清晰度又节省磁盘空间。\n如果视频本身低于 720p，则下载最高可用画质。\n此选项特别适合长期订阅下载。", w),
		resSelect)

	// 表单布局
	// 使用 wrapPadded 包裹每一个 Widget，增加行高
	// generalForm
	generalForm := widget.NewForm(
		widget.NewFormItem("下载目录", wrapPadded(dirContainer)),
		widget.NewFormItem("解析后自动下载", wrapPadded(autoDownloadCheck)),
		widget.NewFormItem("视频分辨率策略", wrapPadded(resContainer)),
		widget.NewFormItem("视频文件名格式", wrapPadded(nameSelect)),
		widget.NewFormItem("自动下载字幕", wrapPadded(subtitleCheck)),
		widget.NewFormItem("启用剪贴板监听", wrapPadded(clipboardCheck)),
		widget.NewFormItem("最大并发数", wrapPadded(concurrentSelect)),
	)

	// subForm
	subForm := widget.NewForm(
		widget.NewFormItem("启动时自动扫描", wrapPadded(autoScanCheck)),
		widget.NewFormItem("后台定时扫描", wrapPadded(backgroundScanCheck)),
		widget.NewFormItem("", wrapPadded(scanIntervalContainer)),
	)

	// appearanceForm
	appearanceForm := widget.NewForm(
		widget.NewFormItem("界面主题", wrapPadded(themeSelect)),
		widget.NewFormItem("   └ 浅色样式", wrapPadded(lightStyleSelect)),
		widget.NewFormItem("   └ 深色样式", wrapPadded(darkStyleSelect)),
	)

	// advancedForm
	advancedForm := widget.NewForm(
		widget.NewFormItem("FFmpeg 路径 (可选)", wrapPadded(ffmpegContainer)),
		widget.NewFormItem("代理设置 (可选)", wrapPadded(proxyContainer)),
		widget.NewFormItem("Cookie 设置", wrapPadded(cookiesContainer)),
	)

	// === 页面管理 ===
	type page struct {
		Name    string
		Icon    fyne.Resource
		Content fyne.CanvasObject
	}

	pages := []page{
		{"基础设置", theme.SettingsIcon(), generalForm},
		{"订阅设置", icons.ThemedSubscriptionsIcon, subForm},
		{"外观设置", theme.ColorPaletteIcon(), appearanceForm},
		{"高级设置", theme.ComputerIcon(), advancedForm},
	}

	// 内容区域容器
	contentContainer := container.NewStack()
	// 加两个 Padded 增加内容区四周留白
	contentContainer.Add(container.NewPadded(container.NewPadded(pages[0].Content)))

	// 侧边栏列表
	sidebarList := widget.NewList(
		func() int { return len(pages) },
		func() fyne.CanvasObject {
			// 左侧菜单行高增加：加两层 Padding，或者自定义 Layout
			// 这里使用 GridWrap 撑大高度? 不，List Item 默认是自适应。
			// 使用 VBox + Padding
			title := widget.NewLabel("Template")
			title.TextStyle = fyne.TextStyle{Bold: true} // 稍微加粗菜单文字
			return container.NewPadded(container.NewHBox(widget.NewIcon(nil), title))
		},
		func(id int, o fyne.CanvasObject) {
			p := pages[id]
			// o 是 Padded -> HBox
			hbox := o.(*fyne.Container).Objects[0].(*fyne.Container)
			icon := hbox.Objects[0].(*widget.Icon)
			label := hbox.Objects[1].(*widget.Label)
			
			icon.SetResource(p.Icon)
			label.SetText(p.Name)
		},
	)
	
	sidebarList.OnSelected = func(id int) {
		// 切换内容时也保持双重 Padding
		contentContainer.Objects = []fyne.CanvasObject{container.NewPadded(container.NewPadded(pages[id].Content))}
		contentContainer.Refresh()
	}
	
	// 选中第一项
	sidebarList.Select(0)
	
	// 底部按钮
	// ... (保持不变)
	saveBtn := widget.NewButtonWithIcon("", icons.ThemedOkIcon, func() {
    // ... (save logic)
		// --- 保存逻辑 ---
		// 1. General
		cfg.DownloadDir = downloadDirEntry.Text
		cfg.AutoDownload = autoDownloadCheck.Checked
		cfg.Subtitle = subtitleCheck.Checked
		cfg.ClipboardMonitor = clipboardCheck.Checked
		cfg.MaxConcurrent, _ = strconv.Atoi(concurrentSelect.Selected)
		
		switch nameSelect.Selected {
		case nameOptions[1]:
			cfg.FilenameFormat = "title_uploader"
		case nameOptions[2]:
			cfg.FilenameFormat = "uploader_title"
		case nameOptions[3]:
			cfg.FilenameFormat = "date_title"
		default:
			cfg.FilenameFormat = "title"
		}
		
		if resSelect.Selected == resOptions[1] {
			cfg.ResolutionStrategy = "saver"
		} else {
			cfg.ResolutionStrategy = "best"
		}

		// 2. Subscription
		cfg.AutoScanSubscriptions = autoScanCheck.Checked
		cfg.AutoBackgroundScan = backgroundScanCheck.Checked
		if iv, err := strconv.Atoi(scanIntervalEntry.Text); err == nil && iv > 0 {
			cfg.BackgroundScanInterval = iv
		}
		
		// 3. Appearance
		// Theme map...
		switch themeSelect.Selected {
		case "light":
			cfg.Theme = "light"
		case "dark":
			cfg.Theme = "dark"
		default:
			cfg.Theme = "auto"
		}
		cfg.LightStyle = lightStyleSelect.Selected
		if cfg.LightStyle == "" { cfg.LightStyle = "polar" }
		cfg.DarkStyle = darkStyleSelect.Selected
		if cfg.DarkStyle == "" { cfg.DarkStyle = "titanium" }

		// 4. Advanced
		cfg.FFmpegPath = ffmpegEntry.Text
		cfg.ProxyURL = proxyEntry.Text
		cfg.EnableBrowserCookie = browserCookieCheck.Checked
		cfg.BrowserName = browserSelect.Selected

		// 执行保存
		if err := config.Save(); err != nil {
			dialog.ShowError(err, w)
			return
		}

		// --- 应用更改 ---
		
		// 应用主题
		themes.SetLightStyle(cfg.LightStyle)
		themes.SetDarkStyle(cfg.DarkStyle)
		w.Close() // 先关闭窗口
		
		// 延迟刷新主题
		time.AfterFunc(200*time.Millisecond, func() {
			fyne.Do(func() {
				switch cfg.Theme {
				case "light":
					app.Settings().SetTheme(themes.NewLightTheme())
				case "dark":
					app.Settings().SetTheme(themes.NewDarkTheme())
				default:
					app.Settings().SetTheme(&themes.VDDTheme{})
				}
			})
		})

		// 触发下载调度
		if downloader != nil {
			downloader.TriggerSchedule()
		}

		// 更新剪贴板
		if cfg.ClipboardMonitor {
			clipboardMonitor.Start()
		} else {
			clipboardMonitor.Stop()
		}
	})
	saveBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButtonWithIcon("", icons.ThemedCancelIcon, func() {
		w.Close()
	})

	bottomBar := container.NewHBox(
		layout.NewSpacer(),
		saveBtn,
		cancelBtn,
		layout.NewSpacer(),
	)

	// 分割线布局
	sidebarContainer := container.NewBorder(nil, nil, nil, nil, sidebarList)
	
	split := container.NewHSplit(
		container.NewPadded(sidebarContainer), 
		container.NewPadded(contentContainer), // 这里已经在 contentContainer 内部加了 padding，外部再加两层可能有点多，保持一层
	)
	split.SetOffset(0.25) // 左侧占 25%

	content := container.NewBorder(
		nil,
		container.NewPadded(bottomBar),
		nil, nil,
		split, 
	)

	w.SetContent(content)
	w.Show()
}

// createHelpButton 创建帮助按钮
func createHelpButton(title, message string, parent fyne.Window) *widget.Button {
	btn := widget.NewButtonWithIcon("", icons.ThemedHelpIcon, func() {
		// 使用共用的提示框组件
		widgets.ShowInformation(title, message, parent)
	})
	return btn
}
