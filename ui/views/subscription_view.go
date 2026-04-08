package views

import (
	"fmt"
	"image/color"
	"path/filepath"
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
	"github.com/hankmor/vdd/core/download"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/subscription"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/ui/helper"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/ui/widgets"
	"github.com/hankmor/vdd/utils"
)

type SubscriptionView struct {
	Container *fyne.Container
	app       fyne.App
	window    fyne.Window
	manager   *subscription.Manager

	contentStack *fyne.Container // 用于在列表和详情之间切换

	// 列表视图组件
	listView      *fyne.Container
	listContainer *fyne.Container // 用于重载列表项

	// 详情视图组件
	detailItems map[string]*DetailItemContext

	// 订阅卡片缓存（用于局部刷新）
	subscriptionCards map[uint]*SubscriptionCardContext

	// 订阅视频列表筛选状态（每个订阅独立）
	currentFilter map[uint][]tasks.TaskStatus // key: subscriptionID, value: 筛选状态列表

	// 跟踪已展开的订阅详情视图
	expandedDetails map[uint]*fyne.Container // key: subscriptionID, value: detailContainer

	// 状态栏组件
	statusContainer *fyne.Container
	statsLabel      *widget.Label // 左侧：统计信息
	messageLabel    *widget.Label // 右侧：临时消息/进度

	// 回调
	OnShowActivation func()
}

type SubscriptionCardContext struct {
	StatusLabel *widget.Label              // 状态统计标签
	Badge       *widgets.NotificationBadge // 新视频角标
	Sub         *subscription.Subscription
}

type DetailItemContext struct {
	Widget    *widgets.TaskItemWidget
	Sub       *subscription.Subscription
	Container *fyne.Container
}

func NewSubscriptionView(app fyne.App, window fyne.Window, manager *subscription.Manager) *SubscriptionView {
	v := &SubscriptionView{
		app:               app,
		window:            window,
		manager:           manager,
		detailItems:       make(map[string]*DetailItemContext),
		subscriptionCards: make(map[uint]*SubscriptionCardContext),
		currentFilter:     make(map[uint][]tasks.TaskStatus),
		expandedDetails:   make(map[uint]*fyne.Container),
	}

	// 注册下载进度回调
	v.manager.GetDownloader().AddProgressListener(func(task *download.DownloadTask) {
		v.updateDetailTask(task)
	})

	// 注册状态变更
	v.manager.GetDownloader().AddNotifyStatusChanged(func(taskID string, status tasks.TaskStatus, errorMsg string) {
		fyne.Do(func() {
			v.refreshDetailTaskItem(taskID)

			// 当任务完成或失败时，局部刷新订阅卡片统计
			if status == tasks.StatusCompleted || status == tasks.StatusFailed || status == tasks.StatusCanceled {
				// 根据 taskID 找到对应的订阅
				task, err := tasks.DAO.GetTaskByID(taskID)
				if err == nil && task != nil && task.SubscriptionID != nil {
					subID := *task.SubscriptionID

					// 刷新订阅卡片统计
					v.refreshSubscriptionCardStats(subID)

					// 局部刷新全局状态栏统计
					v.updateStatusBarStats()

					// 如果该订阅的详情视图已展开，刷新详情列表
					if detailContainer, isExpanded := v.expandedDetails[subID]; isExpanded {
						_ = detailContainer
					}
				}
			}
		})
	})

	// 设置扫描管理器回调，实时更新订阅卡片角标及状态
	scanManager := v.manager.GetScanManager()
	scanManager.SetOnProgress(func(progress *subscription.ScanProgress) {
		// 处理批量扫描通知
		if progress.Category == subscription.ScanCategoryBatch {
			fyne.Do(func() {
				if progress.IsScanning {
					// 显示在右侧消息栏
					v.messageLabel.SetText(fmt.Sprintf("正在更新订阅... (%d/%d)", progress.ScannedSubs, progress.TotalSubs))
				} else {
					v.messageLabel.SetText(fmt.Sprintf("更新完成，新增 %d 个视频", progress.NewCount))
					// 刷新左侧统计
					v.updateStatusBarStats()

					// 5秒后清除消息
					time.AfterFunc(5*time.Second, func() {
						v.messageLabel.SetText("")
					})
				}
			})
			return
		}

		// 处理单个订阅更新
		logger.Debugf("SubscriptionView update: ID=%d, Scanning=%v, NewCount=%d", progress.SubscriptionID, progress.IsScanning, progress.NewCount)
		// 更新对应订阅的角标
		if cardCtx, exists := v.subscriptionCards[progress.SubscriptionID]; exists {
			fyne.Do(func() {
				// 更新角标
				if cardCtx.Badge != nil {
					cardCtx.Badge.SetCount(progress.NewCount)
				}

				// 更新状态文本
				if progress.IsScanning {
					cardCtx.StatusLabel.SetText("正在刷新，请稍后...")
				} else {
					// 扫描结束，恢复显示统计信息
					v.refreshSubscriptionCardStats(progress.SubscriptionID)
					// 也刷新一下底部总统计
					v.updateStatusBarStats()
				}
			})
		} else {
			logger.Debugf("SubscriptionView card not found for ID=%d", progress.SubscriptionID)
		}
	})

	v.buildUI()
	return v
}

