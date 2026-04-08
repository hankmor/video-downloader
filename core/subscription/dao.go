package subscription

import (
	"time"

	"github.com/hankmor/vdd/core/db"
	"github.com/hankmor/vdd/core/tasks"
	"gorm.io/gorm"
)

var DAO = &subscriptionDAO{}

type subscriptionDAO struct{}

// Create 创建订阅
func (d *subscriptionDAO) Create(sub *Subscription) error {
	return db.GormDB.Create(sub).Error
}

// Delete 删除订阅
func (d *subscriptionDAO) Delete(id uint) error {
	return db.GormDB.Transaction(func(tx *gorm.DB) error {
		// 1. 删除关联的任务 (Tasks)
		// 使用 Unscoped 以确保物理删除（如果需要软删除则移除 Unscoped，但通常任务删了就是删了）
		if err := tx.Unscoped().Where("subscription_id = ?", id).Delete(&tasks.Task{}).Error; err != nil {
			return err
		}

		// 2. 删除相关的视频记录 (SubscriptionVideo)
		if err := tx.Where("subscription_id = ?", id).Delete(&SubscriptionVideo{}).Error; err != nil {
			return err
		}

		// 3. 删除订阅本身
		return tx.Delete(&Subscription{}, id).Error
	})
}

// GetAll 获取所有订阅
func (d *subscriptionDAO) GetAll() ([]*Subscription, error) {
	var subs []*Subscription
	err := db.GormDB.Order("created_at desc").Find(&subs).Error
	return subs, err
}

// GetByID 获取单个订阅
func (d *subscriptionDAO) GetByID(id uint) (*Subscription, error) {
	var sub Subscription
	err := db.GormDB.First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// UpdateLastCheck 更新最后检查时间
func (d *subscriptionDAO) UpdateLastCheck(id uint) error {
	return db.GormDB.Model(&Subscription{}).Where("id = ?", id).
		Update("last_check_at", time.Now()).Error
}

// UpdateStatus 更新状态
func (d *subscriptionDAO) UpdateStatus(id uint, status SubscriptionStatus) error {
	return db.GormDB.Model(&Subscription{}).Where("id = ?", id).
		Update("status", status).Error
}

// CountActiveSubscriptions 获取活跃订阅数量
func (d *subscriptionDAO) CountActiveSubscriptions() (int64, error) {
	var count int64
	err := db.GormDB.Model(&Subscription{}).Count(&count).Error
	return count, err
}

// --- SubscriptionVideo 相关 ---

// ExistsVideo 检查视频是否已存在于该订阅记录中
func (d *subscriptionDAO) ExistsVideo(subID uint, videoID string) (bool, error) {
	var count int64
	err := db.GormDB.Model(&SubscriptionVideo{}).
		Where("subscription_id = ? AND video_id = ?", subID, videoID).
		Count(&count).Error
	return count > 0, err
}

// AddVideo 记录新视频
func (d *subscriptionDAO) AddVideo(video *SubscriptionVideo) error {
	return db.GormDB.Create(video).Error
}

// GetVideosBySubscription 获取订阅下的所有视频
func (d *subscriptionDAO) GetVideosBySubscription(subID uint) ([]*SubscriptionVideo, error) {
	var videos []*SubscriptionVideo
	err := db.GormDB.Where("subscription_id = ?", subID).
		Order("created_at desc"). // 最新发现的在最前
		Find(&videos).Error
	return videos, err
}

// --- SubscriptionBadgeState 相关 ---

// GetBadgeState 获取订阅的角标状态
func (d *subscriptionDAO) GetBadgeState(subID uint) (*SubscriptionBadgeState, error) {
	var state SubscriptionBadgeState
	err := db.GormDB.Limit(1).Find(&state, "subscription_id = ?", subID).Error
	if err != nil {
		return nil, err
	}
	if state.SubscriptionID == 0 {
		return &SubscriptionBadgeState{
			SubscriptionID: subID,
			NewCount:       0,
		}, nil
	}
	return &state, err
}

// GetAllBadgeStates 获取所有订阅的角标状态 (用于UI批量显示)
func (d *subscriptionDAO) GetAllBadgeStates() (map[uint]*SubscriptionBadgeState, error) {
	var states []*SubscriptionBadgeState
	if err := db.GormDB.Find(&states).Error; err != nil {
		return nil, err
	}

	result := make(map[uint]*SubscriptionBadgeState)
	for _, s := range states {
		result[s.SubscriptionID] = s
	}
	return result, nil
}

// UpdateBadgeState 更新角标状态 (扫描进度回调时使用)
func (d *subscriptionDAO) UpdateBadgeState(state *SubscriptionBadgeState) error {
	state.UpdatedAt = time.Now()
	return db.GormDB.Save(state).Error
}

// IncrementBadgeCount 增加新视频计数 (扫描到新视频时调用)
func (d *subscriptionDAO) IncrementBadgeCount(subID uint, delta int) error {
	return db.GormDB.Model(&SubscriptionBadgeState{}).
		Where("subscription_id = ?", subID).
		UpdateColumn("new_count", gorm.Expr("new_count + ?", delta)).
		Error
}

// MarkAsRead 标记订阅为已读 (用户点击后清空角标)
func (d *subscriptionDAO) MarkAsRead(subID uint) error {
	now := time.Now()
	return db.GormDB.Model(&SubscriptionBadgeState{}).
		Where("subscription_id = ?", subID).
		Updates(map[string]interface{}{
			"new_count":    0,
			"last_read_at": now,
			"updated_at":   now,
		}).Error
}

// SetScanning 设置扫描状态 (用于UI显示loading动画)
func (d *subscriptionDAO) SetScanning(subID uint, isScanning bool) error {
	return db.GormDB.Model(&SubscriptionBadgeState{}).
		Where("subscription_id = ?", subID).
		Updates(map[string]interface{}{
			"is_scanning": isScanning,
			"updated_at":  time.Now(),
		}).Error
}

// UpdateLastVideoID 更新最后扫描到的视频ID (用于快速判断更新)
func (d *subscriptionDAO) UpdateLastVideoID(subID uint, videoID string) error {
	return db.GormDB.Model(&Subscription{}).
		Where("id = ?", subID).
		Update("last_video_id", videoID).Error
}
