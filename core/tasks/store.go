package tasks

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/db"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/parser"
	"github.com/hankmor/vdd/utils"
)

var DAO = &dao{}

type dao struct {
}

// TaskFilter 任务过滤和分页参数
type TaskFilter struct {
	SubscriptionID uint             // 订阅ID
	Statuses       []TaskStatus     // 状态过滤（nil或空 = 全部）
	Page           int              // 页码（从1开始）
	PageSize       int              // 每页数量
	OrderBy        string           // 排序字段（默认：优先级+updated_at DESC）
}

// CreateTaskFromParser 由解析结果创建任务
func (d *dao) CreateTaskFromParser(url string, pr *parser.VideoInfo, format *parser.Format, templatePath string, isMergeToMp4 bool, cookieFile string) (*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	task := &Task{
		ID:           uuid.New().String(),
		URL:          url,
		Status:       StatusQueued,
		Title:        pr.Title,
		Uploader:     pr.Uploader,
		UploadDate:   pr.UploadDate,
		Description:  pr.Description,
		Thumbnail:    pr.Thumbnail,
		Duration:     pr.Duration,
		Resolution:   format.Resolution,
		Width:        format.Width,
		Height:       format.Height,
		FPS:          format.FPS,
		VCodec:       format.VCodec,
		ACodec:       format.ACodec,
		ABR:          format.ABR,
		VBR:          format.VBR,
		Extension:    format.Extension,
		FormatID:     format.FormatID,
		TemplatePath: templatePath,
		TotalSize:    format.FileSize,
		CookieFile:   cookieFile,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	actualPath := utils.EstimateActualFilePath(task.TemplatePath, task.Title, task.Extension, task.FormatID, task.Uploader, task.UploadDate, config.Get().DownloadDir, isMergeToMp4)
	task.ActualPath = actualPath

	// 使用 GORM Create
	if err := db.GormDB.Create(task).Error; err != nil {
		return nil, err
	}

	return task, nil
}

// CreateTask 直接保存构造好的任务对象
func (d *dao) CreateTask(task *Task) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.GormDB.Create(task).Error
}

// GetTaskByURL 获取指定 URL 的最新任务
func (d *dao) GetTaskByURL(url string) (*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var task Task
	// 取最新的一个 (Order by UpdatedAt DESC)
	if err := db.GormDB.Where("url = ?", url).Order("updated_at desc").First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTaskByID 通过 ID 获取任务
func (d *dao) GetTaskByID(id string) (*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var task Task
	if err := db.GormDB.First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// GetAllTasks 获取所有任务 (按创建时间倒序) - 保留用于需要导出全部数据的情况
func (d *dao) GetAllTasks() ([]*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tasks []*Task
	err := db.GormDB.Order(`case status
             when 'downloading' then 0
             when 'failed' then 1
             when 'queued' then 2
             when 'completed' then 3
             when 'canceled' then 4
             end ASC, created_at DESC`).Find(&tasks).Error

	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetManualTasks 获取所有手动添加的任务 (非订阅, SubscriptionID IS NULL)
func (d *dao) GetManualTasks() ([]*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tasks []*Task
	// 保持原有的复杂排序逻辑
	err := db.GormDB.Where("subscription_id IS NULL").Order(`case status
             when 'downloading' then 0
             when 'failed' then 1
             when 'queued' then 2
             when 'completed' then 3
             when 'canceled' then 4
             end ASC, created_at DESC`).Find(&tasks).Error

	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetActiveManualTasks 获取所有活跃的手动任务 (非订阅)
func (d *dao) GetActiveManualTasks() ([]*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tasks []*Task
	err := db.GormDB.Where("subscription_id IS NULL AND status IN ?", []TaskStatus{StatusDownloading, StatusQueued}).Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetTasksByStatus 分页获取特定状态的任务 (status 为空则获取所有)
// 返回: tasks, totalCount, error
func (d *dao) GetTasksByStatus(status ...TaskStatus) ([]*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tasks []*Task

	// 创建基础查询
	if err := db.GormDB.Model(&Task{}).Where("status in ?", status).Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// UpdateParseResult 更新解析结果
func (d *dao) UpdateParseResult(id string, pr *parser.VideoInfo, format *parser.Format, templatePath string) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}

	// 使用 Updates 进行部分更新
	return db.GormDB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":         pr.Title,
		"uploader":      pr.Uploader,
		"upload_date":   pr.UploadDate,
		"description":   pr.Description,
		"thumbnail":     pr.Thumbnail,
		"duration":      pr.Duration,
		"resolution":    format.Resolution,
		"width":         format.Width,
		"height":        format.Height,
		"fps":           format.FPS,
		"vcodec":        format.VCodec,
		"acodec":        format.ACodec,
		"abr":           format.ABR,
		"vbr":           format.VBR,
		"extension":     format.Extension,
		"format_id":     format.FormatID,
		"template_path": templatePath,
		"total_size":    format.FileSize,
		"status":        StatusQueued,
		"progress":      0,
		"error_msg":     "",
		"updated_at":    time.Now(),
	}).Error
}

// UpdateProgress 更新进度
func (d *dao) UpdateProgress(id string, progress float64) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.GormDB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"progress":   progress,
		"updated_at": time.Now(),
	}).Error
}

