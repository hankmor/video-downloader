package tasks

import (
	"database/sql"
	"time"

	"github.com/hankmor/vdd/utils"
)

// TaskStatus 定义任务状态
type TaskStatus string

const (
	StatusQueued      TaskStatus = "queued"      // 排队中
	StatusDownloading TaskStatus = "downloading" // 下载中
	StatusCompleted   TaskStatus = "completed"   // 已完成
	StatusFailed      TaskStatus = "failed"      // 失败
	StatusCanceled    TaskStatus = "canceled"    // 已取消
)

func (t TaskStatus) Name() string {
	switch t {
	case StatusQueued:
		return "排队中"
	case StatusDownloading:
		return "下载中"
	case StatusCompleted:
		return "已完成"
	case StatusFailed:
		return "下载失败"
	case StatusCanceled:
		return "已取消"
	}
	return "未知"
}

// Task 表示一个下载任务（对应数据库 tasks 表）
type Task struct {
	ID  string `json:"id" gorm:"primaryKey"`
	URL string `json:"url" gorm:"index"` // 原始输入 URL

	// 解析阶段数据 (Cache)
	Title       string  `json:"title"`
	Uploader    string  `json:"uploader"`
	UploadDate  string  `json:"upload_date"` // YYYYMMDD
	Description string  `json:"description"`
	Thumbnail   string  `json:"thumbnail"`
	Duration    float64 `json:"duration"`
	Extension   string  `json:"extension"`
	Resolution  string  `json:"resolution"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
	VCodec      string  `json:"vcodec"`
	ACodec      string  `json:"acodec"`
	ABR         float64 `json:"abr"`
	VBR         float64 `json:"vbr"`
	// 下载阶段数据
	FormatID     string `json:"format_id"`
	TemplatePath string `json:"output_path"`
	OutputFolder string `json:"output_folder"` // 自定义输出目录 (用于订阅分类)
	ActualPath   string `json:"actual_path"`
	TotalSize    int64  `json:"total_size"`
	CookieFile   string `json:"cookie_file"` // 覆盖用的 Cookie 文件路径

	// 关联订阅
	SubscriptionID *uint `json:"subscription_id" gorm:"index"`

	// 状态管理
	Status   TaskStatus `json:"status" gorm:"index"`
	Progress float64    `json:"progress"` // 0.0 - 100.0
	ErrorMsg string     `json:"error_msg"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (t *Task) ResolutionDimension() int {
	return utils.Min(t.Width, t.Height)
}

// NullString helper since sql.NullString is annoying to work with directly in structs if we want JSON
func NewNullString(s string) sql.NullString {
	if len(s) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

func (t *Task) CanSchedule() bool {
	return t.Status == StatusQueued || t.Status == StatusFailed || t.Status == StatusCanceled
}

func (t *Task) CanStartDownload() bool {
	return t.Status == StatusFailed || t.Status == StatusCanceled
}
