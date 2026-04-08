package download

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hankmor/vdd/core/auth"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/utils"
)

// 定义特定错误
var (
	ErrPaused   = fmt.Errorf("下载已暂停")
	ErrCanceled = fmt.Errorf("下载已取消")
)

const (
	updateProgressDuring = 1 * time.Second
)

// Downloader 下载管理器
type Downloader struct {
	// 手动任务 (用户添加)
	manualActiveTasks  map[string]*DownloadContext
	manualWaitingTasks *quenedTasks

	// 订阅任务 (自动添加)
	subActiveTasks  map[string]*DownloadContext
	subWaitingTasks *quenedTasks

	progressListeners []OnProgress
	onStatusChanged   func(taskID string, status tasks.TaskStatus, errorMsg string) // 状态变更回调
	statusListeners   []func(taskID string, status tasks.TaskStatus, errorMsg string)

	mu       sync.RWMutex
	stateMgr *stateManager
	core     *core
}

type quenedTasks struct {
	tasks map[string]*tasks.Task
	ids   []string
}

func newQuenedTasks() *quenedTasks {
	return &quenedTasks{
		tasks: make(map[string]*tasks.Task),
		ids:   make([]string, 0),
	}
}

func (qt *quenedTasks) Push(task *tasks.Task) {
	if _, exists := qt.tasks[task.ID]; exists {
		return
	}
	qt.tasks[task.ID] = task
	qt.ids = append(qt.ids, task.ID)
}

func (qt *quenedTasks) Pop() *tasks.Task {
	if len(qt.tasks) == 0 {
		return nil
	}
	id := qt.ids[0]
	task := qt.tasks[id]
	delete(qt.tasks, id)
	qt.ids = qt.ids[1:]
	return task
}

func (qt *quenedTasks) Get(taskID string) *tasks.Task {
	return qt.tasks[taskID]
}

func (qt *quenedTasks) Remove(taskID string) {
	if _, exists := qt.tasks[taskID]; !exists {
		return
	}
	delete(qt.tasks, taskID)
	for i, id := range qt.ids {
		if id == taskID {
			qt.ids = append(qt.ids[:i], qt.ids[i+1:]...)
			break
		}
	}
}

func (qt *quenedTasks) Clear() {
	qt.tasks = make(map[string]*tasks.Task)
	qt.ids = make([]string, 0)
}

func (qt *quenedTasks) Size() int {
	return len(qt.tasks)
}

func (qt *quenedTasks) UpdateStatus(taskID string, status tasks.TaskStatus, errorMsg string) {
	if qt.Get(taskID) != nil {
		qt.tasks[taskID].Status = status
		qt.tasks[taskID].ErrorMsg = errorMsg
	}
}

// DownloadContext 单个下载任务的上下文
type DownloadContext struct {
	Context    context.Context
	Task       *DownloadTask
	Cmd        *exec.Cmd
	CancelFunc context.CancelFunc
	LastSync   time.Time // 上次同步到数据库的时间
}

