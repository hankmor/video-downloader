package windows

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/download"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/parser"
	"github.com/hankmor/vdd/core/recommender"
	"github.com/hankmor/vdd/core/subscription"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/ui/views"
	"github.com/hankmor/vdd/ui/widgets"
	"github.com/hankmor/vdd/utils"
)

// MainWindow 主窗口
type MainWindow struct {
	app    fyne.App
	window fyne.Window

	// Views
	toolbar     *Toolbar
	homeView    *views.HomeView
	tasksView   *views.TaskListView
	subView     *views.SubscriptionView
	historyView *views.HistoryView

	// Container
	contentContainer *fyne.Container

	// 剪贴板监听控制
	clipboardMonitor *ClipboardMonitor

	CleanUp func()
}

// NewMainWindow 创建主窗口
func NewMainWindow(app fyne.App) *MainWindow {
	w := &MainWindow{
		app:     app,
		window:  app.NewWindow("VDD - 全能视频下载工具"),
		CleanUp: func() {},
	}
	// 初始化剪贴板监听器
	w.clipboardMonitor = newClipboardMonitor(app.Clipboard(), w.clipboardMonitorCallback)

	// 将所有任务更新为取消，防止奔溃后状态不一致
	go tasks.DAO.CancelDownloadingAndQuened()

	// 构建界面
	w.buildUI()

	w.window.Resize(fyne.NewSize(1000, 700))
	w.window.CenterOnScreen()

	// 启动剪贴板监听
	if config.Get().ClipboardMonitor {
		w.clipboardMonitor.Start()
	}

	// 开源版统一标题
	w.updateLicenseTitle()

	// 启动后台清理任务 (清理超过 24 小时的未下载任务)
	// go func() {
	// 	if err := tasks.CleanupExpiredTasks(24 * time.Hour); err != nil {
	// 		logger.Errorf("[主窗] 清理过期任务失败: %v", err)
	// 	}
	// }()

	return w
}

// buildUI 构建用户界面
func (w *MainWindow) buildUI() {
	// ===== 初始化管理器 =====
	ytdlpPath := utils.GetYtDlpPath()
	parser := parser.New(ytdlpPath)
	downloader := download.New(ytdlpPath)
	recommender := recommender.New()

	// 初始化订阅管理器
	subManager := subscription.New(downloader)
	subManager.StartBackgroundPolling()

	// 启动时触发一次扫描（延迟3秒，避免影响启动速度）
	if config.Get().AutoScanSubscriptions {
		go func() {
			time.Sleep(1 * time.Second)
			logger.Info("[主窗] 开始初始订阅扫描...")
			subManager.GetScanManager().ScanAll()
		}()
	} else {
		logger.Info("[主窗] 自动扫描已禁用，跳过初始扫描")
	}

	// 注册下载状态变更通知 (系统通知)
	// macOS 需要应用签名才能正常工作，暂时只在 Windows/Linux 上启用
	downloader.SetNotifyStatusChanged(func(taskID string, status tasks.TaskStatus, errorMsg string) {
		if status == tasks.StatusCompleted {
			// macOS 跳过通知（需要签名才能正常工作）
			if runtime.GOOS == "darwin" {
				return
			}

			// 获取任务详情以显示标题
			task, err := tasks.DAO.GetTaskByID(taskID)
			title := "下载完成"
			if err == nil && task != nil {
				title = fmt.Sprintf("下载完成: %s", task.Title)
			}
			w.app.SendNotification(fyne.NewNotification(title, "任务已成功下载"))
		}
	})

	w.CleanUp = func() {
		logger.Infof("[主窗] 正在退出，清理资源...")

		// 1. 停止剪贴板监听
		w.clipboardMonitor.Stop()

		logger.Infof("[主窗] 取消解析...")
		w.homeView.OnCancelParse()

		logger.Infof("[主窗] 停止所有任务...")
		// 停止所有下载 (包含手动和订阅)
		downloader.StopAll()

		// 停止所有扫描
		subManager.GetScanManager().StopAll()

		// 兜底：更新数据库中的任务状态，防止下次启动状态不一致
		// 虽然 StopAll 会触发状态变更，但那是异步的，这里做一个同步的强制更新
		tasks.DAO.CancelDownloadingAndQuened()

		// 等待日志写入
		time.Sleep(200 * time.Millisecond)

		logger.Infof("[主窗] 清理完成, 欢迎下次使用!")
	}

	// 设置关闭拦截，确保 CleanUp 被调用
	w.window.SetCloseIntercept(func() {
		w.CleanUp()
		w.window.Close()
	})

	// ===== 初始化视图 =====
	// 首页
	// 首页
	w.homeView = views.NewHomeView(w.app, w.window, parser, downloader, recommender, subManager)
	// 设置切换到任务列表的回调
	w.homeView.OnSwitchToTasks = func() {
		w.switchView(ViewTasks)
		w.toolbar.SwitchTo(ViewTasks)
	}
	w.homeView.OnShowLicense = func() {
		ShowLicenseWindow(w.app)
	}
	w.homeView.OnSwitchToSubscriptions = func() {
		w.switchView(ViewSubscriptions)
		w.toolbar.SwitchTo(ViewSubscriptions)
	}

	// 任务view
	w.tasksView = views.NewTaskListView(w.app, w.window, downloader)

	// 订阅view
	w.subView = views.NewSubscriptionView(w.app, w.window, subManager)
	w.subView.OnShowActivation = func() {
		ShowLicenseWindow(w.app)
	}

	// 历史记录view
	// w.historyView = views.NewHistoryView(w.app, w.window)

	// ===== 初始化工具栏 =====
	// 创建 toolbar，传入 MainWindow 引用以便设置窗口可以更新剪贴板监听
	w.toolbar = NewToolbar(w.app, w.window, w.switchView, w.clipboardMonitor, downloader)

	// ===== 设置扫描管理器回调 =====
	scanManager := subManager.GetScanManager()

	// 进度回调：每个订阅扫描完成后更新角标
	scanManager.SetOnProgress(func(progress *subscription.ScanProgress) {
		// 获取总新视频数并更新工具栏角标
		totalNew := scanManager.GetTotalNewCount()
		w.toolbar.UpdateSubscriptionBadge(totalNew)
	})

	// 完成回调：所有扫描完成后
	scanManager.SetOnComplete(func(totalNew int) {
		w.toolbar.UpdateSubscriptionBadge(totalNew)
		if totalNew > 0 {
			// 显示 Toast 提示
			fyne.Do(func() {
				widgets.ShowToast(w.window, fmt.Sprintf("扫描完成，发现 %d 个新视频", totalNew), theme.ConfirmIcon())
			})
		}
	})

	// ===== 内容容器 =====
	w.contentContainer = container.NewStack(
		w.homeView.Container,
		w.tasksView.Container,
		w.subView.Container,
		// w.historyView.Container,
	)

	// 默认显示首页
	w.switchView(ViewHome)

	// ===== 主布局 =====
	mainContent := container.NewBorder(
		w.toolbar.Container, // 顶部工具栏
		nil, nil, nil,
		w.contentContainer, // 内容容器
	)

	// 添加 tooltip 层，使 tooltip 可以在不拦截点击的情况下显示
	w.window.SetContent(widgets.AddTooltipLayer(mainContent, w.window.Canvas()))
}

