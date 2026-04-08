package views

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/hankmor/vdd/assets"
	"github.com/hankmor/vdd/core/auth"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/consts"
	"github.com/hankmor/vdd/core/download"
	"github.com/hankmor/vdd/core/env"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/parser"
	"github.com/hankmor/vdd/core/recommender"
	"github.com/hankmor/vdd/core/subscription"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/ui/helper"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/ui/widgets"
	"github.com/hankmor/vdd/utils"
)

// HomeView 解析与新建任务视图
type HomeView struct {
	Container *fyne.Container

	app    fyne.App
	window fyne.Window

	// 回调函数：切换到任务列表
	OnSwitchToTasks func()
	// 回调函数：显示授权窗口
	OnShowLicense func()
	// 回调函数：切换到订阅列表
	OnSwitchToSubscriptions func()

	// Context for cancellation
	parseCtx    context.Context
	parseCancel context.CancelFunc

	// UI 组件
	urlEntry          *widget.Entry              // url输入框
	cookieSettingsBtn *widget.Button             // Cookie 设置按钮 (图标)
	parseBtn          *widgets.ButtonWithTooltip // 解析按钮
	videoInfo         *fyne.Container            // 视频信息区域
	table             *widgets.ResponsiveTable   // 响应式表格
	welcomeContainer  *fyne.Container            // 欢迎页面
	resultArea        *container.Scroll          // 结果区域

	statusLabel     *widget.Label            // 状态栏
	statusContainer *fyne.Container          // 状态容器
	progressBar     *widgets.ThinProgressBar // 这里不再用于下载进度，因为下载跳走了，但可能用于"准备中"

	cancelBtn   *widgets.ButtonWithTooltip // 取消按钮
	downloadBtn *widgets.ButtonWithTooltip // 下载按钮

	// Data
	formatData       []parser.Format   // 格式数据
	selectedFormat   *parser.Format    // 选中的格式
	currentVideoInfo *parser.VideoInfo // 视频信息
	currentTaskID    string            // 任务ID

	// State
	isParsing       bool               // 是否正在解析
	isCancelParsing bool               // 是否正在取消解析
	useCookieFile   bool               // 是否使用 Cookie 文件 (任务级开关)
	parseCancelFunc context.CancelFunc // 解析取消函数
	resolutionRanks map[int]int        // 分辨率排名
	selectedRow     int                // 当前选中的行索引 (-1 表示未选中)

	parseSpinner *widgets.SpinnerAnimator
	recommender  *recommender.FormatRecommender
	parser       *parser.VideoParser
	downloader   *download.Downloader
	subManager   *subscription.Manager
}

// NewHomeView 创建首页视图
func NewHomeView(app fyne.App, window fyne.Window, p *parser.VideoParser, d *download.Downloader, r *recommender.FormatRecommender, sm *subscription.Manager) *HomeView {
	v := &HomeView{
		app:         app,
		window:      window,
		parser:      p,
		formatData:  make([]parser.Format, 0),
		selectedRow: -1,
		recommender: r,
		downloader:  d,
		subManager:  sm,
	}
	v.buildUI()
	return v
}

// CreateToolbarItems 创建动态工具栏项
func (v *HomeView) CreateToolbarItems() []fyne.CanvasObject {
	return []fyne.CanvasObject{v.cookieSettingsBtn}
}

func (v *HomeView) buildUI() {
	// URL 输入区域
	v.urlEntry = widget.NewEntry()
	v.urlEntry.SetPlaceHolder("粘贴视频链接(支持 YouTube、Bilibili、X 等主流视频平台)...")
	v.urlEntry.OnChanged = v.onUrlChanged
	// 开发环境测试用例
	if env.IsDev {
		v.urlEntry.SetText("https://www.youtube.com/watch?v=yjBUnbRgiNs")
	}

	// Cookie 设置按钮 (极简风格)
	v.cookieSettingsBtn = widget.NewButtonWithIcon("", icons.ThemedCookieIcon, v.showCookieSettingsDialog)
	v.updateCookieBtnState() // 初始化状态

	// 解析按钮
	v.parseBtn = widgets.NewButtonWithTooltip("", icons.ThemedSearchIcon, v.onParse, "解析链接")

	// URL 组合框: [URL输入] [解析按钮]
	urlBox := container.NewBorder(
		nil, nil,
		nil,        // Left (Leading)
		v.parseBtn, // Right (Trailing)
		v.urlEntry,
	)

	// 输入区域
	inputArea := container.NewVBox(urlBox)

	// 视频信息区域
	v.videoInfo = container.NewVBox()

	// 表格
	v.createTable()

	// 初始化欢迎页面
	v.setupWelcomePage()

	// 中心堆叠
	centerStack := container.NewStack(v.resultArea, v.welcomeContainer)

	// 底部控制
	v.statusLabel = widget.NewLabel("空闲")
	v.statusLabel.SizeName = theme.SizeNameCaptionText
	v.statusLabel.Wrapping = fyne.TextWrapBreak
	v.statusContainer = container.NewVBox(v.statusLabel)
	v.progressBar = widgets.NewThinProgressBar()
	v.progressBar.Hide()

	// 按钮
	v.cancelBtn = widgets.NewButtonWithTooltip("", icons.ThemedDeleteCircleIcon, v.OnCancelParse, "取消解析")
	v.cancelBtn.Disable()
	v.downloadBtn = widgets.NewButtonWithTooltip("", icons.ThemedDownloadCloudIcon, v.onDownloadClick, "下载推荐格式")
	v.downloadBtn.Disable() // 初始禁用

	// 底部控制栏
	controlBar := container.NewHBox(
		layout.NewSpacer(),
		v.cancelBtn,
		v.downloadBtn,
		layout.NewSpacer(),
	)

	// 主布局
	content := container.NewBorder(
		container.NewVBox(inputArea, v.videoInfo, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), controlBar, v.statusContainer, v.progressBar),
		nil, nil,
		centerStack,
	)

	v.Container = container.NewPadded(content)
}