// New 创建下载管理器
func New(ytdlpPath string) *Downloader {
	ffmpegPath := resolveFFmpegPath()
	dd := &Downloader{
		core:               newCore(ytdlpPath, ffmpegPath),
		manualActiveTasks:  make(map[string]*DownloadContext),
		manualWaitingTasks: newQuenedTasks(),
		subActiveTasks:     make(map[string]*DownloadContext),
		subWaitingTasks:    newQuenedTasks(),
		stateMgr:           newStateManager(),
		statusListeners:    make([]func(taskID string, status tasks.TaskStatus, errorMsg string), 0),
		progressListeners:  make([]OnProgress, 0),
	}

	dd.onStatusChanged = func(taskID string, status tasks.TaskStatus, errorMsg string) {
		task, err := tasks.DAO.GetTaskByID(taskID)
		if err != nil {
			logger.Errorf("[下载代理] 获取任务状态失败: %v", err)
			return
		}

		err = dd.stateMgr.Transition(
			taskID,
			task.Status, // 当前状态
			status,      // 目标状态
			func(taskID string, status tasks.TaskStatus, errorMsg string) error {
				return tasks.DAO.UpdateStatusAndError(taskID, status, errorMsg)
			},
			func(status tasks.TaskStatus, errorMsg string) {
				// 更新内存映射
				// 此处需要更新 Content 中的 Task 对象，但不知道它在哪个 map 中，所以两个都尝试检查
				if ctx, ok := dd.manualActiveTasks[taskID]; ok {
					ctx.Task.Status = status
					ctx.Task.ErrorMessage = errorMsg
				} else if ctx, ok := dd.subActiveTasks[taskID]; ok {
					ctx.Task.Status = status
					ctx.Task.ErrorMessage = errorMsg
				}

				dd.manualWaitingTasks.UpdateStatus(taskID, status, errorMsg)
				dd.subWaitingTasks.UpdateStatus(taskID, status, errorMsg)
			},
			errorMsg,
		)
		if err != nil {
			logger.Errorf("[下载代理] 状态转换失败: %v", err)
		} else {
			// 通知所有监听器
			for _, listener := range dd.statusListeners {
				listener(taskID, status, errorMsg)
			}
		}
	}
	return dd
}

// DownloadTask 下载任务状态
type DownloadTask struct {
	*tasks.Task
	Speed        int64  // bytes/s
	Downloaded   int64  // bytes
	ETA          int    // seconds
	Stage        string // 当前阶段描述，如 "正在下载视频"
	ErrorMessage string // 错误信息
}

func NewDownloadTask(task *tasks.Task) *DownloadTask {
	return &DownloadTask{
		Task:  task,
		Stage: "准备下载...",
	}
}

// OnProgress 进度回调函数
type OnProgress func(task *DownloadTask)

// SetNotifyStatusChanged 设置状态变更通知 (Legacy support, appends listener)
func (m *Downloader) SetNotifyStatusChanged(notify func(taskID string, status tasks.TaskStatus, errorMsg string)) {
	m.AddNotifyStatusChanged(notify)
}

// AddNotifyStatusChanged 添加状态变更监听器
func (m *Downloader) AddNotifyStatusChanged(notify func(taskID string, status tasks.TaskStatus, errorMsg string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusListeners = append(m.statusListeners, notify)
}

// CancelTaskList 取消所有活跃任务 (仅手动)
func (m *Downloader) CancelTaskList() {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Infof("[Downloader] CancelAll called (Manual Tasks)")

	// 取消活跃手动任务
	for _, ctx := range m.manualActiveTasks {
		if ctx.Task.Status == tasks.StatusDownloading || ctx.Task.Status == tasks.StatusQueued {
			go m.doCancel(ctx.Task.Task, false)
		}
	}

	// 取消等待手动任务
	for _, task := range m.manualWaitingTasks.tasks {
		go m.cancelWaiting(task)
	}
}

// CancelAllSubscriptions 取消所有订阅任务
func (m *Downloader) CancelAllSubscriptions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Infof("[Downloader] CancelAllSubscriptions called")

	// 取消活跃订阅任务
	for _, ctx := range m.subActiveTasks {
		if ctx.Task.Status == tasks.StatusDownloading || ctx.Task.Status == tasks.StatusQueued {
			go m.doCancel(ctx.Task.Task, false)
		}
	}

	// 取消等待订阅任务
	for _, task := range m.subWaitingTasks.tasks {
		go m.cancelWaiting(task)
	}
}

