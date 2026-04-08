package views

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/hankmor/vdd/core/auth"
	"github.com/hankmor/vdd/core/download"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/ui/widgets"
	"github.com/hankmor/vdd/utils"
)

// TaskItemWidget has been moved to ui/widgets/task_item.go
// TaskListView 任务列表视图
type TaskListView struct {
	Container *fyne.Container
	taskList  *fyne.Container // 垂直布局列表
	app       fyne.App
	window    fyne.Window

	downloader *download.Downloader
	// State
	items map[string]*widgets.TaskItemWidget // 任务ID -> 组件映射
	
	// 状态栏
	statusBar *widget.Label
}

func NewTaskListView(app fyne.App, window fyne.Window, d *download.Downloader) *TaskListView {
	v := &TaskListView{
		app:        app,
		window:     window,
		items:      make(map[string]*widgets.TaskItemWidget),
		downloader: d,
	}

	// 下载进度回调
	v.downloader.AddProgressListener(func(task *download.DownloadTask) {
		v.updateTask(task)
	})

	// 设置任务状态改变回调，用于异步下载失败/成功时刷新 UI
	v.downloader.SetNotifyStatusChanged(func(taskID string, status tasks.TaskStatus, errorMsg string) {
		fyne.Do(func() {
			v.refreshTaskItem(taskID)
			// 更新状态栏
			v.updateStatusBar()
		})
	})

	v.taskList = container.NewVBox()

	// 包装在滚动容器中
	scroll := container.NewVScroll(v.taskList)
	
	// 状态栏
	v.statusBar = widget.NewLabel("")
	v.statusBar.SizeName = theme.SizeNameCaptionText
	statusBarContainer := container.NewHBox(v.statusBar)

	// 使用 Border 布局: 列表在上，状态栏在下
	v.Container = container.NewBorder(nil, statusBarContainer, nil, nil, scroll)

	// 初次加载
	v.Refresh()
	
	// 初始化状态栏
	go func() {
		time.Sleep(100 * time.Millisecond) // 短暂延迟确保 Refresh 先执行
		v.updateStatusBar()
	}()

	return v
}