// showCookieSettingsDialog 显示 Cookie 设置弹窗
func (v *HomeView) showCookieSettingsDialog() {
	cfg := config.Get()

	// 1. 复选框
	cookieCheck := widget.NewCheck("启用", nil)
	cookieCheck.Checked = v.useCookieFile

	// 2. 路径显示行 (路径 + 按钮)
	// 路径标签
	displayPath := cfg.CookiesPath
	if displayPath == "" {
		displayPath = "(未选择文件)"
	}
	pathLabel := widget.NewLabel(displayPath)
	pathLabel.Truncation = fyne.TextTruncateEllipsis
	// 稍微调小一点字体或者是次要颜色? Fyne 默认 Label 即可

	// 更换按钮
	changeBtn := widget.NewButtonWithIcon("", icons.ThemedOpenFolderIcon, func() {
		helper.ShowFileOpen(v.window, "选择 Cookies 文件", []string{"txt"}, func(filename string, err error) {
			if err == nil && filename != "" {
				fyne.Do(func() {
					cfg.CookiesPath = filename
					config.Save() // 更新配置
					pathLabel.SetText(filename)

					// 自动勾选
					if !cookieCheck.Checked {
						cookieCheck.SetChecked(true)
					}
				})
			}
		})
	})
	changeBtn.Importance = widget.LowImportance

	// 组合路径行
	pathRow := container.NewBorder(nil, nil, nil, changeBtn, pathLabel)

	// 初始状态可视性
	if !cookieCheck.Checked {
		pathRow.Hide()
	}

	// 联动逻辑
	cookieCheck.OnChanged = func(checked bool) {
		if checked {
			pathRow.Show()
			// 如果从未选过文件，自动触发选择
			if cfg.CookiesPath == "" || !utils.FileExists(cfg.CookiesPath) {
				changeBtn.OnTapped()
			}
		} else {
			pathRow.Hide()
		}
	}

	// 弹窗内容容器
	content := container.NewVBox(
		widget.NewLabelWithStyle("勾选以单独设置本次任务的 Cookie 文件", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		cookieCheck,
		pathRow,
	)

	widgets.ShowConfirmWithContent("Cookie 设置", content, func() {
		// 确认保存状态
		v.useCookieFile = cookieCheck.Checked
		v.updateCookieBtnState()
	}, v.window)
}

func (v *HomeView) updateCookieBtnState() {
	if v.useCookieFile {
		v.cookieSettingsBtn.Importance = widget.HighImportance
		v.cookieSettingsBtn.SetIcon(icons.ThemedCookieIcon)
	} else {
		v.cookieSettingsBtn.Importance = widget.LowImportance
		v.cookieSettingsBtn.SetIcon(icons.ThemedCookieIcon)
	}
	v.cookieSettingsBtn.Refresh()
}

// Remove old helper
// func (v *HomeView) updateCookieCheckLabel() { ... }

func (v *HomeView) createTable() {
	table := widget.NewTable(
		func() (int, int) {
			return len(v.formatData) + 1, 7
		},
		v.createTableCell,
		v.updateTableCell,
	)
	table.OnSelected = v.onTableSelected
	// 滚动容器
	columnDefs := []widgets.ColumnDef{
		{WidthPercent: 10, Alignment: fyne.TextAlignCenter},   // ID - 左对齐
		{WidthPercent: 10, Alignment: fyne.TextAlignCenter},   // 格式 - 居中
		{WidthPercent: 15, Alignment: fyne.TextAlignLeading},  // 分辨率 - 左对齐 (包含徽章)
		{WidthPercent: 25, Alignment: fyne.TextAlignCenter},   // 编码 - 左对齐
		{WidthPercent: 15, Alignment: fyne.TextAlignTrailing}, // 大小 - 右对齐
		{WidthPercent: 10, Alignment: fyne.TextAlignCenter},   // FPS/率 - 居中
		{WidthPercent: 10, Alignment: fyne.TextAlignCenter},   // 操作 - 居中 (最后一列会自动调整到100%)
	}
	v.table = widgets.NewResponsiveTable(table, columnDefs)
	v.resultArea = container.NewVScroll(v.table)
	v.resultArea.Hide()
}

func (v *HomeView) onTableSelected(id widget.TableCellID) {
	// 立即取消选中，禁用行高亮，且不触发任何操作
	v.table.Unselect(id)
}

func (v *HomeView) setupWelcomePage() {
	// 缩略图
	logoImg := canvas.NewImageFromResource(assets.StartupIcon)
	logoImg.FillMode = canvas.ImageFillContain
	logoImg.SetMinSize(fyne.NewSize(128, 128))

	// 日期时间
	now := time.Now()
	dateStr := fmt.Sprintf("%02d月%02d日 %02d:%02d %s",
		now.Month(), now.Day(),
		now.Hour(), now.Minute(),
		widgets.WeekdayToChinese(now.Weekday()))

	// 使用 RichText 以支持主题动态变色 (canvas.NewText 是静态颜色)
	dt := widget.NewRichText(&widget.TextSegment{
		Text: dateStr,
		Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			ColorName: theme.ColorNameForeground,
			SizeName:  theme.SizeNameHeadingText, // 20 左右
		},
	})
	dt.Segments[0].(*widget.TextSegment).Style.Alignment = fyne.TextAlignCenter // Forced alignment

	// 欢迎语
	t1 := widget.NewRichText(&widget.TextSegment{
		Text: "❤️ 您好，VDD 欢迎您！❤️",
		Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			ColorName: theme.ColorNameForeground,
			SizeName:  theme.SizeNameText,
		},
	})

	vSpacer := canvas.NewRectangle(color.Transparent)
	vSpacer.SetMinSize(fyne.NewSize(0, 14))

	welcomeStack := container.NewVBox(
		vSpacer,
		container.NewCenter(t1),
		container.NewCenter(dt),
	)
	// 欢迎区域
	welcomeArea := container.NewCenter(welcomeStack)

	// 随机欢迎语 (显示在中间引用位置，后续被网络金句替换)
	randSource := rand.NewSource(time.Now().UnixNano())
	myrand := rand.New(randSource)
	welcomeMsg := consts.WelcomeMessages[myrand.Intn(len(consts.WelcomeMessages))]

	// 每日一句/欢迎语标签
	// 初始显示随机欢迎语
	quoteLabel := widget.NewLabel(welcomeMsg)
	quoteLabel.Alignment = fyne.TextAlignCenter
	quoteLabel.TextStyle = fyne.TextStyle{Italic: true}
	quoteLabel.Wrapping = fyne.TextWrapWord

	// 小贴士 (显示在最底部)
	randomTip := consts.LocalTips[rand.Intn(len(consts.LocalTips))]
	tipsLabel := widget.NewLabel(randomTip)
	tipsLabel.Alignment = fyne.TextAlignCenter
	tipsLabel.TextStyle = fyne.TextStyle{Italic: false} // 小贴士用正常字体
	tipsLabel.Wrapping = fyne.TextWrapWord
	// 可以设置颜色稍微淡一点? 暂时保持默认

	// 更新引用 (网络获取)
	go func() {
		quote, err := utils.FetchDailyQuote(5 * time.Second)
		if err == nil && quote != "" {
			// 稍微延迟一下，让用户有机会看到欢迎语
			time.Sleep(6 * time.Second)
			fyne.Do(func() {
				quoteLabel.SetText(fmt.Sprintf("“%s”", quote))
			})
		}
	}()

	separatorRect := canvas.NewRectangle(theme.Color(theme.ColorNameDisabled))
	separatorRect.SetMinSize(fyne.NewSize(200, 1))
	separator := container.NewCenter(separatorRect)

	welcomeContent := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(logoImg),
		welcomeArea,
		separator,
		layout.NewSpacer(),
		quoteLabel,
		layout.NewSpacer(),
		tipsLabel,
	)
	v.welcomeContainer = container.NewPadded(welcomeContent)
	v.welcomeContainer.Show()
}