// CancelSubscription 取消特定订阅的所有任务 (原子操作)
func (m *Downloader) CancelSubscription(subID uint) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Infof("[Downloader] CancelSubscription called for subID: %d", subID)
	count := 0

	// 1. 取消活跃中的任务
	// 注意：不能在遍历 map 时修改 map (doCancel 会修改 map)，所以先收集 ID
	var activeIDs []string
	for id, ctx := range m.subActiveTasks {
		if ctx.Task.SubscriptionID != nil && *ctx.Task.SubscriptionID == subID {
			if ctx.Task.Status == tasks.StatusDownloading || ctx.Task.Status == tasks.StatusQueued {
				activeIDs = append(activeIDs, id)
			}
		}
	}

	for _, id := range activeIDs {
		// 这里我们需要手动逻辑，因为我们已经持有锁，不能调用 doCancel (会死锁)
		// 复制 doCancel 的核心逻辑
		ctx := m.subActiveTasks[id]
		delete(m.subActiveTasks, id)

		ctx.CancelFunc()
		oldStatus := ctx.Task.Status

		// 异步通知状态变更 (避免在锁内做复杂操作)
		go func(tid string, oldS tasks.TaskStatus) {
			logger.Debugf("[下载] 任务 %s 下载取消 (Batch)", tid)
			m.onStatusChanged(tid, tasks.StatusCanceled, "")
		}(id, oldStatus)

		count++
	}

	// 2. 取消等待队列中的任务
	var waitingIDs []string
	for id, task := range m.subWaitingTasks.tasks {
		if task.SubscriptionID != nil && *task.SubscriptionID == subID {
			waitingIDs = append(waitingIDs, id)
		}
	}

	for _, id := range waitingIDs {
		m.subWaitingTasks.Remove(id)
		go m.onStatusChanged(id, tasks.StatusCanceled, "")
		count++
	}
	return count
}

// HasDownloadingTask 检查是否有正在进行的下载 (任意)
func (m *Downloader) HasDownloadingTask() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ctx := range m.manualActiveTasks {
		if ctx.Task.Status == tasks.StatusDownloading {
			return true
		}
	}
	for _, ctx := range m.subActiveTasks {
		if ctx.Task.Status == tasks.StatusDownloading {
			return true
		}
	}
	return false
}

// AddProgressListener 添加进度监听器
func (m *Downloader) AddProgressListener(callback OnProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressListeners = append(m.progressListeners, callback)
}

// dispatchProgress 分发进度通知
func (m *Downloader) dispatchProgress(task *DownloadTask) {
	for _, listener := range m.progressListeners {
		listener(task)
	}
}

// GetTask 获取指定任务状态
func (m *Downloader) GetTask(taskID string) *DownloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if ctx, ok := m.manualActiveTasks[taskID]; ok {
		taskCopy := *ctx.Task
		return &taskCopy
	}
	if ctx, ok := m.subActiveTasks[taskID]; ok {
		taskCopy := *ctx.Task
		return &taskCopy
	}
	return nil
}

// GetActiveTaskIDs 获取所有活跃任务的ID列表（用于状态同步）
func (m *Downloader) GetActiveTaskIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.manualActiveTasks)+len(m.subActiveTasks))
	for id := range m.manualActiveTasks {
		ids = append(ids, id)
	}
	for id := range m.subActiveTasks {
		ids = append(ids, id)
	}
	return ids
}

func (m *Downloader) IsMergeToMP4() bool {
	return m.core.isMergeToMP4()
}

func (m *Downloader) createDownloadContext(task *tasks.Task) *DownloadContext {
	dtask := &DownloadTask{
		Task:  task,
		Stage: "准备下载...",
	}

	ctx, cancel := context.WithCancel(context.Background())
	downloadCtx := &DownloadContext{
		Cmd:        nil,
		Context:    ctx,
		Task:       dtask,
		CancelFunc: cancel,
	}
	return downloadCtx
}