func (v *SubscriptionView) buildUI() {
	// 1. 列表容器
	v.listContainer = container.NewVBox()
	scroll := container.NewVScroll(v.listContainer)
	v.listView = container.NewPadded(scroll)

	// 2. 状态栏 (双栏布局)
	v.statsLabel = widget.NewLabel("加载中...")
	v.statsLabel.SizeName = theme.SizeNameCaptionText
	// v.statsLabel.TextStyle = fyne.TextStyle{Monospace: true} // 对齐效果更好

	v.messageLabel = widget.NewLabel("")
	v.messageLabel.Alignment = fyne.TextAlignTrailing
	v.messageLabel.SizeName = theme.SizeNameCaptionText

	// 使用 Border 布局：左侧统计，右侧消息
	// 中间用 Spacer 撑开? Border Center 是撑满的。
	// Left: stats, Right: message? Right side items in Border are packed to the right.
	v.statusContainer = container.NewBorder(nil, nil,
		container.NewPadded(v.statsLabel),   // Left
		container.NewPadded(v.messageLabel), // Right
	)

	// 添加个背景或分割线? Container 默认透明。
	// 可以加个 Separator 在上面
	statusBarWithSep := container.NewBorder(widget.NewSeparator(), nil, nil, nil, v.statusContainer)

	// 3. 使用 Border 布局: 列表在上，状态栏在下
	v.Container = container.NewBorder(nil, statusBarWithSep, nil, nil, v.listView)

	// 加载初始数据
	v.Refresh()
}

// loadInlineDetailVideos 加载内嵌的详情列表
func (v *SubscriptionView) loadInlineDetailVideos(sub *subscription.Subscription, targetContainer *fyne.Container) {
	targetContainer.Objects = nil
	// 注意：这里我们不像之前那样清空所有 detailItems，因为其他订阅可能也是展开的。

	// 工具栏 (Start All, Stop All, etc.)
	toolbar := v.createInlineToolbar(sub, targetContainer)
	targetContainer.Add(toolbar)
	targetContainer.Add(widget.NewSeparator())

	// 使用分页加载（初始加载第1页，每页20个）
	v.loadTasksPage(sub, targetContainer, 1)
}

// loadTasksPage 加载指定页的任务（异步执行）
func (v *SubscriptionView) loadTasksPage(sub *subscription.Subscription, targetContainer *fyne.Container, page int) {
	fyne.Do(func() {
		// 第1页显示加载提示
		if page == 1 {
			loadingLabel := widget.NewLabel("加载中...")
			loadingLabel.Alignment = fyne.TextAlignCenter
			loadingLabel.Importance = widget.LowImportance
			targetContainer.Add(container.NewCenter(loadingLabel))
			targetContainer.Refresh()
		} else {
			// 后续页移除旧的"加载更多"按钮
			v.removeLoadMoreButton(targetContainer)
		}
	})

	// 异步查询数据库
	go func() {
		// 人为延迟，确保 Loading 提示对用户可见 (优化点击反馈)
		time.Sleep(200 * time.Millisecond)

		// 获取当前筛选条件
		filterStatuses, exists := v.currentFilter[sub.ID]
		if !exists {
			filterStatuses = []tasks.TaskStatus{}
			v.currentFilter[sub.ID] = filterStatuses
		}

		// 分页查询
		filter := tasks.TaskFilter{
			SubscriptionID: sub.ID,
			Statuses:       filterStatuses, // 直接使用多状态
			Page:           page,
			PageSize:       10,
		}

		tasksList, total, err := tasks.DAO.GetTasksBySubscriptionIDPaginated(filter)

		// 回到UI线程更新界面
		fyne.Do(func() {

			// 移除加载提示（第1页）
			if page == 1 {
				targetContainer.Objects = nil
				// 重新添加工具栏
				toolbar := v.createInlineToolbar(sub, targetContainer)
				targetContainer.Add(toolbar)
				targetContainer.Add(widget.NewSeparator())
			}

			if err != nil {
				if page == 1 {
					targetContainer.Add(widget.NewLabel(fmt.Sprintf("无法加载视频列表: %v", err)))
				}
				targetContainer.Refresh()
				return
			}

			if page == 1 && len(tasksList) == 0 {
				targetContainer.Add(container.NewCenter(widget.NewLabel("暂无视频记录")))
				targetContainer.Refresh()
				return
			}

			// 添加任务项
			for _, task := range tasksList {
				item := v.createTaskItem(task, sub, targetContainer)
				v.detailItems[task.ID] = &DetailItemContext{
					Widget:    item,
					Sub:       sub,
					Container: targetContainer,
				}

				// 左侧缩进 (视觉层级)
				spacer := canvas.NewRectangle(color.Transparent)
				spacer.SetMinSize(fyne.NewSize(20, 0))

				row := container.NewBorder(nil, nil, spacer, nil, item.Container)
				targetContainer.Add(row)
				targetContainer.Add(widget.NewSeparator())
			}

			// 计算已加载数量
			loadedCount := page * filter.PageSize
			if int64(loadedCount) > total {
				loadedCount = int(total)
			}

			// 如果还有更多数据，显示"加载更多"按钮
			if int64(loadedCount) < total {
				loadMoreBtn := widget.NewButton(
					fmt.Sprintf("加载更多 (已显示 %d/%d 个视频)", loadedCount, total),
					func() {
						v.loadTasksPage(sub, targetContainer, page+1)
					},
				)
				loadMoreBtn.Importance = widget.LowImportance

				loadMoreContainer := container.NewCenter(loadMoreBtn)
				targetContainer.Add(loadMoreContainer)
			} else if total > 0 {
				// 已加载全部，显示提示
				allLoadedLabel := widget.NewLabel(fmt.Sprintf("已显示全部 %d 个视频", total))
				allLoadedLabel.Alignment = fyne.TextAlignCenter
				allLoadedLabel.Importance = widget.LowImportance
				targetContainer.Add(container.NewCenter(allLoadedLabel))
			}

			targetContainer.Refresh()
		})
	}()
}