// maxSizeLayout 限制子组件最大尺寸的布局
type maxSizeLayout struct {
	maxWidth  float32
	maxHeight float32
}

func (m *maxSizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}

	// 限制尺寸不超过 maxWidth 和 maxHeight
	width := fyne.Min(size.Width, m.maxWidth)
	height := fyne.Min(size.Height, m.maxHeight)

	// 居中放置
	for _, obj := range objects {
		obj.Resize(fyne.NewSize(width, height))
		obj.Move(fyne.NewPos(0, 0))
	}
}

func (m *maxSizeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(m.maxWidth, m.maxHeight)
}

func (v *HomeView) createTableCell() fyne.CanvasObject {
	label := widget.NewLabel("")
	label.Truncation = fyne.TextTruncateEllipsis

	// 推荐图标 (左侧，使用 Canvas Text 以便和右侧占位符精确对齐)
	recText := canvas.NewText("⭐️", theme.Color(theme.ColorNameForeground))
	recText.TextStyle = fyne.TextStyle{Bold: true}
	recText.Hide()

	btn := widgets.NewButtonWithTooltip("", icons.ThemedDownloadIcon, nil, "下载")

	// 使用 GridWrap 来强制按钮大小
	btnResizer := container.NewGridWrap(fyne.NewSize(32, 32), btn)

	// 占位标签 (右侧，平衡左侧的推荐图标，确保按钮居中)
	// 使用由 "⭐️" 组成的 Canvas Text，确保宽度与左侧完全一致。
	// 由于 Emoji 可能忽略透明色，我们使用 Masking 策略：在它上面覆盖一个背景色的矩形
	dummyText := canvas.NewText("⭐️", theme.ForegroundColor())
	dummyText.TextStyle = fyne.TextStyle{Bold: true}
	// dummyText 必须由于 Stack 布局计算大小而显示，但被 maskRect 遮挡

	maskRect := canvas.NewRectangle(theme.BackgroundColor())
	dummyContainer := container.NewStack(dummyText, maskRect)
	dummyContainer.Hide() // 初始隐藏

	// 将推荐图标、按钮、占位容器放在一起
	actionHBox := container.NewHBox(recText, btnResizer, dummyContainer)
	btnWrapper := container.NewCenter(actionHBox)
	btnWrapper.Hide()

	bg := canvas.NewRectangle(color.Transparent)

	// Badge for resolution
	badge := widgets.NewBadge("4K", color.Transparent)
	badge.Container.Hide()

	// 使用 MaxSize 容器限制徽章尺寸，防止被 Border 布局拉伸
	// 然后用 VBox with spacers 实现垂直居中
	// maxSize 应该略大于 badge 的实际尺寸以留出一些空间
	badgeMaxSizeBox := container.New(&maxSizeLayout{maxWidth: 26, maxHeight: 14}, badge.Container)
	badgeWrapper := container.NewVBox(
		layout.NewSpacer(),
		badgeMaxSizeBox,
		layout.NewSpacer(),
	)

	// Label with badge
	// 使用 Border 布局：徽章在左（Left），标签在中间（Center）填充剩余空间
	contentBox := container.NewBorder(nil, nil, badgeWrapper, nil, label)

	return container.NewStack(bg, contentBox, btnWrapper)
}