func (m *Downloader) doDownload(ctx *DownloadContext, authInfo auth.AuthInfo) error {
	logger.Debugf("[下载] 开始下载任务 %s", ctx.Task.ID)

	if !ctx.Task.CanSchedule() {
		return fmt.Errorf("当前任务不可下载")
	}

	m.onStatusChanged(ctx.Task.ID, tasks.StatusDownloading, "")

	// 确保退出时清理
	defer func() {
		ctx.CancelFunc()

		m.mu.Lock()
		// 确定从哪个映射中移除
		isManual := ctx.Task.SubscriptionID == nil

		var exists bool
		var currentCtx *DownloadContext

		if isManual {
			currentCtx, exists = m.manualActiveTasks[ctx.Task.ID]
			if exists && currentCtx.Context == ctx.Context {
				delete(m.manualActiveTasks, ctx.Task.ID)
			}
		} else {
			currentCtx, exists = m.subActiveTasks[ctx.Task.ID]
			if exists && currentCtx.Context == ctx.Context {
				delete(m.subActiveTasks, ctx.Task.ID)
			}
		}
		m.mu.Unlock()
	}()

	cmd, err := m.core.buildCmd(ctx, &authInfo)
	if err != nil {
		return err
	}

	ctx.Cmd = cmd

	// 执行cmd
	m.core.startCmd(ctx,
		// 更新任务阶段
		func(taskID, stage string) error {
			m.updateTaskStage(taskID, stage)
			return nil
		},
		// 更新任务进度
		func(taskID string, progress *Progress) {
			m.updateTaskProgress(taskID, progress)
		},
		// 下载完成
		func(taskID string, filename string) {
			logger.Debugf("[下载] 任务 %s 下载完成, 文件名: %s", ctx.Task.ID, filename)

			// 尝试更新文件大小
			if filename != "" {
				// 如果文件名不是绝对路径，且有输出目录，则拼接
				fullPath := filename
				if !filepath.IsAbs(fullPath) && ctx.Task.OutputFolder != "" {
					fullPath = filepath.Join(ctx.Task.OutputFolder, fullPath)
				}

				if info, err := os.Stat(fullPath); err == nil {
					if err := tasks.DAO.UpdateTaskSize(ctx.Task.ID, info.Size()); err != nil {
						logger.Errorf("[下载] 更新任务 %s 文件大小失败: %v", ctx.Task.ID, err)
					} else {
						logger.Debugf("[下载] 已更新任务 %s 文件大小为: %d", ctx.Task.ID, info.Size())
					}

					// 更新实际路径（订阅任务特别需要）
					if err := tasks.DAO.UpdateTaskPath(ctx.Task.ID, fullPath); err != nil {
						logger.Errorf("[下载] 更新任务 %s 文件路径失败: %v", ctx.Task.ID, err)
					}
				} else {
					logger.Warnf("[下载] 无法获取文件大小 %s: %v", fullPath, err)
				}
			}

			m.onStatusChanged(ctx.Task.ID, tasks.StatusCompleted, "")
		},
		func(taskID string) {
			logger.Debugf("[下载] 任务 %s 下载取消", ctx.Task.ID)
			m.onStatusChanged(ctx.Task.ID, tasks.StatusCanceled, "")
		},
		// 下载失败
		func(taskID string, err error) {
			// 智能错误翻译
			showError := humanReadableError(err.Error())
			if showError != "" {
				logger.Debugf("[下载] 任务 %s 下载失败: %v, 详细: %s", ctx.Task.ID, err, showError)
				m.onStatusChanged(ctx.Task.ID, tasks.StatusFailed, showError)
			} else {
				logger.Debugf("[下载] 任务 %s 下载失败: %v, 详细: %s", ctx.Task.ID, err, err.Error())
				m.onStatusChanged(ctx.Task.ID, tasks.StatusFailed, err.Error())
			}
		})
	return nil
}