// removeLoadMoreButton 移除"加载更多"按钮或"已显示全部"提示
func (v *SubscriptionView) removeLoadMoreButton(targetContainer *fyne.Container) {
	if len(targetContainer.Objects) == 0 {
		return
	}

	// 查找并移除最后的按钮容器
	lastIdx := len(targetContainer.Objects) - 1
	lastObj := targetContainer.Objects[lastIdx]

	// 检查是否为居中容器（我们用 NewCenter 创建的）
	if centerContainer, ok := lastObj.(*fyne.Container); ok {
		if len(centerContainer.Objects) > 0 {
			// 检查是否包含按钮或标签
			if _, isBtn := centerContainer.Objects[0].(*widget.Button); isBtn {
				targetContainer.Objects = targetContainer.Objects[:lastIdx]
				return
			}
			if label, isLabel := centerContainer.Objects[0].(*widget.Label); isLabel {
				if label.Importance == widget.LowImportance {
					targetContainer.Objects = targetContainer.Objects[:lastIdx]
					return
				}
			}
		}
	}
}

// createFilterButtons 创建筛选按钮组
func (v *SubscriptionView) createFilterButtons(sub *subscription.Subscription, targetContainer *fyne.Container) *fyne.Container {
	// 获取当前筛选状态（如果没有则使用默认）
	currentStatuses, exists := v.currentFilter[sub.ID]
	if !exists {
		// 默认：全部
		currentStatuses = []tasks.TaskStatus{}
		v.currentFilter[sub.ID] = currentStatuses
	}

	// 筛选按钮创建函数
	createFilterBtn := func(label string, filterStatuses []tasks.TaskStatus) *widget.Button {
		btn := widget.NewButton(label, func() {
			// 设置新筛选条件
			v.currentFilter[sub.ID] = filterStatuses
			// 重新加载第1页
			v.loadInlineDetailVideos(sub, targetContainer)
		})

		// 检查当前是否选中
		isSelected := len(currentStatuses) == len(filterStatuses)
		if isSelected && len(filterStatuses) > 0 {
			// 比较每个状态
			statusMap := make(map[tasks.TaskStatus]bool)
			for _, s := range currentStatuses {
				statusMap[s] = true
			}
			for _, s := range filterStatuses {
				if !statusMap[s] {
					isSelected = false
					break
				}
			}
		}

		if isSelected {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}

		return btn
	}

	// 5个筛选选项
	allBtn := createFilterBtn("全部", nil) // nil = 查询所有
	downloadingBtn := createFilterBtn("下载", []tasks.TaskStatus{tasks.StatusDownloading, tasks.StatusQueued})
	failedBtn := createFilterBtn("失败", []tasks.TaskStatus{tasks.StatusFailed})
	completedBtn := createFilterBtn("完成", []tasks.TaskStatus{tasks.StatusCompleted})
	canceledBtn := createFilterBtn("取消", []tasks.TaskStatus{tasks.StatusCanceled})

	return container.NewHBox(allBtn, downloadingBtn, failedBtn, completedBtn, canceledBtn)
}

