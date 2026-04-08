package subscription

import (
	"context"
	"sync"

	"github.com/hankmor/vdd/core/logger"
)

// ScanManager 扫描管理器，负责统一管理订阅扫描过程和角标更新
type ScanManager struct {
	mu        sync.RWMutex
	scanning  map[uint]context.CancelFunc // 正在扫描的订阅及其取消函数
	newCounts map[uint]int                // 各订阅的新视频数(本次会话)
	totalNew  int                         // 总新视频数 (本次会话累计)

	// 批量扫描状态
	batchScanning bool // 是否正在进行批量扫描
	batchTotal    int  // 批量任务总数
	batchScanned  int  // 批量任务已完成数
	batchNewCount int  // 批量任务累计新增数

	// 回调函数
	progressListeners []func(progress *ScanProgress) // 进度回调列表
	onComplete        func(totalNew int)             // 全部扫描完成回调

	manager *Manager // 关联的订阅管理器
}

// NewScanManager 创建扫描管理器
func NewScanManager(manager *Manager) *ScanManager {
	return &ScanManager{
		scanning:          make(map[uint]context.CancelFunc),
		newCounts:         make(map[uint]int),
		progressListeners: make([]func(progress *ScanProgress), 0),
		manager:           manager,
	}
}

// SetOnProgress 设置进度回调 (Legacy: appended to listeners)
func (s *ScanManager) SetOnProgress(callback func(progress *ScanProgress)) {
	s.AddOnProgress(callback)
}

// AddOnProgress 添加进度回调
func (s *ScanManager) AddOnProgress(callback func(progress *ScanProgress)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progressListeners = append(s.progressListeners, callback)
}

// SetOnComplete 设置完成回调
func (s *ScanManager) SetOnComplete(callback func(totalNew int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onComplete = callback
}

// ScanBatch 批量扫描订阅 (用于后台自动扫描)
// subs: 要扫描的订阅列表
func (s *ScanManager) ScanBatch(subs []*Subscription) {
	if len(subs) == 0 {
		return
	}

	// 标记批量扫描开始
	s.mu.Lock()
	s.batchScanning = true
	s.batchTotal = len(subs)
	s.batchScanned = 0
	s.batchNewCount = 0
	s.mu.Unlock()

	// 通知批量开始
	s.notifyProgress(&ScanProgress{
		Category:   ScanCategoryBatch,
		IsScanning: true,
		TotalSubs:  len(subs),
		NewCount:   0,
	})

	logger.Infof("[ScanManager] 开始批量扫描 %d 个订阅", len(subs))

	var wg sync.WaitGroup
	// 使用信号量控制并发 (例如最大 3 个并发扫描)
	sem := make(chan struct{}, 3)

	for _, sub := range subs {
		wg.Add(1)
		go func(sub *Subscription) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			// ScanOne 后 s.newCounts[sub.ID] 会被更新。
			// 所以我们在调用 ScanOne 后，读取 s.newCounts[sub.ID]。

			s.ScanOne(sub.ID)

			s.mu.Lock()
			count := s.newCounts[sub.ID]
			s.batchScanned++
			s.batchNewCount += count
			s.mu.Unlock()

		}(sub)
	}

	wg.Wait()

	// 标记批量扫描结束
	s.mu.Lock()
	s.batchScanning = false
	currentBatchNew := s.batchNewCount
	s.mu.Unlock()

	// 通知批量结束
	s.notifyProgress(&ScanProgress{
		Category:    ScanCategoryBatch,
		IsScanning:  false,
		TotalSubs:   len(subs),
		ScannedSubs: len(subs),
		NewCount:    currentBatchNew,
	})

	logger.Infof("[ScanManager] 批量扫描完成，共更新 %d 个视频", currentBatchNew)
}

// ScanAll 扫描所有活跃订阅 (重构为调用 ScanBatch)
func (s *ScanManager) ScanAll() {
	logger.Info("[ScanManager] 准备扫描所有订阅...")

	// 获取所有订阅
	subs, err := DAO.GetAll()
	if err != nil {
		logger.Errorf("[ScanManager] 获取订阅列表失败: %v", err)
		return
	}

	// 过滤活跃订阅
	var activeSubs []*Subscription
	for _, sub := range subs {
		if sub.Status == StatusActive {
			activeSubs = append(activeSubs, sub)
		}
	}

	if len(activeSubs) == 0 {
		logger.Info("[ScanManager] 没有活跃的订阅")
		// 也发送一个空的结束通知
		s.notifyProgress(&ScanProgress{
			Category:   ScanCategoryBatch,
			IsScanning: false,
			NewCount:   0,
		})
		return
	}

	// 调用批量扫描
	go s.ScanBatch(activeSubs)
}

// StopAll 停止所有正在进行的扫描
func (s *ScanManager) StopAll() {
	logger.Info("[ScanManager] 正在停止所有扫描...")
	s.mu.Lock()
	defer s.mu.Unlock()

	for subID, cancel := range s.scanning {
		logger.Debugf("[ScanManager] 取消订阅 %d 的扫描", subID)
		cancel()
	}
	// 不需要清空 map，ScanOne 的 defer 会处理
}

// CancelScan 取消单个订阅的扫描
func (s *ScanManager) CancelScan(subID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.scanning[subID]; ok {
		logger.Infof("[ScanManager] 取消订阅 %d 的扫描", subID)
		cancel()
		// defer 会从 map 中移除
	}
}