func humanReadableError(detailError string) string {
	// ... (Existing error mapping logic is fine, can effectively copy paste or reuse, but for rewriting I need to include it)
	// For brevity in prompt I will not expand all if possible, but I must provide valid Go code.
	// I'll copy the existing logic.

	var showError = ""
	if strings.Contains(detailError, "premium member") {
		showError = "下载失败: 该画质需要大会员权限, 请检查Cookie设置或降低画质"
	} else if strings.Contains(detailError, "Requested format is not available") {
		showError = "下载失败: 请求的画质不可用, 可能需要会员权限"
	} else if strings.Contains(detailError, "Sign in to confirm") {
		showError = "下载失败: 该视频需登录验证, 请检查Cookie设置"
	} else if strings.Contains(detailError, "HTTP Error 403: Forbidden") {
		showError = "下载失败: 访问被拒绝, 可能是Cookie失效或IP问题"
	} else if strings.Contains(detailError, "Connection refused") || strings.Contains(detailError, "Connection reset") ||
		strings.Contains(detailError, "Connection timed out") || strings.Contains(detailError, "Read timed out") ||
		strings.Contains(detailError, "Connection aborted") {
		showError = "下载失败: 网络连接不稳定或被重置, 请检查代理设置"
	} else if strings.Contains(detailError, "This video is available to") && strings.Contains(detailError, "members only") {
		// YouTube 频道会员限定
		showError = "下载失败: 该视频仅限频道会员观看, 需要提供有权限的Cookie"
	} else if strings.Contains(detailError, "Private video") {
		showError = "下载失败: 这是一个私有视频, 您没有权限访问"
	} else if strings.Contains(detailError, "Video unavailable") || strings.Contains(detailError, "This video has been removed") {
		showError = "下载失败: 视频已失效或被删除"
	} else if strings.Contains(detailError, "uploaded by the uploader") && strings.Contains(detailError, "is not available in your country") {
		showError = "下载失败: 该视频在您的地区不可用 (地区限制)"
	} else if strings.Contains(detailError, "Join this channel to get access") {
		showError = "下载失败: 需要加入频道会员才能观看此视频"
	} else if strings.Contains(detailError, "This live event will begin in") {
		showError = "下载失败: 直播尚未开始"
	} else if strings.Contains(detailError, "is offline") {
		showError = "下载失败: 直播已结束或不在线"
	} else if strings.Contains(detailError, "account has been terminated") {
		showError = "下载失败: 发布者账号已被封禁"
	} else if strings.Contains(detailError, "ffmpeg not found") || strings.Contains(detailError, "ffprobe not found") {
		showError = "下载失败: 未找到 FFmpeg, 请在设置中指定正确的路径"
	} else if strings.Contains(detailError, "No space left on device") {
		showError = "下载失败: 磁盘空间不足"
	} else if strings.Contains(detailError, "Permission denied") {
		showError = "下载失败: 无法写入文件 (权限拒绝), 请检查下载目录权限"
	} else if strings.Contains(detailError, "Read-only file system") {
		showError = "下载失败: 无法写入文件 (只读文件系统)"
	} else if strings.Contains(detailError, "File name too long") {
		showError = "下载失败: 文件名过长, 请尝试更改文件名格式"
	} else if strings.Contains(detailError, "Unsupported URL") {
		showError = "下载失败: 不支持的视频链接"
	} else if strings.Contains(detailError, "Playlist does not exist") {
		showError = "下载失败: 播放列表不存在或为空"
	}

	logger.Debugf("转为友好错误信息: %s -> %s", detailError, showError)

	if showError != "" {
		return showError
	}
	return detailError
}

// resolveFFmpegPath 解析 FFmpeg 路径
func resolveFFmpegPath() string {
	configPath := config.Get().FFmpegPath

	if configPath != "" {
		logger.Debugf("[下载] 正在解析 ffmpeg 路径 %s", configPath)
		if filepath.IsAbs(configPath) && fileExists(configPath) {
			return configPath
		}
		if path, err := exec.LookPath(configPath); err == nil {
			return path
		}
	} else {
		ffmpegPath := utils.GetFFmpegPath()
		logger.Debugf("[下载] 正在解析 ffmpeg 路径 %s", ffmpegPath)
		if filepath.IsAbs(ffmpegPath) && fileExists(ffmpegPath) {
			return ffmpegPath
		}
		if path, err := exec.LookPath(ffmpegPath); err == nil {
			return path
		}
	}
	logger.Warnf("[下载] 未找到有效的 ffmpeg 路径，合并功能可能不可用")
	return ""
}