// CreateToolbarItems 创建批量操作工具栏项 (返回按钮列表)
func (v *TaskListView) CreateToolbarItems() []fyne.CanvasObject {
	// 1. Start All
	startBtn := widgets.NewButtonWithTooltip("", icons.ThemedPlayIcon, func() {
		// 加载所有任务 (过滤订阅任务)
		// 原逻辑: GetTasksByStatus or GetAllTasks?
		// 这里我们需要所有非订阅任务
		allTasks, err := tasks.DAO.GetManualTasks()
		if err != nil {
			logger.Errorf("Failed to load tasks: %v", err)
			return
		}

		// 筛选需要启动的任务 (排除订阅任务)
		var toStart []*tasks.Task
		for _, t := range allTasks {
			if t.SubscriptionID != nil {
				continue
			}
			// 只有 暂停、失败、取消、或者 Pending 的任务需要启动
			// Download, Completed, Queued 不需要
			if t.Status != tasks.StatusDownloading && t.Status != tasks.StatusCompleted && t.Status != tasks.StatusQueued {
				toStart = append(toStart, t)
			}
		}

		if len(toStart) == 0 {
			return
		}

		logger.Infof("批量启动 %d 个任务", len(toStart))
		v.downloader.StartAll(toStart)
	}, "全部开始")
	startBtn.Importance = widget.LowImportance // 扁平风格

	// 2. Stop All
	// 2. Stop All
	stopBtn := widgets.NewButtonWithTooltip("", icons.ThemedDeleteCircleIcon, func() {
		// CancelAll 需要区分吗？Downloader 目前是全局的。
		// 如果订阅任务也在运行，CancelAll 会全部停止。
		// 这是一个策略问题：主页面的"全部停止"是否应该停止后台自动的订阅任务？
		// 通常用户认为"Stop All"是停止一切。保持现状即可，或者遍历 active tasks 过滤。
		// 暂时保持 Stop All 停止所有，包括订阅任务，这也合理。
		// v.downloader.CancelAll()
		// 仅停止任务列表中的任务 (非订阅任务)
		manualTasks, err := tasks.DAO.GetActiveManualTasks()
		if err != nil {
			logger.Errorf("Failed to get active manual tasks: %v", err)
			return
		}

		logger.Infof("[Tasks] Batch stopping %d manual tasks", len(manualTasks))

		// 逐个取消
		for _, task := range manualTasks {
			err := v.downloader.Cancel(task)
			if err != nil {
				logger.Warnf("Failed to cancel task %s: %v", task.ID, err)
			}
		}
	}, "全部停止")
	stopBtn.Importance = widget.LowImportance

	// 3. Clear Completed
	clearBtn := widgets.NewButtonWithTooltip("", icons.ThemedClearAllIcon, func() {
		// 检查是否有完成的任务
		hasCompleted := false
		manualTasks, err := tasks.DAO.GetManualTasks()
		if err == nil {
			for _, t := range manualTasks {
				if t.Status == tasks.StatusCompleted {
					hasCompleted = true
					break
				}
			}
		}
		
		if !hasCompleted {
			return 
		}

		v.showBatchDeleteDialog("清除已完成", "确定要清除所有已完成任务吗？", func(deleteFile bool) {
			
			var count int
			for _, t := range manualTasks {
				if t.Status == tasks.StatusCompleted {
					if err := v.deleteTask(t, deleteFile); err != nil {
						logger.Errorf("删除已完成任务 %s 失败: %v", t.ID, err)
					} else {
						count++
					}
				}
			}
			logger.Infof("已清除 %d 个完成任务", count)
		})
	}, "清除已完成")
	clearBtn.Importance = widget.LowImportance

	// 4. Delete All
	deleteBtn := widgets.NewButtonWithTooltip("", icons.ThemedClearAllHistoryIcon, func() {
		manualTasks, err := tasks.DAO.GetManualTasks()
		if err != nil || len(manualTasks) == 0 {
			return
		}

		// 检查是否有非订阅任务
		hasNormalTasks := len(manualTasks) > 0

		if !hasNormalTasks {
			return
		}

		v.showBatchDeleteDialog("清空列表", "确定要删除所有任务吗？", func(deleteFile bool) {
			// 1. 先停止所有
			// v.downloader.CancelAll() // 原有: CancelAll
			// 现在: 只停止手动任务
			runningTasks, _ := tasks.DAO.GetActiveManualTasks()
			for _, t := range runningTasks {
				v.downloader.Cancel(t)
			}
			
			// 2. 逐个删除
			var count int
			for _, t := range manualTasks {
				if err := v.deleteTask(t, deleteFile); err != nil {
					logger.Errorf("删除任务 %s 失败: %v", t.ID, err)
				} else {
					count++
				}
			}
			logger.Infof("已清空 %d 个任务", count)
		})
	}, "清空所有任务")
	deleteBtn.Importance = widget.LowImportance

	// 返回按钮列表，不需要 Spacer，因为全局 Toolbar 已经有了 Spacer
	return []fyne.CanvasObject{
		startBtn,
		stopBtn,
		widget.NewSeparator(),
		clearBtn,
		deleteBtn,
	}
}

// showBatchDeleteDialog 显示带文件删除选项的确认框
func (v *TaskListView) showBatchDeleteDialog(title, content string, onConfirm func(deleteFile bool)) {
	check := widget.NewCheck("同时删除本地文件", nil)
	// 默认选中删除文件，符合用户直觉
	// check.SetChecked(true) 

	// 内容容器：文本 + 复选框
	contentContainer := container.NewVBox(
		widget.NewLabel(content),
		check,
	)

	dlg := dialog.NewCustomConfirm(
		title,
		"确定",
		"取消",
		contentContainer,
		func(confirm bool) {
			if confirm {
				onConfirm(check.Checked)
			}
		},
		v.window,
	)
	dlg.Show()
}