// UpdateTaskSize 更新任务文件大小
func (d *dao) UpdateTaskSize(id string, size int64) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.GormDB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"total_size": size,
		"updated_at": time.Now(),
	}).Error
}

// UpdateTaskPath 更新任务文件实际路径（用于订阅任务下载完成后）
func (d *dao) UpdateTaskPath(id string, actualPath string) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.GormDB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"actual_path": actualPath,
		"updated_at":  time.Now(),
	}).Error
}

// UpdateError 记录错误
func (d *dao) UpdateStatusAndError(id string, status TaskStatus, errorMsg string) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.GormDB.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"error_msg":  errorMsg,
		"updated_at": time.Now(),
	}).Error
}

// DeleteTask 删除任务
func (d *dao) DeleteTask(id string) error {
	if db.GormDB == nil {
		return nil
	}
	// 使用 Unscoped() 强制物理删除
	return db.GormDB.Unscoped().Where("id = ?", id).Delete(&Task{}).Error
}

// ClearCookieFile 清除任务的 Cookie 文件路径 设置为空
func (d *dao) ClearCookieFile(id string) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}
	// 更新 cookie_file 为空字符串
	// 注意 GORM 默认忽略零值更新，所以必须使用 map 或者指定 Select/UpdateColumn
	return db.GormDB.Model(&Task{}).Where("id = ?", id).Select("cookie_file").Update("cookie_file", "").Error
}

func (d *dao) CancelDownloadingAndQuened() {
	downloadingTasks, err := DAO.GetTasksByStatus(StatusDownloading, StatusQueued)
	if err != nil {
		return
	}
	for _, task := range downloadingTasks {
		if err := DAO.UpdateStatusAndError(task.ID, StatusCanceled, ""); err != nil {
			logger.Errorf("更新任务状态失败: %v", err)
		}
	}
}

// GetTasksBySubscriptionID 获取特定订阅的所有任务
func (d *dao) GetTasksBySubscriptionID(subID uint) ([]*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tasks []*Task
	if err := db.GormDB.Where("subscription_id = ?", subID).Order(`
		CASE status
			WHEN 'downloading' THEN 1
			WHEN 'queued' THEN 2
			WHEN 'failed' THEN 3
			WHEN 'completed' THEN 4
			WHEN 'canceled' THEN 5
			ELSE 99
		END ASC,
		created_at DESC`).Find(&tasks).Error; err != nil {

		return nil, err
	}
	return tasks, nil
}