// updateTaskStage 更新任务阶段
func (m *Downloader) updateTaskStage(taskID, stage string) {
	logger.Debugf("[下载] 正在更新任务阶段: %s, stage: %s", taskID, stage)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 尝试手动任务
	if ctx, ok := m.manualActiveTasks[taskID]; ok {
		ctx.Task.Stage = stage
		m.dispatchProgress(ctx.Task)
		return
	}
	// 尝试订阅任务
	if ctx, ok := m.subActiveTasks[taskID]; ok {
		ctx.Task.Stage = stage
		m.dispatchProgress(ctx.Task)
		return
	}
}

// updateTaskProgress 更新任务进度
func (m *Downloader) updateTaskProgress(taskID string, p *Progress) {
	logger.Debugf("[下载] 正在更新任务进度: %s", taskID)

	m.mu.Lock()
	defer m.mu.Unlock()

	var ctx *DownloadContext
	var ok bool

	// 查找上下文辅助函数 (尝试避免代码重复，但锁范围很棘手)
	if c, ex := m.manualActiveTasks[taskID]; ex {
		ctx = c
		ok = true
	} else if c, ex := m.subActiveTasks[taskID]; ex {
		ctx = c
		ok = true
	}

	if ok {
		t := ctx.Task
		t.Progress = p.Percent
		t.Speed = p.Speed
		t.Downloaded = p.Downloaded
		t.TotalSize = p.Total
		t.ETA = p.ETA
		t.UpdatedAt = time.Now()

		if t.Stage == "" {
			t.Stage = "下载中..."
		}

		// 同步进度到数据库 (每 5 秒一次)
		// 避免频繁写入导致 IO 压力
		if time.Since(ctx.LastSync) >= updateProgressDuring || t.Progress >= 100 {
			ctx.LastSync = time.Now()
			go func(tid string, prog float64) {
				if err := tasks.DAO.UpdateProgress(tid, prog); err != nil {
					logger.Errorf("Failed to sync progress to DB for task %s: %v", tid, err)
				}
			}(taskID, p.Percent)
		}

		m.dispatchProgress(ctx.Task)
	}
}

// Schedule 请求调度下载
// 如果当前并发数 < MaxConcurrent，则立即开始下载（异步）
// 否则加入等待队列，状态置为 Queued
func (m *Downloader) Schedule(task *tasks.Task, authInfo auth.AuthInfo, allDownloadingReady ...func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	isManual := task.SubscriptionID == nil

	// 检查是否正在运行
	if _, ok := m.manualActiveTasks[task.ID]; ok {
		return fmt.Errorf("任务 %s 正在运行中 (Manual)", task.ID)
	}
	if _, ok := m.subActiveTasks[task.ID]; ok {
		return fmt.Errorf("任务 %s 正在运行中 (Sub)", task.ID)
	}

	// 检查是否在等待
	if m.manualWaitingTasks.Get(task.ID) != nil || m.subWaitingTasks.Get(task.ID) != nil {
		return fmt.Errorf("任务 %s 已经在排队中", task.ID)
	}

	maxConcurrent := config.Get().MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	currentRunning := len(m.manualActiveTasks) + len(m.subActiveTasks)

	// 是否可以立即开始?
	if currentRunning < maxConcurrent {
		ctx := m.createDownloadContext(task)
		if isManual {
			m.manualActiveTasks[task.ID] = ctx
		} else {
			m.subActiveTasks[task.ID] = ctx
		}

		// 同步调用，等待数据库状态更新，避免状态不同步, 实际的命令执行是异步的
		m.download(ctx, authInfo)
		return nil
	}

	// 所有可以开始下载的处理完成，直接回调，不用等到调度队列状态更新成功
	if len(allDownloadingReady) > 0 {
		allDownloadingReady[0]()
	}

	// 加入队列
	if isManual {
		m.manualWaitingTasks.Push(task)
		logger.Infof("[调度器] 手动任务 %s 加入队列", task.ID)
	} else {
		m.subWaitingTasks.Push(task)
		logger.Infof("[调度器] 订阅任务 %s 加入队列", task.ID)
	}

	go m.onStatusChanged(task.ID, tasks.StatusQueued, "")
	return nil
}