// updateTask 更新单个任务的 UI
func (v *TaskListView) updateTask(task *download.DownloadTask) {
	// 必须在 UI 线程执行，防止 map 并发读写和 UI 调用错误
	fyne.Do(func() {
		// logger.Debugf("UI 收到任务进度更新: %s, Stage: %s", task.ID, task.Stage)
		v.refreshTask(task)
	})
}

// refreshTask 更新任务项
func (v *TaskListView) refreshTask(task *download.DownloadTask) *widgets.TaskItemWidget {
	item, exists := v.items[task.ID]
	if !exists || item == nil {
		// logger.Warnf("UI 刷新任务失败: 找不到任务项 %s", task.ID)
		return nil
	}
	
	// 这里我们需要将 download.DownloadTask 转换为 tasks.Task 或者只更新部分字段
	// widgets.TaskItemWidget.StatusLabel 等是公开的，可以直接更新
	
	// 更新状态标签
	statusText := fmt.Sprintf("%s • %s • %s/s • ETA: %s",
		task.Stage,
		utils.FormatBytes(task.TotalSize),
		utils.FormatBytes(task.Speed),
		utils.FormatDurationSeconds(float64(task.ETA)),
	)
	item.StatusLabel.SetText(statusText)

	// 更新进度条
	item.ProgressBar.SetValue(task.Progress / 100.0)

	return item
}

// Refresh 刷新任务列表（异步加载）
func (v *TaskListView) Refresh() {
	// 显示加载提示
	loadingLabel := widget.NewLabel("加载中...")
	loadingLabel.Alignment = fyne.TextAlignCenter
	loadingLabel.Importance = widget.LowImportance
	
	v.taskList.Objects = []fyne.CanvasObject{
		container.NewCenter(loadingLabel),
	}
	v.taskList.Refresh()

	// 异步加载任务数据
	go func() {
		allTasks, err := tasks.DAO.GetManualTasks()
		if err != nil {
			fyne.Do(func() {
				v.taskList.Objects = []fyne.CanvasObject{
					container.NewCenter(widget.NewLabel(fmt.Sprintf("加载失败: %v", err))),
				}
				v.taskList.Refresh()
			})
			logger.Errorf("[任务] 加载任务失败: %v", err.Error())
			return
		}

		// 同步活跃任务状态
		activeTaskIDs := v.downloader.GetActiveTaskIDs()
		activeTaskMap := make(map[string]bool)
		for _, id := range activeTaskIDs {
			activeTaskMap[id] = true
		}

		// 过滤和处理任务
		validIDs := make(map[string]bool)
		var displayTasks []*tasks.Task

		for _, t := range allTasks {
			if t.SubscriptionID == nil {
				validIDs[t.ID] = true
				displayTasks = append(displayTasks, t)
			}
		}

		// 回到UI线程更新界面
		fyne.Do(func() {
			var newUIObjects []fyne.CanvasObject

			// 移除无效项
			for id := range v.items {
				if !validIDs[id] {
					delete(v.items, id)
				}
			}

			for _, task := range displayTasks {
				// 检查是否已有挂件
				if item, exists := v.items[task.ID]; exists {
					// 更新按钮状态
					v.updateActions(item, task)
					newUIObjects = append(newUIObjects, item.Container, widget.NewSeparator())
				} else {
					// 创建新项
					newItem := v.createTaskItem(task)
					v.items[task.ID] = newItem
					newUIObjects = append(newUIObjects, newItem.Container, widget.NewSeparator())
				}
			}

			if len(newUIObjects) == 0 {
				v.taskList.Objects = []fyne.CanvasObject{
					container.NewCenter(widget.NewLabel("没有进行中的任务")),
				}
			} else {
				v.taskList.Objects = newUIObjects
			}

			v.taskList.Refresh()
		})
	}()
}