func (v *SubscriptionView) createInlineToolbar(sub *subscription.Subscription, targetContainer *fyne.Container) fyne.CanvasObject {
	// 辅助函数：获取当前筛选下的所有任务
	getFilteredTasks := func() ([]*tasks.Task, error) {
		filterStatuses, exists := v.currentFilter[sub.ID]
		if !exists {
			// 默认：下载中+失败
			filterStatuses = []tasks.TaskStatus{tasks.StatusDownloading, tasks.StatusQueued, tasks.StatusFailed, tasks.StatusCanceled}
		}

		return tasks.DAO.GetTasksByFilter(tasks.TaskFilter{
			SubscriptionID: sub.ID,
			Statuses:       filterStatuses,
		})
	}

	// 批量开始（仅针对当前筛选结果）
	// 批量开始（针对当前订阅所有任务，忽略筛选）
	startBtn := widgets.NewButtonWithTooltip("", icons.ThemedPlayIcon, func() {
		// 获取所有任务（忽略筛选）
		allTasks, err := tasks.DAO.GetTasksBySubscriptionID(sub.ID)
		if err != nil {
			return
		}

		var toStart []*tasks.Task
		for _, t := range allTasks {
			if t.CanStartDownload() {
				toStart = append(toStart, t)
			}
		}

		if len(toStart) > 0 {
			var refreshed bool
			v.manager.GetDownloader().StartAllWithCallback(toStart, func() {
				if refreshed {
					return
				}
				refreshed = true
				// 回调在所有任务调度完成后触发
				fyne.Do(func() {
					v.loadInlineDetailVideos(sub, targetContainer)
					widgets.ShowToast(v.window, fmt.Sprintf("已开始 %d 个任务", len(toStart)), icons.ThemedPlayIcon)
				})
			})
		} else {
			widgets.ShowToast(v.window, "没有可开始的任务", theme.InfoIcon())
		}
	}, "开始所有任务")

	// 停止所有（针对当前订阅所有任务，忽略筛选）
	stopBtn := widgets.NewButtonWithTooltip("", icons.ThemedDeleteCircleIcon, func() {
		logger.Debug("停止当前订阅所有下载任务...")

		// 使用原子的批量取消方法，避免并发问题
		count := v.manager.GetDownloader().CancelSubscription(sub.ID)

		if count > 0 {
			widgets.ShowToast(v.window, fmt.Sprintf("已停止 %d 个任务", count), icons.ThemedDeleteCircleIcon)
			// 刷新当前列表以立即显示状态变化
			v.loadInlineDetailVideos(sub, targetContainer)
		} else {
			widgets.ShowToast(v.window, "没有正在下载的任务", theme.InfoIcon())
		}
	}, "取消所有任务")

	// 清空列表（仅针对当前筛选结果）
	deleteBtn := widgets.NewButtonWithTooltip("", icons.ThemedClearAllHistoryIcon, func() {
		widgets.ShowConfirmDialog("清空当前列表", "确定要删除当前筛选列表中的所有视频记录吗？\n该操作仅影响当前可见的任务。", func() {
			filteredTasks, _ := getFilteredTasks()
			for _, t := range filteredTasks {
				v.manager.GetDownloader().Cancel(t)
				tasks.DAO.DeleteTask(t.ID)
			}
			v.loadInlineDetailVideos(sub, targetContainer) // Reload inline
		}, v.window)
	}, "清空当前列表")

	// 筛选按钮组
	filterButtons := v.createFilterButtons(sub, targetContainer)

	// 工具栏布局：左侧=筛选按钮，右侧=操作按钮
	return container.NewBorder(
		nil, nil,
		filterButtons, // 左侧
		container.NewHBox(startBtn, stopBtn, deleteBtn), // 右侧
	)
}

func (v *SubscriptionView) createTaskItem(task *tasks.Task, sub *subscription.Subscription, targetContainer *fyne.Container) *widgets.TaskItemWidget {
	actions := widgets.TaskActions{
		OnStart: func() {
			go v.manager.GetDownloader().Schedule(task, auth.GetAutherization())
		},
		OnCancel: func() {
			v.manager.GetDownloader().Cancel(task)
		},
		OnOpenFolder: func() {
			dir := filepath.Join(config.Get().DownloadDir, utils.SanitizeFileName(sub.Name))
			logger.Debugf("打开文件夹: %s", dir)
			utils.OpenFolder(dir)
		},
		OnDelete: func() {
			chkDeleteFile := widget.NewCheck("同时删除本地文件", nil)
			content := container.NewVBox(
				widget.NewLabel("确定要删除此任务吗？"),
				chkDeleteFile,
			)
			widgets.ShowConfirmWithContent("删除确认", content, func() {
				v.manager.GetDownloader().Cancel(task)
				tasks.DAO.DeleteTask(task.ID)
				// Delete files logic (simplified here or call helper)
				// refresh list
				v.loadInlineDetailVideos(sub, targetContainer) // Inline reload
			}, v.window)
		},
	}
	return widgets.NewTaskItemWidget(task, actions)
}

// updateDetailTask 更新详情页的单个任务进度
func (v *SubscriptionView) updateDetailTask(task *download.DownloadTask) {
	// 必须在 UI 线程执行
	fyne.Do(func() {
		if v.detailItems == nil {
			return
		}
		ctx, exists := v.detailItems[task.ID]
		if !exists || ctx == nil || ctx.Widget == nil {
			return
		}

		// Update UI directly using exposed fields from TaskItemWidget
		statusText := fmt.Sprintf("%s • %s • %s/s • ETA: %s",
			task.Stage,
			utils.FormatBytes(task.TotalSize),
			utils.FormatBytes(task.Speed),
			utils.FormatDurationSeconds(float64(task.ETA)),
		)
		ctx.Widget.StatusLabel.SetText(statusText)
		ctx.Widget.ProgressBar.SetValue(task.Progress / 100.0)
	})
}