// switchView 切换视图
func (w *MainWindow) switchView(view ViewType) {
	// 隐藏所有
	w.homeView.Container.Hide()
	w.tasksView.Container.Hide()
	w.subView.Container.Hide()
	// w.historyView.Container.Hide()

	// 1. 先清除所有动态按钮
	w.toolbar.ClearActions()

	// 显示目标
	switch view {
	case ViewHome:
		w.homeView.Container.Show()
		w.toolbar.SetActions(w.homeView.CreateToolbarItems())
	case ViewTasks:
		w.tasksView.Container.Show()
		w.tasksView.Refresh()
		// 注入任务页面的操作按钮
		w.toolbar.SetActions(w.tasksView.CreateToolbarItems())
	case ViewSubscriptions:
		w.subView.Container.Show()
		w.subView.Refresh()
		w.toolbar.SetActions(w.subView.CreateToolbarItems())
		// case ViewHistory:
		// w.historyView.Container.Show()
		// w.historyView.Refresh()
	}

	// 这里可以添加 "Auto Refresh" 逻辑，例如切到 Tasks 时刷新列表
	// w.toolbar.SwitchTo(view) // Toolbar 自身会处理高亮，还是需要回调通知？
	// Toolbar 的回调会调用 switchView。
	// 如果是外部调用 switchView，我们需要反向更新 Toolbar 状态吗？
	// 是的，如果这是 onDownloadStart 触发的跳转。
	// Toolbar.SwitchTo 也是 Public 方法。

	// 为防止无限循环 (Toolbar.onSwitch -> switchView -> Toolbar.highlight ... -> onSwitch),
	// 我们约定 switchView 是核心执行者。
	// 但 Toolbar 目前设计的是内部 highlight 后调用 onSwitch。
	// 所以外部调用时，应该调用 toolbar.SwitchTo(view) -- Wait, toolbar.SwitchTo only highlights.
	// Let's ensure consistency.
	// For now, assume Toolbar handles its internal state well.

	w.contentContainer.Refresh()
}

// updateLicenseTitle 更新窗口标题
func (w *MainWindow) updateLicenseTitle() {
	w.window.SetTitle("VDD - 全能视频下载工具 (开源版)")
}

type ClipboardMonitor struct {
	clipboard  fyne.Clipboard
	ctx        context.Context
	cancel     context.CancelFunc
	errorCount int

	callbackFunc func(lastContent string, content string)

	running bool
}

