package subscription

import (
	"time"
)

// SubscriptionStatus 订阅状态
type SubscriptionStatus string

const (
	StatusActive SubscriptionStatus = "active"
	StatusPaused SubscriptionStatus = "paused"
)

// Subscription 订阅源模型
type Subscription struct {
	ID             uint               `gorm:"primaryKey" json:"id"`
	Name           string             `json:"name"`
	URL            string             `gorm:"uniqueIndex" json:"url"` // 播放列表/频道 URL
	Status         SubscriptionStatus `json:"status"`                 // 状态
	Interval       int                `json:"interval"`               // 检查间隔(秒)，默认 3600
	LastCheckAt    time.Time          `json:"last_check_at"`          // 上次检查时间
	FilterKeywords string             `json:"filter_keywords"`        // 标题关键词过滤(可选)
	Thumbnail      string             `json:"thumbnail"`              // 封面图 URL
	LastVideoID    string             `json:"last_video_id"`          // 最后扫描到的视频ID (用于快速判断更新)

	// 统计信息
	TotalCount int `gorm:"-" json:"total_count"` // 总视频数 (实时查询)
	NewCount   int `gorm:"-" json:"new_count"`   // 本次会话新增数 (UI Badge用)

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SubscriptionVideo 订阅视频记录 (用于去重和历史记录)
type SubscriptionVideo struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	SubscriptionID uint   `gorm:"index" json:"subscription_id"`
	VideoID        string `gorm:"index" json:"video_id"` // 平台视频ID (e.g. YouTube ID)
	Title          string `json:"title"`
	URL            string `json:"url"`
	PublishedAt    string `json:"published_at"` // YYYYMMDD

	// 冗余字段，方便快速显示
	Thumbnail string  `json:"thumbnail"`
	Duration  float64 `json:"duration"`

	CreatedAt time.Time `json:"created_at"`
}

// ScanCategory 扫描类型
type ScanCategory int

const (
	ScanCategorySingle ScanCategory = iota
	ScanCategoryBatch
)

// ScanProgress 扫描进度
type ScanProgress struct {
	Category         ScanCategory // 扫描类别 (单次/批量)
	SubscriptionID   uint
	SubscriptionName string
	IsScanning       bool // 是否正在进行中 (对于 CategoryBatch，表示整个批次的状态)
	NewCount         int  // 新增视频数 (对于 CategoryBatch，表示批次累计)
	Error            string

	// 批量扫描特有
	TotalSubs   int // 批次总订阅数
	ScannedSubs int // 已扫描订阅数
}

// SubscriptionBadgeState 订阅角标状态 (用于持久化新视频数量和已读状态)
type SubscriptionBadgeState struct {
	SubscriptionID uint      `gorm:"primaryKey" json:"subscription_id"` // 订阅ID
	NewCount       int       `json:"new_count"`                         // 新视频数量
	LastScanAt     time.Time `json:"last_scan_at"`                      // 最后扫描时间
	LastReadAt     time.Time `json:"last_read_at"`                      // 最后已读时间
	IsScanning     bool      `json:"is_scanning"`                       // 是否正在扫描
	UpdatedAt      time.Time `json:"updated_at"`
}