func (v *HomeView) updateTableCell(id widget.TableCellID, o fyne.CanvasObject) {
	c, ok := o.(*fyne.Container)
	if !ok {
		return
	}

	// 安全检查子组件数量
	if len(c.Objects) < 3 {
		return
	}

	bg, ok := c.Objects[0].(*canvas.Rectangle)
	if !ok {
		return
	}

	// 索引 1 (Border 布局容器)
	contentBox, ok := c.Objects[1].(*fyne.Container)
	if !ok {
		return
	}

	// Border 布局存储非 nil 对象的顺序: center, left, right, top, bottom
	// 我们的布局: NewBorder(nil, nil, badgeWrapper, nil, label)
	// 所以 Objects[0] = label (center), Objects[1] = badgeWrapper (left)
	if len(contentBox.Objects) < 2 {
		return
	}

	// 标签 (Border 中心) -> 索引 0
	label, ok := contentBox.Objects[0].(*widget.Label)
	if !ok {
		return
	}

	// 徽章包装器 (Border 左侧) -> 索引 1
	badgeWrapper, ok := contentBox.Objects[1].(*fyne.Container)
	if !ok {
		return
	}

	// 应用列对齐方式
	label.Alignment = v.table.GetColumnAlignment(id.Col)

	// 层次结构: Stack -> Center (btnWrapper) -> HBox (actionHBox) -> [Label(recommendLabel), GridWrap(btnResizer)]
	btnWrapper, ok := c.Objects[2].(*fyne.Container)
	if !ok {
		return
	}
	if len(btnWrapper.Objects) < 1 {
		return
	}
	actionHBox, ok := btnWrapper.Objects[0].(*fyne.Container)
	if !ok || len(actionHBox.Objects) < 2 {
		return
	}

	recText, ok := actionHBox.Objects[0].(*canvas.Text)
	if !ok {
		return
	}
	// 更新颜色以适应主题
	recText.Color = theme.ForegroundColor()
	recText.Refresh()

	btnResizer, ok := actionHBox.Objects[1].(*fyne.Container)
	if !ok {
		return
	}
	if len(btnResizer.Objects) < 1 {
		return
	}
	btn, ok := btnResizer.Objects[0].(*widgets.ButtonWithTooltip)
	if !ok {
		return
	}

	dummyContainer, ok := actionHBox.Objects[2].(*fyne.Container)
	if !ok {
		return
	}
	// 更新遮罩颜色以适应主题
	if len(dummyContainer.Objects) > 1 {
		if mask, ok := dummyContainer.Objects[1].(*canvas.Rectangle); ok {
			mask.FillColor = theme.BackgroundColor()
			mask.Refresh()
		}
	}

	// 背景色设置为主题背景色，覆盖默认的 hover 效果
	bg.FillColor = theme.Color(theme.ColorNameBackground)
	bg.Refresh()

	label.Show()
	btnWrapper.Hide()
	badgeWrapper.Hide()
	label.TextStyle = fyne.TextStyle{} // 重置样式

	// 1. 标题行
	if id.Row == 0 {
		v.setupTitleRow(label, id)
		return
	}

	// 2. 数据行
	if id.Row-1 >= len(v.formatData) {
		label.SetText("")
		return
	}

	format := v.formatData[id.Row-1]
	// 如果是推荐格式，加粗
	if format.Recommended {
		label.TextStyle = fyne.TextStyle{Bold: true}
	}

	switch id.Col {
	case 0:
		label.SetText(format.FormatID)
	case 1:
		label.SetText(format.Extension)
	case 2:
		label.SetText(format.Resolution)
		v.setupResolutionBadge(contentBox, format)
	case 3:
		codecs := format.VCodec
		if format.ACodec != "" && format.ACodec != "none" {
			if codecs != "" && codecs != "none" {
				codecs += "+" + format.ACodec
			} else {
				codecs = format.ACodec
			}
		}
		label.SetText(codecs)
	case 4:
		label.SetText(utils.FormatBytes(format.FileSize))
	case 5:
		if format.FPS > 0 {
			label.SetText(fmt.Sprintf("%.0ffps", format.FPS))
		} else if format.ABR > 0 {
			label.SetText(fmt.Sprintf("%.0fk", format.ABR))
		} else {
			label.SetText("-")
		}
	case 6:
		v.setupActions(label, btnWrapper, recText, btn, dummyContainer, format) // Updated signature
	}
}

func (v *HomeView) setupTitleRow(label *widget.Label, id widget.TableCellID) {
	label.TextStyle = fyne.TextStyle{Bold: true}
	switch id.Col {
	case 0:
		label.SetText("ID")
	case 1:
		label.SetText("格式")
	case 2:
		label.SetText("分辨率")
	case 3:
		label.SetText("编码")
	case 4:
		label.SetText("大小")
	case 5:
		label.SetText("FPS/率")
	case 6:
		label.SetText("操作")
	}
}

func (v *HomeView) setupResolutionBadge(contentBox *fyne.Container, format parser.Format) {
	// 徽章包装器 (Border 左侧) -> 索引 1
	badgeWrapper, ok := contentBox.Objects[1].(*fyne.Container)
	if !ok {
		return
	}
	badgeWrapper.Hide() // 默认隐藏徽章包装器
	// 徽章检查 - 仅在需要时访问徽章内部
	text, col := widgets.GetResolutionColor(format.ResolutionDimension())
	if text == "8K" || text == "4K" || text == "2K" {
		// badgeWrapper 现在是 VBox with [Spacer, maxSizeBox, Spacer]
		// maxSizeBox 包含 badge.Container (使用 BadgeLayout)
		if len(badgeWrapper.Objects) >= 3 {
			// Objects[1] 是 maxSizeBox 容器
			if maxSizeBox, ok := badgeWrapper.Objects[1].(*fyne.Container); ok {
				if len(maxSizeBox.Objects) >= 1 {
					// Objects[0] 是 badge.Container (使用 BadgeLayout，包含 bg 和 text)
					if badgeContainer, ok := maxSizeBox.Objects[0].(*fyne.Container); ok {
						if len(badgeContainer.Objects) >= 2 {
							// BadgeLayout: Objects[0] 是 bg (Rectangle)
							badgeBg, ok1 := badgeContainer.Objects[0].(*canvas.Rectangle)
							// BadgeLayout: Objects[1] 是 text (Text) - 直接是 Text，不是 Center 容器
							badgeText, ok2 := badgeContainer.Objects[1].(*canvas.Text)
							if ok1 && ok2 {
								badgeBg.FillColor = col
								badgeBg.Refresh()
								badgeText.Text = text
								badgeText.Refresh()
								// 更新背景尺寸以适应新文本
								textSize := badgeText.MinSize()
								badgeBg.SetMinSize(fyne.NewSize(textSize.Width+12, textSize.Height+6))
								badgeContainer.Refresh()
							}
						}
					}
				}
			}
		}
		badgeWrapper.Show()
		// 关键: 也显示 badge.Container 内部, 它在创建时被隐藏
		if len(badgeWrapper.Objects) >= 3 {
			if maxSizeBox, ok := badgeWrapper.Objects[1].(*fyne.Container); ok {
				if len(maxSizeBox.Objects) >= 1 {
					maxSizeBox.Objects[0].Show() // 显示 badge.Container
				}
			}
		}
	}
}