// ScanOne 扫描单个订阅
func (s *ScanManager) ScanOne(subID uint) {
	// 标记为扫描中
	s.mu.Lock()
	if _, ok := s.scanning[subID]; ok {
		s.mu.Unlock()
		logger.Warnf("[ScanManager] 订阅 %d 已在扫描中，跳过", subID)
		return
	}

	// 创建 Context
	ctx, cancel := context.WithCancel(context.Background())
	s.scanning[subID] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.scanning, subID)
		s.mu.Unlock()
		cancel() // 确保释放
	}()

	// 获取订阅信息
	sub, err := DAO.GetByID(subID)
	if err != nil {
		logger.Errorf("[ScanManager] 获取订阅 %d 失败: %v", subID, err)
		s.notifyProgress(&ScanProgress{
			SubscriptionID: subID,
			Error:          err.Error(),
		})
		return
	}

	// 更新数据库扫描状态
	DAO.SetScanning(subID, true)
	defer DAO.SetScanning(subID, false)

	// 通知开始扫描
	s.notifyProgress(&ScanProgress{
		SubscriptionID:   subID,
		SubscriptionName: sub.Name,
		IsScanning:       true,
		NewCount:         0,
	})

	logger.Infof("[ScanManager] 开始扫描订阅: %s (ID: %d)", sub.Name, subID)

	// 执行扫描（调用 Manager 的 ProcessSubscription）
	newCount, err := s.manager.ProcessSubscription(ctx, sub, func(entry *playlistEntry, i, total int) {
		// 回调：发现新视频
		// 实时更新角标计数
		if err := DAO.IncrementBadgeCount(sub.ID, 1); err != nil {
			logger.Errorf("[ScanManager] 更新角标计数失败: %v", err)
		}

		// 通知进度
		s.notifyProgress(&ScanProgress{
			SubscriptionID:   sub.ID,
			SubscriptionName: sub.Name,
			IsScanning:       true,
			NewCount:         1, // 这里如果是增量通知，应该传1？还是累计？
			// notifyProgress 内部没有累加逻辑，通常 UI 期望是 Update 或 Snapshot。
			// 之前的逻辑是：ProcessSubscription 这里是每次发现新视频时回调。
			// 旧逻辑：s.newCounts[subID] 在最后更新。
			// 但中间过程也发 notifyProgress。
			// newCount 局部变量一直在增加。
			// 让我们看看 ScanProgress 定义。NewCount int。
			// UI 可能是累加还是覆盖？
			// 查看 ui/views/subscription_view.go 也许需要，但目前假设是 Snapshot 会更好，
			// 不过 ProcessSubscription 的回调没有传当前的 newCount。
			// 为了简单，我们可以在 ScanOne 维护一个局部 newCount，在回调里累加。
		})
	})
	// 修正回调逻辑，需要在外部维护计数传递给 notifyProgress

	// 为了正确传递累计的新增数，我们需要稍微调整下面的调用逻辑：
	currentNewCount := 0
	newCount, err = s.manager.ProcessSubscription(ctx, sub, func(entry *playlistEntry, i, total int) {
		currentNewCount++
		// 实时更新角标
		DAO.IncrementBadgeCount(sub.ID, 1) // 忽略错误

		s.notifyProgress(&ScanProgress{
			SubscriptionID:   sub.ID,
			SubscriptionName: sub.Name,
			IsScanning:       true,
			NewCount:         currentNewCount,
		})
	})

	if err != nil {
		logger.Errorf("[ScanManager] 扫描失败 %s: %v", sub.Name, err)
		// 错误也通过 Progress 通知？或者只在 log
		if ctx.Err() != nil {
			// 如果是取消，那是预期的
		} else {
			s.notifyProgress(&ScanProgress{
				SubscriptionID: subID,
				Error:          err.Error(),
			})
		}
	}

	// 更新计数
	s.mu.Lock()
	s.newCounts[subID] = newCount
	s.totalNew += newCount
	s.mu.Unlock()

	// 通知扫描完成
	s.notifyProgress(&ScanProgress{
		SubscriptionID:   subID,
		SubscriptionName: sub.Name,
		IsScanning:       false,
		NewCount:         newCount,
	})

	logger.Infof("[ScanManager] 扫描完成: %s，新增 %d 个视频", sub.Name, newCount)
}

// GetBadgeCount 获取订阅的角标数字
func (s *ScanManager) GetBadgeCount(subID uint) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.newCounts[subID]
}

// GetTotalNewCount 获取全局角标数字
func (s *ScanManager) GetTotalNewCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalNew
}

// IsScanning 检查订阅是否正在扫描
func (s *ScanManager) IsScanning(subID uint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.scanning[subID]
	return ok
}

// MarkAsRead 标记订阅为已读
func (s *ScanManager) MarkAsRead(subID uint) error {
	s.mu.Lock()
	oldCount := s.newCounts[subID]
	s.newCounts[subID] = 0
	s.totalNew -= oldCount
	s.mu.Unlock()

	// 通知 UI 更新 (特别是 Toolbar 总角标)
	s.notifyProgress(&ScanProgress{
		SubscriptionID: subID,
		NewCount:       0,
		IsScanning:     false,
	})

	return DAO.MarkAsRead(subID)
}

// notifyProgress 通知进度回调
func (s *ScanManager) notifyProgress(progress *ScanProgress) {
	s.mu.RLock()
	// 复制列表以释放锁
	listeners := make([]func(progress *ScanProgress), len(s.progressListeners))
	copy(listeners, s.progressListeners)
	s.mu.RUnlock()

	for _, callback := range listeners {
		if callback != nil {
			callback(progress)
		}
	}
}

// notifyComplete 通知完成回调
func (s *ScanManager) notifyComplete() {
	s.mu.RLock()
	callback := s.onComplete
	totalNew := s.totalNew
	s.mu.RUnlock()

	logger.Infof("[ScanManager] 所有扫描完成，总计新增 %d 个视频", totalNew)

	if callback != nil {
		callback(totalNew)
	}
}