// download 内部执行下载，并在结束后触发调度
func (m *Downloader) download(ctx *DownloadContext, authInfo auth.AuthInfo) {
	// 1. 检查下载权限 (每日限额)
	canDl, err := auth.CanDownload()
	if !canDl {
		logger.Warnf("[下载] 每日限额已达，取消任务 %s", ctx.Task.ID)
		m.onStatusChanged(ctx.Task.ID, tasks.StatusFailed, err.Error())
		// 不需要再次调度了
		// m.scheduleNext()
		return
	}

	// 2. 如有需要，记录下载次数
	if authInfo.UserTrialDaysExpired() {
		auth.IncrementTodayDownloadCount()
	}

	go func() {
		// 调用现有的阻塞 Download 方法
		err = m.doDownload(ctx, authInfo)
		if err != nil {
			logger.Debugf("[下载] 任务 %s 执行结束: %v", ctx.Task.ID, err)
		}

		// 下载结束（无论成功失败），尝试调度下一个
		m.scheduleNext()
	}()
}

// scheduleNext 调度下一个任务
func (m *Downloader) scheduleNext() {
	m.mu.Lock()
	defer m.mu.Unlock()

	maxConcurrent := config.Get().MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	currentRunning := len(m.manualActiveTasks) + len(m.subActiveTasks)
	if currentRunning >= maxConcurrent {
		return
	}

	// 优先级: 手动 > 订阅

	var next *tasks.Task
	var isManual bool

	if m.manualWaitingTasks.Size() > 0 {
		next = m.manualWaitingTasks.Pop()
		isManual = true
	} else if m.subWaitingTasks.Size() > 0 {
		next = m.subWaitingTasks.Pop()
		isManual = false
	} else {
		return
	}

	logger.Infof("[调度器] 正在从队列启动任务 %s (Manual=%v)", next.ID, isManual)

	go func(tid string, manual bool) {
		task, err := tasks.DAO.GetTaskByID(tid)
		if err != nil {
			logger.Errorf("[调度器] 无法读取排队任务 %s: %v", tid, err)
			return
		}

		token := auth.GetAutherization()

		// 需要再次加锁以插入活跃映射
		m.mu.Lock()
		ctx := m.createDownloadContext(task)
		if manual {
			m.manualActiveTasks[task.ID] = ctx
		} else {
			m.subActiveTasks[task.ID] = ctx
		}
		m.mu.Unlock()

		m.download(ctx, token)
	}(next.ID, isManual)
}

func (m *Downloader) StartAll(taskList []*tasks.Task) {
	token := auth.GetAutherization()
	for _, t := range taskList {
		if t.Status != tasks.StatusDownloading && t.Status != tasks.StatusCompleted && t.Status != tasks.StatusQueued {
			m.Schedule(t, token)
		}
	}
}

// StartAllWithCallback 批量开始任务，完成后调用回调
func (m *Downloader) StartAllWithCallback(taskList []*tasks.Task, onComplete func()) {
	if len(taskList) == 0 {
		return
	}

	token := auth.GetAutherization()
	for _, t := range taskList {
		if t.Status != tasks.StatusDownloading && t.Status != tasks.StatusCompleted && t.Status != tasks.StatusQueued {
			task := t // capture
			m.Schedule(task, token, func() {
				onComplete()
			})
		}
	}
}

// updateTaskStatusInMemeory 更新任务状态（仅更新内存状态）
func (m *Downloader) updateTaskStatusInMemeory(taskID string, status tasks.TaskStatus, errMsg string) {
	logger.Debugf("[下载] 正在更新任务状态: %s, status: %s, errMsg: %s", taskID, status, errMsg)

	m.mu.Lock()
	defer m.mu.Unlock()

	var ctx *DownloadContext
	if c, ok := m.manualActiveTasks[taskID]; ok {
		ctx = c
	} else if c, ok := m.subActiveTasks[taskID]; ok {
		ctx = c
	}

	if ctx != nil {
		ctx.Task.Status = status
		if errMsg != "" {
			ctx.Task.ErrorMessage = errMsg
		}
		ctx.Task.UpdatedAt = time.Now()
		m.dispatchProgress(ctx.Task)
	}
}