func newClipboardMonitor(clipboard fyne.Clipboard, callbackFunc func(lastContent string, content string)) *ClipboardMonitor {
	return &ClipboardMonitor{
		clipboard:    clipboard,
		callbackFunc: callbackFunc,
	}
}

func (c *ClipboardMonitor) Start() {
	if c.running {
		return
	}

	logger.Info("[主窗] 剪贴板监听已启动")

	c.ctx, c.cancel = context.WithCancel(context.Background())

	go func() {
		// 延迟启动，确保窗口已完全初始化
		time.Sleep(500 * time.Millisecond)

		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("剪贴板监听发生错误，已停止监听: %v", r)
			}
		}()

		errorCount := 0
		maxErrors := 10
		var lastContent string

		c.running = true

		for {
			// 检查 context 是否被取消
			select {
			case <-c.ctx.Done():
				logger.Info("[主窗] 剪贴板监听已停止")
				c.running = false
				return
			default:
			}

			if errorCount >= maxErrors {
				logger.Errorf("[主窗] 剪贴板监听错误过多，已停止监听")
				c.running = false
				return
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						errorCount++
						logger.Errorf("[主窗] 读取剪贴板时发生错误 (错误次数: %d/%d): %v", errorCount, maxErrors, r)

						time.Sleep(2 * time.Second)
					}
				}()

				// 尝试读取剪贴板内容
				// 注意：在 macOS 上，直接使用 Fyne/GLFW 读取可能会导致 panic (FormatUnavailable)
				// 因此使用 pbpaste 命令作为安全替代方案
				var content string
				if runtime.GOOS == "darwin" {
					cmd := exec.Command("pbpaste")
					out, err := cmd.Output()
					if err == nil {
						content = string(out)
					} else {
						logger.Errorf("读取剪贴板时发生错误: %v", err)
						// 忽略错误，可能是剪贴板为空或非文本，避免 panic
					}
				} else {
					content = c.clipboard.Content()
				}

				// 成功读取，重置错误计数
				errorCount = 0

				c.callbackFunc(lastContent, content)

				lastContent = content
			}()

			select {
			case <-c.ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()
}

func (c *ClipboardMonitor) Stop() {
	logger.Info("[主窗] 剪贴板监听已停止")

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
		c.ctx = nil
		c.running = false
	}
}

func (w *MainWindow) clipboardMonitorCallback(lastContent string, content string) {
	if content != "" && lastContent != content {
		// 简单的 URL 判定
		if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
			fyne.Do(func() {
				if !w.subView.Container.Hidden {
					// 智能判断：是否为订阅链接
					err := w.subView.ValidateSubscriptionURL(content)

					if err == nil {
						w.subView.SetURL(content)
						widgets.ShowToast(w.window, "检测到订阅链接，已自动填入", theme.InfoIcon())

						// w.switchView(ViewSubscriptions)
						// w.subView.SetURL(content)
						// 显示可点击的 Toast 提示用户可以切换为单次下载
						// toast := widgets.NewClickableToast(
						// 	"检测到订阅链接 [点击切换为单次下载]",
						// 	func() {
						// 		w.switchView(ViewHome)
						// 		w.homeView.SetURL(content)
						// 	},
						// )
						// toast.ShowToast(w.window)
					}
				} else {
					// 不支持订阅：正常下载流程
					w.switchView(ViewHome)
					w.homeView.SetURL(content)
				}

				w.window.Show() // 显示窗口
			})
		}
	}
}

func (w *MainWindow) UpdateClipboardMonitor() {
	cfg := config.Get()
	if cfg.ClipboardMonitor {
		w.clipboardMonitor.Start()
		logger.Info("[主窗] 剪贴板监听已启用")
	} else {
		w.clipboardMonitor.Stop()
		logger.Info("[主窗] 剪贴板监听已禁用")
	}
}

// SetCloseIntercept 设置关闭拦截
func (w *MainWindow) SetCloseIntercept(callback func()) {
	w.window.SetCloseIntercept(callback)
}

// ShowSimpleHelp 显示简单帮助 (Delegator)
func (w *MainWindow) ShowSimpleHelp() {
	// Re-implement or delegate
	// For now, since Toolbar calls it directly via closure if accessible, but here w.ShowSimpleHelp is called by Toolbar?
	// No, Toolbar implementation I wrote uses a dialog directly.
	// But duplicate code?
	// Let's keep this method if other parts need it, or remove.
	// Toolbar passed `w`? No, Toolbar passed `app` and `window`.
	// So Toolbar handles it.
	// We can remove this method if unused.
}

// Run 运行主窗口
func (w *MainWindow) Run() {
	w.window.ShowAndRun()
}

// Show 显示窗口
func (w *MainWindow) Show() {
	w.window.Show()
}

// Hide 隐藏窗口
func (w *MainWindow) Hide() {
	w.window.Hide()
}