func (v *SubscriptionView) refreshDetailTaskItem(taskID string) {
	if v.detailItems == nil {
		return
	}
	ctx, exists := v.detailItems[taskID]
	if !exists || ctx == nil {
		return
	}

	// Reload from DB
	task, err := tasks.DAO.GetTaskByID(taskID)
	if err != nil {
		return
	}

	actions := widgets.TaskActions{
		OnStart: func() {
			go v.manager.GetDownloader().Schedule(task, auth.GetAutherization())
		},
		OnCancel: func() {
			v.manager.GetDownloader().Cancel(task)
		},
		OnOpenFolder: func() {
			utils.OpenFolder(filepath.Dir(task.TemplatePath))
		},
		OnDelete: func() {
			v.manager.GetDownloader().Cancel(task)
			tasks.DAO.DeleteTask(task.ID)
			v.loadInlineDetailVideos(ctx.Sub, ctx.Container)
		},
	}

	fyne.Do(func() {
		ctx.Widget.RefreshActions(task, actions)

		// 尝试从 Downloader 获取实时运行时状态 (优先显示 Stage)
		var statusText string
		rtTask := v.manager.GetDownloader().GetTask(taskID)
		if rtTask != nil && rtTask.Stage != "" {
			statusText = fmt.Sprintf("%s • %s • %s/s • ETA: %s",
				rtTask.Stage,
				utils.FormatBytes(rtTask.TotalSize),
				utils.FormatBytes(rtTask.Speed),
				utils.FormatDurationSeconds(float64(rtTask.ETA)))
		} else {
			statusText = fmt.Sprintf("%s • %s", task.Status.Name(), utils.FormatBytes(task.TotalSize))
		}

		ctx.Widget.StatusLabel.SetText(statusText)
		ctx.Widget.ProgressBar.SetValue(task.Progress / 100.0)
	})
}

// refreshSubscriptionCardStats 局部刷新订阅卡片的统计信息（不重建UI）
func (v *SubscriptionView) refreshSubscriptionCardStats(subID uint) {
	cardCtx, exists := v.subscriptionCards[subID]
	if !exists {
		return // 卡片不存在，跳过
	}

	// 重新查询统计数据
	stats, err := tasks.DAO.GetSubscriptionStats(subID)
	if err != nil {
		return
	}

	// 更新状态文本
	statusText := ""
	if cardCtx.Sub.Status == subscription.StatusPaused {
		statusText = "已暂停"
	} else {
		statusText = "活跃"
	}

	statsStr := fmt.Sprintf("共%d个视频(完成:%d, 下载:%d, 失败:%d, 取消:%d)",
		stats.Total, stats.Completed, stats.Downloading, stats.Failed, stats.Canceled)

	statusInfo := fmt.Sprintf("状态: %s • %s", statusText, statsStr)

	fyne.Do(func() {
		// 只更新标签文本，不重建UI
		cardCtx.StatusLabel.SetText(statusInfo)
	})
}

func (v *SubscriptionView) Refresh() {
	// 清空容器并显示加载提示
	v.listContainer.Objects = nil
	v.subscriptionCards = make(map[uint]*SubscriptionCardContext) // 清空缓存

	// 显示加载中提示
	loadingLabel := widget.NewLabel("加载中...")
	loadingLabel.Alignment = fyne.TextAlignCenter
	progressBar := widgets.NewThinProgressBar()

	loadingContainer := container.NewCenter(
		container.NewVBox(
			layout.NewSpacer(),
			loadingLabel,
			progressBar,
			layout.NewSpacer(),
		),
	)
	v.listContainer.Add(loadingContainer)
	v.listContainer.Refresh()

	// 异步加载订阅列表
	go func() {
		subs, err := subscription.DAO.GetAll()

		fyne.Do(func() {
			// 移除加载提示
			v.listContainer.Objects = nil

			if err != nil {
				v.listContainer.Add(widget.NewLabel(fmt.Sprintf("加载失败: %v", err)))
				v.listContainer.Refresh()
				return
			}

			if len(subs) == 0 {
				emptyLabel := widget.NewLabel("还没有订阅任何频道")
				emptyLabel.Alignment = fyne.TextAlignCenter
				emptyLabel.Importance = widget.LowImportance

				v.listContainer.Add(container.NewCenter(
					container.NewVBox(
						layout.NewSpacer(),
						emptyLabel,
						layout.NewSpacer(),
					),
				))
				v.listContainer.Refresh()
				return
			}

			// 先快速创建所有卡片（不加载统计数据）
			for _, sub := range subs {
				card := v.createSubscriptionCardQuick(sub)
				v.listContainer.Add(card)
				v.listContainer.Add(widget.NewSeparator())
			}
			v.listContainer.Refresh()

			// 然后异步加载每个卡片的统计数据
			for _, sub := range subs {
				go v.loadCardStats(sub)
			}

			// 更新状态栏
			v.updateStatusBarStats()
		})
	}()
}