func (m *Downloader) Cancel(task *tasks.Task) error {
	// 需要检查任务位置
	if task.Status == tasks.StatusDownloading {
		// 需要找到它在哪个 map 中以便安全取消，但 doCancel 如果包含上下文查找会更简单
		// doCancel 负责从 map 中移除
		return m.doCancel(task, true)
	} else if task.Status == tasks.StatusQueued {
		return m.cancelWaiting(task)
	}
	return nil
}

func (m *Downloader) doCancel(task *tasks.Task, scheduleNext bool) error {
	logger.Debugf("[下载] 取消任务 %s", task.ID)
	m.mu.Lock()
	defer m.mu.Unlock()

	// 首先尝试从等待队列移除 (以防状态不匹配)
	m.manualWaitingTasks.Remove(task.ID)
	m.subWaitingTasks.Remove(task.ID)

	// 尝试活跃任务
	var ctx *DownloadContext
	var ok bool
	isManual := true

	if c, ex := m.manualActiveTasks[task.ID]; ex {
		ctx = c
		ok = true
	} else if c, ex := m.subActiveTasks[task.ID]; ex {
		ctx = c
		ok = true
		isManual = false
	}

	if !ok {
		// 如果在活跃任务中未找到，不处理 (直接更新状态)
		// return fmt.Errorf("task %s not found in active tasks", task.ID)
		// 如果未找到，我们只是在数据库更新状态并通知
	} else {
		// 在活跃任务中找到
		if isManual {
			delete(m.manualActiveTasks, task.ID)
		} else {
			delete(m.subActiveTasks, task.ID)
		}
		ctx.CancelFunc()
		oldStatus := ctx.Task.Status
		ctx.Task.Status = tasks.StatusCanceled
		ctx.Task.Stage = "已取消"
		logger.Debugf("[下载] 通知状态变更: %s, status: %s -> %s", task.ID, oldStatus, tasks.StatusCanceled)
	}

	m.onStatusChanged(task.ID, tasks.StatusCanceled, "")

	if scheduleNext {
		go m.scheduleNext()
	}
	return nil
}

func (m *Downloader) cancelWaiting(task *tasks.Task) error {
	logger.Debugf("[下载] 取消等待的任务 %s", task.ID)
	m.mu.Lock()
	defer m.mu.Unlock()

	m.manualWaitingTasks.Remove(task.ID)
	m.subWaitingTasks.Remove(task.ID)

	m.onStatusChanged(task.ID, tasks.StatusCanceled, "")
	return nil
}

// StopAll 停止所有正在进行的任务（用于程序退出）
func (m *Downloader) StopAll() {
	logger.Info("[下载] 正在停止所有下载任务...")
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 取消所有手动任务
	count := 0
	for _, ctx := range m.manualActiveTasks {
		if ctx.CancelFunc != nil {
			ctx.CancelFunc()
			count++
		}
	}
	// 2. 取消所有订阅任务
	for _, ctx := range m.subActiveTasks {
		if ctx.CancelFunc != nil {
			ctx.CancelFunc()
			count++
		}
	}

	// 3. 清空等待队列
	m.manualWaitingTasks.Clear()
	m.subWaitingTasks.Clear()

	logger.Infof("[下载] 已触发 %d 个任务的取消操作", count)
}

// TriggerSchedule 触发任务调度（用于配置变更时立即生效，如增加并发数）
func (m *Downloader) TriggerSchedule() {
	logger.Debug("[下载] 配置变更，手动触发任务调度")
	// 异步执行，避免阻塞 UI
	go m.scheduleNext()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