// OpenFolder 打开任务文件夹
func (v *TaskListView) OpenFolder(task *tasks.Task) error {
	// 构造 file URL
	u := &url.URL{
		Scheme: "file",
		Path:   filepath.Dir(task.TemplatePath),
	}
	widgets.SafeOpenURL(fyne.CurrentApp(), u)
	return nil
}

// updateStatusBar 更新底部状态栏的统计信息
func (v *TaskListView) updateStatusBar() {
	go func() {
		// 获取所有普通任务的统计信息
		stats, err := tasks.DAO.GetManualTasksStats()

		fyne.Do(func() {
			if err != nil {
				v.statusBar.SetText("统计数据加载失败")
				return
			}

			// 格式化状态栏文本
			statusText := fmt.Sprintf("总视频: %d  •  下载中: %d  •  排队: %d  •  已完成: %d  •  失败: %d",
				stats.Total,
				stats.Downloading,
				stats.Queued,
				stats.Completed,
				stats.Failed)

			v.statusBar.SetText(statusText)
		})
	}()
}

func (v *TaskListView) createTaskItem(task *tasks.Task) *widgets.TaskItemWidget {
	actions := v.createActions(task)
	return widgets.NewTaskItemWidget(task, actions)
}

func (v *TaskListView) createActions(task *tasks.Task) widgets.TaskActions {
	return widgets.TaskActions{
		OnStart: func() {
			if task.Status == tasks.StatusFailed && task.CookieFile != "" {
				if !utils.FileExists(task.CookieFile) {
					logger.Warnf("[任务] 检测到 Cookie 文件 %s 不存在，自动清除以使用全局配置", task.CookieFile)
					task.CookieFile = ""
					tasks.DAO.ClearCookieFile(task.ID)
				}
			}
			logger.Infof("下载任务启动: %s", task.ID)
			go v.downloader.Schedule(task, auth.GetAutherization())
		},
		OnCancel: func() {
			v.downloader.Cancel(task)
		},
		OnOpenFolder: func() {
			v.OpenFolder(task)
		},
		OnDelete: func() {
			chkDeleteFile := widget.NewCheck("同时删除本地文件", nil)
			content := container.NewVBox(
				widget.NewLabel("确定要删除此任务吗？此操作无法撤销。"),
				chkDeleteFile,
			)

			widgets.ShowConfirmWithContent("删除确认", content, func() {
				if err := v.deleteTask(task, chkDeleteFile.Checked); err != nil {
					widgets.ShowInformation("错误", err.Error(), v.window)
					return
				}
			}, v.window)
		},
	}
}

func (v *TaskListView) updateActions(item *widgets.TaskItemWidget, task *tasks.Task) {
	item.RefreshActions(task, v.createActions(task))
}

func (v *TaskListView) deleteTask(task *tasks.Task, deleteFile bool) error {
	v.downloader.Cancel(task)

	if err := tasks.DAO.DeleteTask(task.ID); err != nil {
		widgets.ShowInformation("错误", err.Error(), v.window)
		logger.Errorf("删除任务失败: %v", err)
		return fmt.Errorf("删除任务失败: %v", err.Error())
	}
	if deleteFile {
		deleteFiles(task)
	}

	// 删除成功后刷新整个任务列表
	v.Refresh()
	return nil
}