// updateStatusBarStats 更新底部状态栏左侧的统计信息
func (v *SubscriptionView) updateStatusBarStats() {
	go func() {
		// 获取所有订阅任务的统计信息
		stats, err := tasks.DAO.GetAllSubscriptionStats()

		fyne.Do(func() {
			if err != nil {
				v.statsLabel.SetText("统计数据加载失败")
				return
			}

			// 格式化状态栏文本
			statusText := fmt.Sprintf("总视频: %d  •  下载中: %d  •  排队: %d  •  已完成: %d  •  失败: %d",
				stats.Total,
				stats.Downloading,
				stats.Queued,
				stats.Completed,
				stats.Failed)

			v.statsLabel.SetText(statusText)
		})
	}()
}

// updateStatusBar 兼容旧方法名，重定向到 Stats
func (v *SubscriptionView) updateStatusBar() {
	v.updateStatusBarStats()
}

// createSubscriptionCardQuick 快速创建订阅卡片（不加载统计数据）
func (v *SubscriptionView) createSubscriptionCardQuick(sub *subscription.Subscription) fyne.CanvasObject {
	// ... 返回基础卡片，统计信息显示为"加载中..."
	// 完整实现见下一个方法
	return v.createSubscriptionCardWithStats(sub, nil, false)
}

// loadCardStats 异步加载卡片统计数据
func (v *SubscriptionView) loadCardStats(sub *subscription.Subscription) {
	stats, err := tasks.DAO.GetSubscriptionStats(sub.ID)

	fyne.Do(func() {
		if err != nil {
			stats = &tasks.SubscriptionStats{} // 使用空统计
		}

		// 更新卡片上下文中的状态标签
		if ctx, exists := v.subscriptionCards[sub.ID]; exists {
			statusText := "活跃"
			if sub.Status == subscription.StatusPaused {
				statusText = "已暂停"
			}

			statsStr := fmt.Sprintf("共%d个视频(完成:%d, 下载:%d, 失败:%d, 取消:%d)",
				stats.Total, stats.Completed, stats.Downloading, stats.Failed, stats.Canceled)

			statusInfo := fmt.Sprintf("状态: %s • %s", statusText, statsStr)
			ctx.StatusLabel.SetText(statusInfo)
		}
	})
}

func (v *SubscriptionView) createSubscriptionCard(sub *subscription.Subscription) fyne.CanvasObject {
	// 同步加载统计数据（向后兼容）
	stats, _ := tasks.DAO.GetSubscriptionStats(sub.ID)
	return v.createSubscriptionCardWithStats(sub, stats, true)
}