// GetTasksBySubscriptionIDPaginated 分页查询订阅任务（带状态过滤）
func (d *dao) GetTasksBySubscriptionIDPaginated(filter TaskFilter) ([]*Task, int64, error) {
	if db.GormDB == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	// 构建查询
	query := db.GormDB.Where("subscription_id = ?", filter.SubscriptionID)

	// 状态过滤（支持多个状态）
	if len(filter.Statuses) > 0 {
		query = query.Where("status IN ?", filter.Statuses)
	}

	// 查询总数
	var total int64
	if err := query.Model(&Task{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页参数
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20 // 默认每页20个
	}

	// 排序规则：优先级（下载中>队列中>失败>已完成>已取消）+ updated_at DESC
	// 使用 updated_at 确保最近活跃的任务（状态刚变化的）排在前面
	orderSQL := `CASE status WHEN 'downloading' THEN 1 WHEN 'queued' THEN 2 WHEN 'failed' THEN 3 WHEN 'completed' THEN 4 WHEN 'canceled' THEN 5 ELSE 99 END ASC, created_at DESC`

	// 分页查询
	var tasks []*Task
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Debug(). // 打印SQL，用于调试排序问题
		Order(orderSQL). // 直接传递字符串，不需要 Raw()
		Limit(filter.PageSize).
		Offset(offset).
		Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// GetTasksByFilter 根据过滤条件获取所有任务（不分页）
func (d *dao) GetTasksByFilter(filter TaskFilter) ([]*Task, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// 构建查询
	query := db.GormDB.Where("subscription_id = ?", filter.SubscriptionID)

	// 状态过滤（支持多个状态）
	if len(filter.Statuses) > 0 {
		query = query.Where("status IN ?", filter.Statuses)
	}

	// 排序规则：优先级 + updated_at DESC
	orderSQL := `CASE status WHEN 'downloading' THEN 1 WHEN 'queued' THEN 2 WHEN 'failed' THEN 3 WHEN 'completed' THEN 4 WHEN 'canceled' THEN 5 ELSE 99 END ASC, created_at DESC`

	var tasks []*Task
	if err := query.Order(db.GormDB.Raw(orderSQL)).Find(&tasks).Error; err != nil {
		return nil, err
	}

	return tasks, nil
}

type SubscriptionStats struct {
	Total       int64
	Completed   int64
	Downloading int64 // 下载中
	Queued      int64 // 排队中
	Failed      int64
	Canceled    int64
}

// GetSubscriptionStats 获取订阅的统计信息
func (d *dao) GetSubscriptionStats(subID uint) (*SubscriptionStats, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats := &SubscriptionStats{}

	// 总数
	if err := db.GormDB.Model(&Task{}).Where("subscription_id = ?", subID).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	if stats.Total == 0 {
		return stats, nil
	}

	// 已完成
	db.GormDB.Model(&Task{}).Where("subscription_id = ? AND status = ?", subID, StatusCompleted).Count(&stats.Completed)

	// 下载中 + 排队中
	db.GormDB.Model(&Task{}).Where("subscription_id = ? AND status IN ?", subID, []TaskStatus{StatusDownloading, StatusQueued}).Count(&stats.Downloading)

	// 失败
	db.GormDB.Model(&Task{}).Where("subscription_id = ? AND status = ?", subID, StatusFailed).Count(&stats.Failed)

	// 已取消
	db.GormDB.Model(&Task{}).Where("subscription_id = ? AND status = ?", subID, StatusCanceled).Count(&stats.Canceled)

	return stats, nil
}

// GetAllSubscriptionStats 获取所有订阅任务的统计信息
func (d *dao) GetAllSubscriptionStats() (*SubscriptionStats, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats := &SubscriptionStats{}

	// 总数 (只统计有订阅ID的任务)
	if err := db.GormDB.Model(&Task{}).Where("subscription_id IS NOT NULL").Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	if stats.Total == 0 {
		return stats, nil
	}

	// 已完成
	db.GormDB.Model(&Task{}).Where("subscription_id IS NOT NULL AND status = ?", StatusCompleted).Count(&stats.Completed)

	// 正在下载
	db.GormDB.Model(&Task{}).Where("subscription_id IS NOT NULL AND status = ?", StatusDownloading).Count(&stats.Downloading)
	
	// 排队中
	db.GormDB.Model(&Task{}).Where("subscription_id IS NOT NULL AND status = ?", StatusQueued).Count(&stats.Queued)

	// 失败
	db.GormDB.Model(&Task{}).Where("subscription_id IS NOT NULL AND status = ?", StatusFailed).Count(&stats.Failed)

	// 已取消
	db.GormDB.Model(&Task{}).Where("subscription_id IS NOT NULL AND status = ?", StatusCanceled).Count(&stats.Canceled)

	return stats, nil
}

// GetManualTasksStats 获取所有普通任务（非订阅）的统计信息
func (d *dao) GetManualTasksStats() (*SubscriptionStats, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats := &SubscriptionStats{}

	// 总数 (只统计没有订阅ID的任务)
	if err := db.GormDB.Model(&Task{}).Where("subscription_id IS NULL").Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	if stats.Total == 0 {
		return stats, nil
	}

	// 已完成
	db.GormDB.Model(&Task{}).Where("subscription_id IS NULL AND status = ?", StatusCompleted).Count(&stats.Completed)

	// 正在下载
	db.GormDB.Model(&Task{}).Where("subscription_id IS NULL AND status = ?", StatusDownloading).Count(&stats.Downloading)

	// 排队中
	db.GormDB.Model(&Task{}).Where("subscription_id IS NULL AND status = ?", StatusQueued).Count(&stats.Queued)

	// 失败
	db.GormDB.Model(&Task{}).Where("subscription_id IS NULL AND status = ?", StatusFailed).Count(&stats.Failed)

	// 已取消
	db.GormDB.Model(&Task{}).Where("subscription_id IS NULL AND status = ?", StatusCanceled).Count(&stats.Canceled)

	return stats, nil
}
