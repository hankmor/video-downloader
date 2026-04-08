package download

import (
	"fmt"
	"sync"

	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/tasks"
)

// StateManager 任务状态管理器
// 职责：
// 1. 统一管理状态转换逻辑
// 2. 验证状态转换的合法性
// 3. 同步内存状态和数据库状态
// 4. 触发状态变更回调
type stateManager struct {
	mu sync.RWMutex

	// 状态变更回调函数
	onStateChanged func(taskID string, oldStatus, newStatus tasks.TaskStatus)

	// 状态转换规则映射
	// key: 当前状态, value: 允许转换到的状态列表
	allowedTransitions map[tasks.TaskStatus][]tasks.TaskStatus
}

// NewStateManager 创建状态管理器
func newStateManager() *stateManager {
	sm := &stateManager{
		allowedTransitions: make(map[tasks.TaskStatus][]tasks.TaskStatus),
	}

	// 初始化状态转换规则
	sm.initTransitionRules()

	return sm
}

// initTransitionRules 初始化状态转换规则
func (sm *stateManager) initTransitionRules() {
	// queued -> downloading (开始下载)
	sm.allowedTransitions[tasks.StatusQueued] = []tasks.TaskStatus{
		tasks.StatusDownloading,
		tasks.StatusCanceled, // 可以取消排队的任务
	}

	// downloading -> completed, failed, canceled
	sm.allowedTransitions[tasks.StatusDownloading] = []tasks.TaskStatus{
		tasks.StatusCompleted,
		tasks.StatusFailed,
		tasks.StatusCanceled,
	}

	// completed, failed, canceled 是终态，不允许再转换
	// 但为了灵活性，允许 canceled -> downloading (重新开始)
	sm.allowedTransitions[tasks.StatusCanceled] = []tasks.TaskStatus{
		tasks.StatusDownloading, // 允许重新开始已取消的任务
		tasks.StatusQueued,
	}

	// failed -> downloading (允许重试)
	sm.allowedTransitions[tasks.StatusFailed] = []tasks.TaskStatus{
		tasks.StatusDownloading,
		tasks.StatusQueued,
	}
}

// SetOnStateChanged 设置状态变更回调
func (sm *stateManager) SetOnStateChanged(callback func(taskID string, oldStatus, newStatus tasks.TaskStatus)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onStateChanged = callback
}

// CanTransition 检查状态转换是否合法
func (sm *stateManager) CanTransition(from, to tasks.TaskStatus) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 相同状态允许转换（幂等）
	if from == to {
		return true
	}

	allowed, exists := sm.allowedTransitions[from]
	if !exists {
		// 如果当前状态没有定义转换规则，默认不允许转换
		return false
	}

	for _, allowedStatus := range allowed {
		if allowedStatus == to {
			return true
		}
	}

	return false
}

// Transition 执行状态转换
// 参数：
//   - taskID: 任务ID
//   - from: 当前状态（从数据库或内存中获取）
//   - to: 目标状态
//   - updateDB: 是否更新数据库
//   - updateMemory: 更新内存状态的函数（可选）
//   - errorMsg: 错误信息（可选，用于失败状态）
//
// 返回：
//   - error: 如果转换不合法或更新失败，返回错误
func (sm *stateManager) Transition(
	taskID string,
	from tasks.TaskStatus,
	to tasks.TaskStatus,
	updateDB func(taskID string, status tasks.TaskStatus, errorMsg string) error,
	updateMemory func(status tasks.TaskStatus, errorMsg string),
	errorMsg string,
) error {
	if from == to {
		return nil
	}

	// 验证状态转换合法性
	if !sm.CanTransition(from, to) {
		return fmt.Errorf("不允许的状态转换: %s -> %s", from, to)
	}

	// 更新数据库状态
	if updateDB != nil {
		if err := updateDB(taskID, to, errorMsg); err != nil {
			logger.Errorf("[状态管理] 更新数据库状态失败: taskID=%s, status=%s, error=%v", taskID, to, err)
			return fmt.Errorf("更新数据库状态失败: %w", err)
		}
	}

	// 更新内存状态
	if updateMemory != nil {
		updateMemory(to, errorMsg)
	}

	// 触发状态变更回调
	sm.mu.RLock()
	callback := sm.onStateChanged
	sm.mu.RUnlock()

	if callback != nil {
		callback(taskID, from, to)
	}

	logger.Debugf("[状态管理] 状态转换成功: taskID=%s, %s -> %s", taskID, from, to)
	return nil
}

// TransitionWithValidation 执行状态转换（带验证）
// 先从数据库获取当前状态，然后执行转换
func (sm *stateManager) TransitionWithValidation(
	taskID string,
	to tasks.TaskStatus,
	getCurrentStatus func(taskID string) (tasks.TaskStatus, error),
	updateDB func(taskID string, status tasks.TaskStatus, errorMsg string) error,
	updateMemory func(status tasks.TaskStatus, errorMsg string),
	errorMsg string,
) error {
	// 获取当前状态
	currentStatus, err := getCurrentStatus(taskID)
	if err != nil {
		return fmt.Errorf("获取当前状态失败: %w", err)
	}

	// 执行转换
	return sm.Transition(taskID, currentStatus, to, updateDB, updateMemory, errorMsg)
}

// GetAllowedTransitions 获取允许的状态转换列表
func (sm *stateManager) GetAllowedTransitions(from tasks.TaskStatus) []tasks.TaskStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	allowed, exists := sm.allowedTransitions[from]
	if !exists {
		return []tasks.TaskStatus{}
	}

	// 返回副本，避免外部修改
	result := make([]tasks.TaskStatus, len(allowed))
	copy(result, allowed)
	return result
}

// IsTerminalState 检查是否为终态（不允许再转换的状态）
func (sm *stateManager) IsTerminalState(status tasks.TaskStatus) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	allowed, exists := sm.allowedTransitions[status]
	if !exists {
		return true // 没有定义转换规则，视为终态
	}

	// 如果只允许转换到相同状态，视为终态
	return len(allowed) == 0 || (len(allowed) == 1 && allowed[0] == status)
}