// createSubscriptionCardWithStats 创建订阅卡片（支持异步统计数据加载）
// statsLoaded: true表示stats有效，false表示显示"加载中..."
func (v *SubscriptionView) createSubscriptionCardWithStats(sub *subscription.Subscription, stats *tasks.SubscriptionStats, statsLoaded bool) fyne.CanvasObject {
	// 封面图片 (缩略图)
	coverImg := canvas.NewImageFromResource(assets.DefaultThumbnail) // 默认
	coverImg.FillMode = canvas.ImageFillContain
	coverImg.SetMinSize(fyne.NewSize(120, 68)) // 类似 16:9 的比例

	// 异步加载缩略图
	if sub.Thumbnail != "" {
		go func() {
			helper.SharedThumbnailManager().LoadThumbnail(sub.Thumbnail, func(res fyne.Resource) {
				fyne.Do(func() {
					coverImg.Resource = res
					coverImg.Refresh()
				})
			})
		}()
	}

	// 标题
	// 标题 - 使用 RichText 以支持更大的字体
	titleLabel := widget.NewRichText(&widget.TextSegment{
		Text: sub.Name,
		Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignLeading,
			TextStyle: fyne.TextStyle{Bold: true},
			SizeName:  theme.SizeNameText, // 使用副标题大小 (18px)
		},
	})
	titleLabel.Truncation = fyne.TextTruncateEllipsis

	// 创建角标
	badge := widgets.NewNotificationBadge()
	// 从数据库读取角标数量（仅显示新视频，不包括已读）
	badgeState, _ := subscription.DAO.GetBadgeState(sub.ID)
	if badgeState != nil && badgeState.NewCount > 0 {
		badge.SetCount(badgeState.NewCount)
	}

	// 将角标和标题组合(角标在左，标题在右)
	// 使用Border布局：角标在左侧固定，标题占据中心区域
	// 使用 NewCenter 包裹 badge 以防止被 Border 布局纵向拉伸导致变形
	badgeWrapper := container.NewCenter(badge)
	titleContainer := container.NewBorder(nil, nil, badgeWrapper, nil, titleLabel)

	// 状态行
	statusText := ""
	if sub.Status == subscription.StatusPaused {
		statusText = "已暂停"
	} else {
		statusText = "活跃"
	}

	// 统计信息
	var statusInfo string
	if !statsLoaded || stats == nil {
		// 未加载统计数据时显示"加载中..."
		statusInfo = fmt.Sprintf("状态: %s • 加载中...", statusText)
	} else {
		statsStr := fmt.Sprintf("共%d个视频(完成:%d, 下载:%d, 失败:%d, 取消:%d)",
			stats.Total, stats.Completed, stats.Downloading, stats.Failed, stats.Canceled)
		statusInfo = fmt.Sprintf("状态: %s • %s", statusText, statsStr)
	}

	// 使用标准主题颜色创建标签
	statusLabel := widget.NewLabel(statusInfo)
	statusLabel.SizeName = theme.SizeNameCaptionText

	// 缓存卡片上下文，用于局部刷新
	v.subscriptionCards[sub.ID] = &SubscriptionCardContext{
		StatusLabel: statusLabel,
		Badge:       badge,
		Sub:         sub,
	}

	// 详情容器 (隐藏)
	detailContainer := container.NewVBox()
	detailContainer.Hide()

	// 按钮组
	// 1. 暂停/恢复
	pauseIcon := icons.ThemedPauseIcon
	pauseTip := "暂停更新"
	if sub.Status == subscription.StatusPaused {
		pauseIcon = icons.ThemedPlayIcon
		pauseTip = "恢复更新"
	}
	pauseBtn := widgets.NewButtonWithTooltip("", pauseIcon, nil, pauseTip)
	pauseBtn.OnTapped = func() {
		newStatus := subscription.StatusPaused
		if sub.Status == subscription.StatusPaused {
			newStatus = subscription.StatusActive
		}

		if err := subscription.DAO.UpdateStatus(sub.ID, newStatus); err != nil {
			logger.Errorf("更新订阅状态失败: %v", err)
			return
		}

		// 更新本地对象状态
		sub.Status = newStatus

		// 局部更新按钮图标和提示
		if newStatus == subscription.StatusPaused {
			pauseBtn.SetIcon(icons.ThemedPlayIcon)
			pauseBtn.SetToolTip("恢复更新")
		} else {
			pauseBtn.SetIcon(icons.ThemedPauseIcon)
			pauseBtn.SetToolTip("暂停更新")
		}

		// 局部更新状态文本
		v.refreshSubscriptionCardStats(sub.ID)

		// 如果是暂停，需要取消正在进行的下载吗？目前逻辑是"停止更新"，不一定是停止当前任务。
		// 保持原逻辑，这里只更新了状态。
	}

	// 2. Open Folder
	folderBtn := widgets.NewButtonWithTooltip("", icons.ThemedOpenFolderIcon, func() {
		dir := filepath.Join(config.Get().DownloadDir, utils.SanitizeFileName(sub.Name))
		logger.Debugf("打开文件夹: %s", dir)
		utils.OpenFolder(dir)
	}, "打开文件夹")

	// 3. 检查更新
	checkUpdateBtn := widgets.NewButtonWithTooltip("", icons.ThemedRefreshIcon, func() {
		// 检查是否正在扫描
		if v.manager.GetScanManager().IsScanning(sub.ID) {
			widgets.ShowToast(v.window, "正在扫描中，请稍候...", theme.InfoIcon())
			return
		}

		go func() {
			widgets.ShowToast(v.window, fmt.Sprintf("正在检查更新: %s", sub.Name), theme.InfoIcon())
			scanMgr := v.manager.GetScanManager()
			scanMgr.ScanOne(sub.ID)
		}()
	}, "检查更新")

	// 4. Delete
	deleteBtn := widgets.NewButtonWithTooltip("", icons.ThemedClearAllHistoryIcon, func() {
		widgets.ShowConfirmDialog("删除订阅", fmt.Sprintf("确定要删除订阅 \"%s\" 吗？\n这将清空当前的列表和所有关联的任务记录。", sub.Name), func() {
			// 检查是否正在解析中
			scanMgr := v.manager.GetScanManager()
			if scanMgr.IsScanning(sub.ID) {
				// 先取消正在进行的扫描
				logger.Infof("[订阅] 删除前取消订阅 %d (%s) 的扫描", sub.ID, sub.Name)
				scanMgr.CancelScan(sub.ID)
			}

			// 同时也尝试取消可能的初始化过程 (如果是刚添加正在解析元数据)
			v.manager.CancelInitialization(sub.URL)

			// 等待一点时间让进程退出
			if scanMgr.IsScanning(sub.ID) {
				time.Sleep(500 * time.Millisecond)
			}

			// 执行删除
			if err := subscription.DAO.Delete(sub.ID); err != nil {
				widgets.ShowError("删除失败, 请稍后重试", err, v.window)
			}
			v.Refresh()
		}, v.window)
	}, "删除订阅")

	// 5. Details (Toggle)
	detailBtnIcon := theme.InfoIcon()
	// We might want an icon indicating "Expand" like ArrowDown, but Info is fine as "Manage".

	detailBtn := widgets.NewButtonWithTooltip("", detailBtnIcon, nil, "视频列表")
	detailBtn.SetOnTapped(func() {
		if detailContainer.Visible() {
			detailContainer.Hide()
			detailBtn.Importance = widget.LowImportance
			// 移除展开状态
			delete(v.expandedDetails, sub.ID)
		} else {
			detailContainer.Show()
			detailBtn.Importance = widget.HighImportance

			// 清除角标 (用户已查看)
			if cardCtx, exists := v.subscriptionCards[sub.ID]; exists && cardCtx.Badge != nil {
				cardCtx.Badge.SetCount(0)
				// 更新数据库和内存计数
				v.manager.GetScanManager().MarkAsRead(sub.ID)
			}

			// 记录展开状态
			v.expandedDetails[sub.ID] = detailContainer
			// 加载数据
			v.loadInlineDetailVideos(sub, detailContainer)
		}
	})
	detailBtn.Importance = widget.LowImportance

	actions := container.NewHBox(pauseBtn, folderBtn, checkUpdateBtn, deleteBtn, detailBtn)

	// 布局
	leftContainer := container.NewPadded(coverImg)

	// 使用 VBox 技巧垂直居中: VBox(Spacer, info, Spacer)
	centeredInfo := container.NewVBox(layout.NewSpacer(), titleContainer, statusLabel, layout.NewSpacer())
	centeredActions := container.NewVBox(layout.NewSpacer(), actions, layout.NewSpacer())

	cardBorder := container.NewBorder(nil, nil, leftContainer, centeredActions, centeredInfo)
	cardPadded := container.NewPadded(cardBorder)

	// 组合: 卡片在上, 详情在下
	return container.NewVBox(cardPadded, detailContainer)
}

