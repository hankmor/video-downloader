package windows

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/hankmor/vdd/core/download"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/ui/widgets"
)

// ViewType 定义视图类型
type ViewType int

const (
	ViewHome ViewType = iota
	ViewTasks
	ViewSubscriptions
	ViewHistory
)

// Toolbar 控制顶部导航栏
type Toolbar struct {
	Container *fyne.Container

	homeBtn  *widgets.ButtonWithTooltip
	tasksBtn *widgets.ButtonWithTooltip
	subBtn   *widgets.ButtonWithTooltip
	subBadge *widgets.NotificationBadge // 订阅按钮的角标
	// historyBtn *widgets.ButtonWithTooltip

	dynamicContainer *fyne.Container // 动态操作区域

	currentView ViewType
	onSwitch    func(ViewType)
}

// NewToolbar 创建新的导航栏
func NewToolbar(app fyne.App, parent fyne.Window, onSwitch func(ViewType), clipboardMonitor *ClipboardMonitor, downloader *download.Downloader, extraItems ...fyne.CanvasObject) *Toolbar {
	t := &Toolbar{
		onSwitch:         onSwitch,
		currentView:      ViewHome,
		dynamicContainer: container.NewHBox(), // 初始化动态容器
	}

	// 1. 视图切换按钮 (使用带Tooltip的图标按钮，无文字)
	t.homeBtn = widgets.NewButtonWithTooltip("", icons.ThemedHomeIcon, func() {
		t.highlight(ViewHome)
		if t.onSwitch != nil {
			t.onSwitch(ViewHome)
		}
	}, "首页")
	t.homeBtn.Importance = widget.HighImportance // 默认选中高亮

	t.tasksBtn = widgets.NewButtonWithTooltip("", icons.ThemedTaskIcon, func() {
		t.highlight(ViewTasks)
		if t.onSwitch != nil {
			t.onSwitch(ViewTasks)
		}
	}, "任务列表")
	t.tasksBtn.Importance = widget.LowImportance

	t.subBtn = widgets.NewButtonWithTooltip("", icons.ThemedSubscriptionsIcon, func() {
		t.highlight(ViewSubscriptions)
		if t.onSwitch != nil {
			t.onSwitch(ViewSubscriptions)
		}
	}, "订阅管理")
	t.subBtn.Importance = widget.LowImportance
	// 创建订阅按钮的角标
	t.subBadge = widgets.NewNotificationBadge()
	t.CheckSubscriptionStatus()

	// 2. 辅助工具按钮
	settingsBtn := widgets.NewButtonWithTooltip("", icons.ThemedSettingIcon, func() {
		ShowSettingsWindow(app, parent, clipboardMonitor, downloader)
	}, "软件设置")

	helpBtn := widgets.NewButtonWithTooltip("", icons.ThemedHelpIcon, func() {
		// 简单的帮助弹窗
		widgets.ShowInformation("使用帮助", "访问官网查看帮助文档: https://vdd.hankmo.com", parent, true)
	}, "使用帮助")

	aboutBtn := widgets.NewButtonWithTooltip("", icons.ThemedAboutIcon, func() {
		ShowAboutWindow(app, parent)
	}, "软件信息")

	// 3. 布局组装
	// 将订阅按钮包装在角标容器中
	subBtnWithBadge := widgets.NewBadgeContainer(t.subBtn, t.subBadge)

	items := []fyne.CanvasObject{
		t.homeBtn,
		t.tasksBtn,
		subBtnWithBadge, // 使用带角标的容器
	}

	// 插入额外工具
	if len(extraItems) > 0 {
		items = append(items, extraItems...)
	}

	// 原有工具栏按钮保持在左侧
	items = append(items, settingsBtn, helpBtn, aboutBtn)

	// 添加 Spacer 将动态区域推到最右侧
	items = append(items, layout.NewSpacer())

	// 动态操作区域（显示在最右侧）
	items = append(items, t.dynamicContainer)

	// 使用单一的 Toolbar 容器 (水平布局)
	t.Container = container.NewPadded(container.NewHBox(items...))

	return t
}

// highlight 更新按钮高亮状态
func (t *Toolbar) highlight(view ViewType) {
	t.currentView = view

	// 重置所有
	t.homeBtn.Importance = widget.LowImportance
	t.tasksBtn.Importance = widget.LowImportance
	t.subBtn.Importance = widget.LowImportance
	// t.historyBtn.Importance = widget.LowImportance

	// 高亮当前
	switch view {
	case ViewHome:
		t.homeBtn.Importance = widget.HighImportance
	case ViewTasks:
		t.tasksBtn.Importance = widget.HighImportance
	case ViewSubscriptions:
		t.subBtn.Importance = widget.HighImportance
		// case ViewHistory:
		// t.historyBtn.Importance = widget.HighImportance
	}

	t.Container.Refresh()
}

// SwitchTo 外部触发切换 (如自动跳到任务页)
func (t *Toolbar) SwitchTo(view ViewType) {
	t.highlight(view)
}

// SetActions 设置动态操作按钮
func (t *Toolbar) SetActions(objects []fyne.CanvasObject) {
	t.dynamicContainer.Objects = objects
	t.dynamicContainer.Refresh()
}

// ClearActions 清除动态操作按钮
func (t *Toolbar) ClearActions() {
	t.dynamicContainer.Objects = nil
	t.dynamicContainer.Refresh()
}

// UpdateSubscriptionBadge 更新订阅按钮的角标
func (t *Toolbar) UpdateSubscriptionBadge(count int) {
	if t.subBadge != nil {
		fyne.Do(func() {
			t.subBadge.SetCount(count)
		})
	}
}

// CheckSubscriptionStatus 主动检查并更新订阅状态 (用于初始化)
func (t *Toolbar) CheckSubscriptionStatus() {
	t.UpdateSubscriptionBadge(0)
}