func (v *HomeView) setupActions(label *widget.Label, btnWrapper *fyne.Container, recText *canvas.Text, btn *widgets.ButtonWithTooltip, dummyContainer *fyne.Container, format parser.Format) {
	// 操作列: 显示按钮
	label.Hide()
	btnWrapper.Show()

	// 重置按钮文本为空，因为我们使用图标
	btn.SetText("")

	// 使用短边检测
	if format.Limited {
		btn.SetText("🔒") // 锁定状态仍显示锁图标文本
		btn.Icon = nil
		btn.Importance = widget.LowImportance
		recText.Hide()        // 锁定时不显示推荐星号
		dummyContainer.Hide() // 隐藏占位符
		btn.SetOnTapped(func() {
			label := widget.NewLabel("该格式当前不可用，请选择其他格式。")
			label.Alignment = fyne.TextAlignCenter
			content := container.NewVBox(label)
			widgets.ShowDialog("提示", content, v.window)
		})
	} else {
		// 恢复下载图标（必须显式设置，因为可能从锁定状态复用）
		btn.Icon = icons.ThemedDownloadIcon
		btn.Importance = widget.LowImportance

		if format.Recommended {
			recText.Show()
			dummyContainer.Show() // 显示占位符以保持平衡
		} else {
			recText.Hide()
			dummyContainer.Hide()
		}

		// 捕获变量用于闭包
		f := format
		btn.SetOnTapped(func() {
			v.selectedFormat = &f
			v.onDownloadClick()
		})
	}
	// 必须刷新以应用样式更改
	btn.Refresh()
}