// ShowAddDialog 显示添加订阅对话框
// 如果提供了 prefilledURL，会自动填充到输入框
func (v *SubscriptionView) ShowAddDialog(prefilledURL ...string) {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("粘贴播放列表或频道链接...")

	// 如果有预填充URL，设置到输入框
	if len(prefilledURL) > 0 && prefilledURL[0] != "" {
		entry.SetText(prefilledURL[0])
	}

	content := container.NewVBox(
		widget.NewLabel("请输入 YouTube/Bilibili 播放列表或频道链接:"),
		entry,
	)

	widgets.ShowConfirmWithContent("添加订阅", content, func() {
		url := entry.Text
		if url == "" {
			return
		}

		// 1. 快速验证URL格式
		if err := v.manager.ValidateSubscriptionURL(url); err != nil {
			// 确保在主线程显示错误
			fyne.Do(func() {
				dialog.ShowError(err, v.window)
			})
			return
		}

		// 2. 立即创建占位订阅并显示
		v.manager.AddSubscriptionWithPlaceholder(url,
			func(placeholder *subscription.Subscription) {
				// 占位记录创建成功，立即刷新UI
				fyne.Do(func() {
					widgets.ShowToast(v.window, "订阅已添加，正在解析...", icons.ThemedOkIcon)
					v.Refresh() // 立即显示占位记录
				})
			},
			func(sub *subscription.Subscription) {
				// 解析成功，更新UI
				fyne.Do(func() {
					widgets.ShowToast(v.window,
						fmt.Sprintf("订阅 \"%s\" 解析完成！", sub.Name),
						icons.ThemedOkIcon)
					v.Refresh() // 刷新显示完整信息
				})
			},
			func(err error) {
				// 解析失败
				fyne.Do(func() {
					dialog.ShowError(
						fmt.Errorf("订阅解析失败: %v", err),
						v.window)
					v.Refresh() // 刷新列表（占位记录已被删除）
				})
			})
	}, v.window)
}

// CreateToolbarItems 返回工具栏需要的动态按钮
func (v *SubscriptionView) CreateToolbarItems() []fyne.CanvasObject {
	addBtn := widgets.NewButtonWithTooltip("", theme.ContentAddIcon(), func() {
		v.ShowAddDialog()
	}, "添加新订阅")

	// 停止所有订阅下载 (全局)
	stopAllBtn := widgets.NewButtonWithTooltip("", icons.ThemedDeleteCircleIcon, func() {
		logger.Debug("取消下载所有订阅任务...")
		v.manager.GetDownloader().CancelAllSubscriptions()
	}, "取消所有任务")

	return []fyne.CanvasObject{addBtn, stopAllBtn}
}

// ValidateSubscriptionURL 验证URL是否为有效订阅（用于剪贴板智能识别）
func (v *SubscriptionView) ValidateSubscriptionURL(url string) error {
	return v.manager.ValidateSubscriptionURL(url)
}

// SetURL 设置订阅URL并显示添加对话框（用于剪贴板自动填充）
func (v *SubscriptionView) SetURL(url string) {
	v.ShowAddDialog(url)
}