// refreshTaskItem 刷新单个任务项的 UI（根据 taskID 从数据库获取最新状态）
func (v *TaskListView) refreshTaskItem(taskID string) {
	// 必须在 UI 线程执行
	fyne.Do(func() {
		// 1. 从数据库获取最新任务信息
		task, err := tasks.DAO.GetTaskByID(taskID)
		if err != nil {
			// 任务不存在，移除
			if _, exists := v.items[taskID]; exists {
				delete(v.items, taskID)
				v.Refresh()
			}
			return
		}

		// 过滤订阅任务 (如果它变成了订阅任务，应该移除)
		if task.SubscriptionID != nil {
			if _, exists := v.items[taskID]; exists {
				delete(v.items, taskID)
				v.Refresh()
			}
			return
		}

		// 2. 检查 UI 中是否存在该任务项
		item, exists := v.items[taskID]
		if !exists {
			v.Refresh()
			return
		}

		// 3. 更新状态标签
		var statusText string
		
		// 尝试从 Downloader 获取实时运行时状态 (优先显示 Stage)
		rtTask := v.downloader.GetTask(taskID)
		if rtTask != nil && rtTask.Stage != "" {
			statusText = fmt.Sprintf("%s • %s • %s/s • ETA: %s", 
				rtTask.Stage, 
				utils.FormatBytes(rtTask.TotalSize), 
				utils.FormatBytes(rtTask.Speed),
				utils.FormatDurationSeconds(float64(rtTask.ETA)))
		} else {
			if task.Status == tasks.StatusFailed && task.ErrorMsg != "" {
				statusText = fmt.Sprintf("❌ %s", task.ErrorMsg)
			} else {
				statusText = fmt.Sprintf("%s • %s", task.Status.Name(), utils.FormatBytes(task.TotalSize))
			}
		}
		item.StatusLabel.SetText(statusText)

		// 4. 更新进度条
		item.ProgressBar.SetValue(task.Progress / 100.0)

		// 5. 更新按钮状态
		v.updateActions(item, task)

		// 6. 刷新容器
		item.Container.Refresh()
	})
}

func deleteFiles(task *tasks.Task) {
	if task.Status != tasks.StatusCompleted {
		basePath := task.ActualPath
		if basePath == "" {
			basePath = task.TemplatePath
		}

		if basePath != "" {
			dir := filepath.Dir(basePath)
			nameStart := filepath.Base(basePath)
			ext := filepath.Ext(nameStart)
			prefix := strings.TrimSuffix(nameStart, ext)

			pattern := filepath.Join(dir, prefix+"*")
			files, err := filepath.Glob(pattern)
			if err != nil {
				logger.Warnf("查找临时文件失败: %v", err)
			} else {
				for _, f := range files {
					// 检查是否是相关的临时文件
					// 1. 包含 .part
					// 2. 包含 .f+数字 (格式代码)
					// 3. 包含 .temp
					// 4. 字幕文件 (且包含相同前缀) - 通常字幕不会太大，删除也没关系，如果是取消任务，应该连带字幕一起删

					filename := filepath.Base(f)
					shouldDelete := false

					if strings.HasSuffix(filename, ".part") {
						shouldDelete = true
					} else if strings.Contains(filename, ".f") && (strings.HasSuffix(filename, ".mp4") || strings.HasSuffix(filename, ".m4a") || strings.HasSuffix(filename, ".webm")) {
						// 简单的启发式：包含 .fXXX.
						// 正则检查更准确: \.f[0-9]+\.
						if matched, _ := regexp.MatchString(`\.f[0-9]+\.`, filename); matched {
							shouldDelete = true
						}
					} else if strings.HasSuffix(filename, ".temp") {
						shouldDelete = true
					} else if strings.HasSuffix(filename, ".vtt") || strings.HasSuffix(filename, ".srt") || strings.HasSuffix(filename, ".ass") {
						// 删除关联字幕
						shouldDelete = true
					} else if strings.HasSuffix(filename, ".ytdl") {
						shouldDelete = true
					}

					if shouldDelete {
						logger.Debugf("删除残留文件: %s", f)
						if err := os.Remove(f); err != nil {
							logger.Warnf("删除文件失败 %s: %v", f, err)
						}
					}
				}
			}
		}
	} else {
		f := task.ActualPath
		logger.Debugf("删除文件: %s", f)
		if utils.FileExists(f) {
			err := os.Remove(f)
			if err != nil && !os.IsNotExist(err) {
				logger.Errorf("删除文件失败: %v", err)
			}
		}
	}
}
