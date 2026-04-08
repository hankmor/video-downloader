package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hankmor/vdd/core/db"
	"gorm.io/gorm"
)

var DAO = &dao{}	

type dao struct {
}

// ParseResult represents the parsing result of a URL
type ParseResult struct {
	ID          string    `json:"id"`
	URL         string    `json:"url" gorm:"primaryKey"` // Use URL as primary key for caching
	Title       string    `json:"title"`
	Uploader    string    `json:"uploader"`
	Description string    `json:"description"`
	Thumbnail   string    `json:"thumbnail"`
	Duration    float64   `json:"duration"`
	MetaJSON    string    `json:"meta_json"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName explicitly sets the table name
func (ParseResult) TableName() string {
	return "parsing_results"
}

// SaveParseResult saves or updates the parsing result for a URL
func (d *dao) SaveParseResult(url string, info *VideoInfo) error {
	if db.GormDB == nil {
		return fmt.Errorf("database not initialized")
	}

	metaJSON, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal meta info: %w", err)
	}

	now := time.Now()

	// Use GORM Clauses for upsert (OnConflict)
	// 如果 URL 存在，则更新所有字段
	result := &ParseResult{
		ID:          info.ID,
		URL:         url,
		Title:       info.Title,
		Uploader:    info.Uploader,
		Description: info.Description,
		Thumbnail:   info.Thumbnail,
		Duration:    info.Duration,
		MetaJSON:    string(metaJSON),
		CreatedAt:   now, // 如果是新记录，使用当前时间
		UpdatedAt:   now,
	}

	// Save 会自动处理 Update (如果主键存在) 或 Insert (如果不存在)
	// 由于我们将 URL 设为 primaryKey，GORM 会根据 URL 判断是否存在
	if err := db.GormDB.Save(result).Error; err != nil {
		return err
	}

	// 注意：Save 默认会更新所有字段 (除了零值/空值在某些情况下)。
	// 为了确保 CreatedAt 不被覆盖（如果是更新），我们可以先查再更新，或者使用 Clauses。
	// 但在这个简单缓存场景中，覆盖 CreatedAt 也可以接受（视为新的缓存项），或者 GORM 的 Save 行为符合预期。
	// 这里简化处理，直接 Save。
	return nil
}

// GetParseResult retrieves the parsing result for a URL
func (d *dao) GetParseResult(url string) (*ParseResult, error) {
	if db.GormDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var p ParseResult
	// Find based on primary key (URL)
	if err := db.GormDB.First(&p, "url = ?", url).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}