func (v *HomeView) updateVideoInfo(info *parser.VideoInfo) {
	// 1. 缩略图 (左侧)
	// 使用 Stack 容器，默认显示图标，加载完成后显示缩略图
	thumbImg := canvas.NewImageFromResource(theme.FileVideoIcon())
	thumbImg.FillMode = canvas.ImageFillContain
	thumbImg.SetMinSize(fyne.NewSize(160, 90))

	// 2. 信息 (右侧)
	titleLabel := widget.NewLabelWithStyle(info.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Wrapping = fyne.TextWrapBreak

	uploaderDuration := fmt.Sprintf("UP主: %s  •  时长: %s",
		info.Uploader,
		utils.FormatDuration(info.Duration),
	)
	metaLabel := widget.NewLabel(uploaderDuration)

	infoVBox := container.NewVBox(titleLabel, metaLabel)

	// 布局: Border(Left=Thumb, Center=Info)
	content := container.NewBorder(nil, nil, thumbImg, nil, infoVBox)

	v.videoInfo.Objects = []fyne.CanvasObject{container.NewPadded(content)}
	v.videoInfo.Refresh()

	// 异步加载缩略图
	if info.Thumbnail != "" {
		logger.Debugf("[首页] 异步加载缩略图: %s", info.Thumbnail)
		go func() {
			helper.SharedThumbnailManager().LoadThumbnail(info.Thumbnail, func(res fyne.Resource) {
				fyne.Do(func() {
					thumbImg.Resource = res
					thumbImg.Refresh()
				})
			})
		}()
	}

	// 强制父布局更新以防止重叠
	if v.Container != nil {
		v.Container.Refresh()
	}
}

func (v *HomeView) onUrlChanged(s string) {
	if v.statusLabel == nil {
		return // UI 未完全初始化
	}
	text := strings.TrimSpace(s)
	if text == "" {
		v.statusLabel.SetText("空闲")
	} else if isSupportedURL(text) {
		v.statusLabel.SetText("就绪")
	} else {
		v.statusLabel.SetText("❌ 无效的视频链接")
	}
}

func isSupportedURL(u string) bool {
	return strings.HasPrefix(u, "http")
}

func (v *HomeView) onParse() {
	logger.Infof("[首页] 用户点击了解析...")

	if v.isParsing {
		return
	}
	url := strings.TrimSpace(v.urlEntry.Text)
	if url == "" {
		return
		widgets.ShowToast(v.window, "请输入视频链接", icons.ThemedDeleteCircleIcon)
		return
	}

	// 智能检测: 播放列表/频道/List参数
	// 如果返回 true，说明已由弹窗接管，直接返回
	if v.checkAmbiguousURL(url) {
		return
	}

	// 创建上下文用于取消 BEFORE goroutine
	ctx, cancel := context.WithCancel(context.Background())

	// 注入 Cookie 覆盖 (如果启用)
	if v.useCookieFile {
		// 获取当前配置中的路径
		path := config.Get().CookiesPath
		if path != "" {
			ctx = context.WithValue(ctx, consts.CtxKeyCookieFile, path)
		}
	}

	v.parseCancelFunc = cancel
	v.isParsing = true

	v.cancelBtn.Enable()
	v.downloadBtn.Disable()

	v.statusLabel.SetText("解析中...")
	v.statusContainer.Objects = []fyne.CanvasObject{v.statusLabel}
	v.statusContainer.Refresh()

	v.formatData = nil
	v.table.Refresh()
	v.resultArea.Hide()
	v.welcomeContainer.Show()
	v.currentVideoInfo = nil
	v.videoInfo.Objects = nil
	v.videoInfo.Refresh()

	v.parseSpinner = widgets.NewSpinnerAnimator(v.statusLabel, "🚀 解析中, 请耐心等待...")
	v.parseSpinner.Start()

	go func() {
		defer func() {
			v.isParsing = false
			v.isCancelParsing = false // 确保状态被重置
			// 不要立即 nil parseCancelFunc 这里如果需要支持延迟取消？
			// 实际上通常我们 nil 它当完成时。
			v.parseCancelFunc = nil
		}()

		// 1. 检查 parsing_results 表中的缓存
		logger.Debugf("[解析] 检查 parsing_results 表中的缓存...")
		pResult, err := parser.DAO.GetParseResult(url)
		if err == nil && pResult != nil {
			var cachedInfo parser.VideoInfo
			if err := json.Unmarshal([]byte(pResult.MetaJSON), &cachedInfo); err == nil {
				v.parseSpinner.Stop()
				// 获取权限信息
				authInfo := auth.GetAutherization()
				maxQuality := int(authInfo.UserMaxQuality())

				recommended := v.recommender.Recommend(cachedInfo.Formats, maxQuality)
				v.computeResolutionRanks(recommended)

				v.computeResolutionRanks(recommended)

				v.Refresh(&cachedInfo, recommended)
				return
			}
		}

		fyne.Do(func() {
			// 如果不在缓存中，则继续实际解析
			v.statusLabel.SetText("🚀 正在解析视频...")
			v.cancelBtn.Enable()
		})

		// 使用外部上下文 ctx 从闭包捕获

		info, err := v.parser.ParseVideoWithContext(ctx, url)
		v.parseSpinner.Stop()

		if err != nil {
			fyne.Do(func() {
				if v.parseSpinner != nil {
					v.parseSpinner.Stop()
				}
				if ctx.Err() == context.Canceled {
					v.statusLabel.SetText("🛑 解析被中断")
				} else {
					logger.Errorf("[首页] 解析视频失败: %v", err.Error())
					v.statusLabel.SetText(fmt.Sprintf("❌ %v", err.Error()))
				}
			})
			return
		}

		// 保存解析结果到 parsing_results 表
		if err := parser.DAO.SaveParseResult(url, info); err != nil {
			logger.Errorf("[首页] 保存解析结果失败: %v", err.Error())
		}

		// 推荐格式并评分
		authInfo := auth.GetAutherization()
		maxQuality := int(authInfo.UserMaxQuality())
		recommended := v.recommender.Recommend(info.Formats, maxQuality)
		v.computeResolutionRanks(recommended)

		v.Refresh(info, recommended)
	}()
}

func (v *HomeView) Refresh(info *parser.VideoInfo, recommended []parser.Format) {
	fyne.Do(func() {
		v.currentVideoInfo = info
		v.updateVideoInfo(info)
		v.formatData = recommended
		v.table.Refresh()
		v.statusLabel.SetText(fmt.Sprintf("✅ 找到 %d 个格式", len(recommended)))
		v.cancelBtn.Disable()
		v.downloadBtn.Enable()
		v.welcomeContainer.Hide()
		v.resultArea.Show()

		// 启用清除按钮
		v.enableClearButton()
	})
}

// OnCancelParse 取消解析或清除结果
func (v *HomeView) OnCancelParse() {
	if v.isParsing {
		// 取消模式
		if v.parseCancelFunc != nil && !v.isCancelParsing {
			v.cancelBtn.Disable()
			if v.parseSpinner != nil {
				v.parseSpinner.Stop()
			}
			v.parseSpinner = widgets.NewSpinnerAnimator(v.statusLabel, "🚀 正在取消, 请稍后...")
			v.parseSpinner.Start()

			logger.Infof("[首页] 用户取消解析...")
			v.isCancelParsing = true

			v.statusLabel.Refresh()
			v.parseCancelFunc()
		}
	} else {
		// 清除模式
		v.resetToWelcome()
	}
}

func (v *HomeView) selectFormat() {
	if v.selectedFormat == nil {
		if len(v.formatData) > 0 {
			for _, f := range v.formatData {
				if f.Recommended {
					v.selectedFormat = &f
					break
				}
			}
			if v.selectedFormat == nil {
				v.selectedFormat = &v.formatData[0]
			}
		} else {
			return
		}
	}
}

func (v *HomeView) onDownloadClick() {
	logger.Infof("[首页] 用户点击了下载...")

	canDownload, err := auth.CanDownload()
	if err != nil {
		widgets.ShowInformation("提示", err.Error(), v.window)
		return
	}
	if !canDownload {
		return
	}

	// 选择格式
	v.selectFormat()
	if v.selectedFormat == nil {
		logger.Errorf("[首页] 没有可用的格式")
		return
	}

	// 最终权限检查 (防止绕过 UI)
	authInfo := auth.GetAutherization()
	maxQuality := int(authInfo.UserMaxQuality())

	// 使用短边检测
	dim := v.selectedFormat.ResolutionDimension()
	if maxQuality > 0 && dim > maxQuality {
		content := container.NewVBox(
			widget.NewLabel("当前配置不允许下载该画质的视频。"),
			widget.NewLabel("请在设置中调整画质策略后重试。"),
		)
		dialog.ShowCustom("权限不足", "关闭", content, v.window)
		return
	}

	logger.Debugf("[首页] 用户选择了格式: %s", v.selectedFormat.FormatID)

	// fix: 必须以视频的 web_url 作为任务的 URL，而不是输入的，否则可能造成不一致
	url := v.currentVideoInfo.WebpageURL

	logger.Debugf("[首页] 正在检查是否已有相同URL的任务...")

	// 检查是否已有相同URL的任务
	existingTask, err := tasks.DAO.GetTaskByURL(url)
	if err == nil && existingTask != nil {
		logger.Infof("[首页] 找到相同URL的任务: %s", existingTask.ID)

		v.showTaskExistsDialog()
		return
	}

	// 创建任务
	logger.Infof("[首页] 正在创建任务...")
	isMergeToMp4 := v.downloader.IsMergeToMP4()

	// 获取 CookieFile (如果启用)
	cookieFile := ""
	if v.useCookieFile {
		cookieFile = config.Get().CookiesPath
	}

	task, err := tasks.DAO.CreateTaskFromParser(url, v.currentVideoInfo, v.selectedFormat, v.getDownloadTemplatePath(), isMergeToMp4, cookieFile)
	if err != nil {
		logger.Errorf("[首页] 创建任务失败: %v", err.Error())
		return
	}
	v.currentTaskID = task.ID

	v.currentTaskID = task.ID

	go v.downloader.Schedule(task, authInfo)

	v.resetUI()
	v.showTaskCreatedDialog()
}

func (v *HomeView) getDownloadTemplatePath() string {
	downloadDir := config.Get().DownloadDir
	if downloadDir == "" {
		downloadDir = utils.GetDownloadDir()
	}
	template := config.Get().GetFilenameTemplate()
	return filepath.Join(downloadDir, template)
}

func (v *HomeView) showTaskExistsDialog() {
	// 先创建对话框变量，以便在按钮回调中使用
	var d dialog.Dialog

	// 创建选项按钮
	reuseBtn := widget.NewButtonWithIcon("", icons.ThemedSearchIcon, func() {
		if d != nil {
			d.Hide()
		}
		// 跳转到任务列表
		if v.OnSwitchToTasks != nil {
			v.OnSwitchToTasks()
		}
	})

	cancelBtn := widget.NewButtonWithIcon("", icons.ThemedCancelIcon, func() {
		if d != nil {
			d.Hide()
		}
	})

	// 创建对话框内容
	content := container.NewVBox(
		widget.NewLabelWithStyle("该视频您已经下载过了哦！", fyne.TextAlignCenter, fyne.TextStyle{}),
		container.NewHBox(
			layout.NewSpacer(),
			reuseBtn,
			cancelBtn,
			layout.NewSpacer(),
		),
	)

	// 创建并显示对话框
	d = dialog.NewCustom("检测到重复任务", "", content, v.window)
	if customDlg, ok := d.(*dialog.CustomDialog); ok {
		customDlg.SetButtons([]fyne.CanvasObject{})
	}
	d.Show()
}

func (v *HomeView) showTaskCreatedDialog() {
	widgets.ShowConfirmDialog("任务创建成功", "任务创建成功，请在任务列表中查看", func() {
		if v.OnSwitchToTasks != nil {
			v.OnSwitchToTasks()
		}
	}, v.window)
}

func (v *HomeView) resetUI() {
	// 复用 resetToWelcome，但加上清空 URL (如果是任务创建后)
	v.resetToWelcome()
	v.urlEntry.SetText("")
	v.selectedRow = -1
	v.currentTaskID = ""
	v.statusLabel.SetText("空闲")
}

func (v *HomeView) computeResolutionRanks(formats []parser.Format) {
	// 收集所有不同的分辨率高度
	heights := make(map[int]bool)
	for _, f := range formats {
		if f.Height > 0 {
			heights[f.Height] = true
		}
	}

	// 将高度转换为切片并排序（从高到低）
	heightList := make([]int, 0, len(heights))
	for h := range heights {
		heightList = append(heightList, h)
	}

	// 按高度降序排序
	sort.Slice(heightList, func(i, j int) bool {
		return heightList[i] > heightList[j]
	})

	// 分配排名：最高分辨率排名为1，依次递增
	v.resolutionRanks = make(map[int]int, len(heightList))
	for rank, height := range heightList {
		v.resolutionRanks[height] = rank + 1
	}
}

// SetURL 设置输入框内容
func (v *HomeView) SetURL(url string) {
	if v.urlEntry != nil {
		v.urlEntry.SetText(url)
	}
}

// resetToWelcome 重置为欢迎页
func (v *HomeView) resetToWelcome() {
	v.formatData = nil
	v.currentVideoInfo = nil
	v.videoInfo.Objects = nil
	v.videoInfo.Refresh()

	// v.urlEntry.SetText("") // 保持 URL 不变，方便修改

	// 切换视图
	v.resultArea.Hide()
	v.welcomeContainer.Show()

	// 重置按钮
	v.downloadBtn.Disable()

	// 重置取消按钮为禁用状态（等待下一次解析）
	v.cancelBtn.SetIcon(icons.ThemedDeleteCircleIcon)
	v.cancelBtn.Disable()
	v.cancelBtn.Refresh()

	v.statusLabel.SetText("就绪")
}

// 辅助方法：确保取消按钮切换为清除模式
func (v *HomeView) enableClearButton() {
	v.cancelBtn.SetIcon(icons.ThemedDeleteCircleIcon) // 复用删除圈图标
	v.cancelBtn.Enable()
	v.cancelBtn.Refresh()
}

// checkAmbiguousURL 检查是否为歧义URL (如播放列表)，并询问用户意图
// 返回 true 表示已拦截处理，false 表示继续常规解析
func (v *HomeView) checkAmbiguousURL(url string) bool {
	// 简单特征检测
	isPlaylist := strings.Contains(url, "list=") || strings.Contains(url, "/lists/") || strings.Contains(url, "collections")
	isChannel := strings.Contains(url, "/channel/") || strings.Contains(url, "/c/") || strings.Contains(url, "/user/") || strings.Contains(url, "@") || strings.Contains(url, "space.bilibili.com")

	// 如果既不是显式列表也不是频道特征，且不含 list 参数，暂不拦截
	if !isPlaylist && !isChannel {
		return false
	}

	// 弹窗询问
	content := container.NewVBox(
		widget.NewLabelWithStyle("检测到该链接包含播放列表或频道信息", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("您希望如何处理？"),
	)

	d := dialog.NewCustom("操作选择", "取消", content, v.window)

	// 解析当前 (单视频)
	parseBtn := widget.NewButtonWithIcon("仅解析当前视频", theme.SearchIcon(), func() {
		d.Hide()
		// 继续解析流程 -> 但我们需要重新触发 parsing，且设置 flag 跳过检查
		// 或者 better: 我们只返回 false?
		// 不行，因为 Dialog 是非阻塞的 (async)。
		// 所以必须在这里触发解析。
		// 但 onParse 中有 checkAmbiguousURL 调用。
		// 我们需要一个 skipFlag 吗？
		// 或者拆分 onParse 逻辑。

		// 简单方案:
		// 这里是 checkAmbiguousURL，如果返回 true，onParse 就 return 了。
		// 用户点击按钮后，我们再次调用一个 "internalParse" 方法。
		v.startParseInternal(url)
	})

	// 添加订阅
	subBtn := widget.NewButtonWithIcon("添加为订阅 (自动更新)", icons.ThemedSubscriptionsIcon, func() {
		d.Hide()
		// 执行添加订阅逻辑
		go func() {
			_, err := v.subManager.AddSubscription(url)
			if err != nil {
				dialog.ShowError(err, v.window)
			} else {
				widgets.ShowToast(v.window, "订阅添加成功", icons.ThemedOkIcon)
				// 跳转到订阅页
				if v.OnSwitchToSubscriptions != nil {
					v.OnSwitchToSubscriptions()
				}
			}
		}()
	})
	subBtn.Importance = widget.HighImportance

	// 布局按钮
	// Fyne dialog Custom can have custom buttons?
	// dialog.NewCustom returns a Dialog interface.
	// Use SetButtons helper if available? No, standard dialog doesn't expose easy button setter.
	// We can use container for buttons.

	btnBox := container.NewHBox(layout.NewSpacer(), parseBtn, subBtn, layout.NewSpacer())
	content.Add(widget.NewSeparator())
	content.Add(btnBox)

	// d.SetContent(content) // Removed: content is already set
	d.Show()

	return true
}

// startParseInternal 内部解析流程 (跳过检查)
func (v *HomeView) startParseInternal(url string) {
	// 复制 onParse 的后续逻辑
	// 创建上下文用于取消 BEFORE goroutine
	ctx, cancel := context.WithCancel(context.Background())

	// 注入 Cookie 覆盖 (如果启用)
	if v.useCookieFile {
		// 获取当前配置中的路径
		path := config.Get().CookiesPath
		if path != "" {
			ctx = context.WithValue(ctx, consts.CtxKeyCookieFile, path)
		}
	}

	v.parseCancelFunc = cancel
	v.isParsing = true

	v.cancelBtn.Enable()
	v.downloadBtn.Disable()

	v.statusLabel.SetText("解析中...")
	v.statusContainer.Objects = []fyne.CanvasObject{v.statusLabel}
	v.statusContainer.Refresh()

	v.formatData = nil
	v.table.Refresh()
	v.resultArea.Hide()
	v.welcomeContainer.Show()
	v.currentVideoInfo = nil
	v.videoInfo.Objects = nil
	v.videoInfo.Refresh()

	v.parseSpinner = widgets.NewSpinnerAnimator(v.statusLabel, "🚀 解析中, 请耐心等待...")
	v.parseSpinner.Start()

	go func() {
		defer func() {
			v.isParsing = false
			v.isCancelParsing = false
			v.parseCancelFunc = nil
		}()

		// 1. 检查 parsing_results 表中的缓存
		logger.Debugf("[解析] 检查 parsing_results 表中的缓存...")
		pResult, err := parser.DAO.GetParseResult(url)
		if err == nil && pResult != nil {
			var cachedInfo parser.VideoInfo
			if err := json.Unmarshal([]byte(pResult.MetaJSON), &cachedInfo); err == nil {
				v.parseSpinner.Stop()
				// 获取权限信息
				authInfo := auth.GetAutherization()
				maxQuality := int(authInfo.UserMaxQuality())

				recommended := v.recommender.Recommend(cachedInfo.Formats, maxQuality)
				v.computeResolutionRanks(recommended)

				v.Refresh(&cachedInfo, recommended)
				return
			}
		}

		fyne.Do(func() {
			v.statusLabel.SetText("🚀 正在解析视频...")
			v.cancelBtn.Enable()
		})

		info, err := v.parser.ParseVideoWithContext(ctx, url)
		v.parseSpinner.Stop()

		if err != nil {
			fyne.Do(func() {
				if v.parseSpinner != nil {
					v.parseSpinner.Stop()
				}
				if ctx.Err() == context.Canceled {
					v.statusLabel.SetText("🛑 解析被中断")
				} else {
					logger.Errorf("[首页] 解析视频失败: %v", err.Error())
					v.statusLabel.SetText(fmt.Sprintf("❌ %v", err.Error()))
				}
			})
			return
		}

		if err := parser.DAO.SaveParseResult(url, info); err != nil {
			logger.Errorf("[首页] 保存解析结果失败: %v", err.Error())
		}

		authInfo := auth.GetAutherization()
		maxQuality := int(authInfo.UserMaxQuality())
		recommended := v.recommender.Recommend(info.Formats, maxQuality)
		v.computeResolutionRanks(recommended)

		v.Refresh(info, recommended)
	}()
}
