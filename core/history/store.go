package history

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/hankmor/vdd/core/db"
	"github.com/hankmor/vdd/core/logger"
)

// HistoryRecord 下载历史记录
type HistoryRecord struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	FormatID    string    `json:"format_id"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	Duration    string    `json:"duration"`
	CompletedAt time.Time `json:"completed_at"`
	Status      string    `json:"status"`
	Thumbnail   string    `json:"thumbnail"`
}

// Add 添加记录
func Add(record HistoryRecord) error {
	if db.DB == nil {
		return nil // 或者 db.Init()
	}

	// 规范化文件路径：转换为绝对路径，统一路径分隔符
	normalizedPath := NormalizeFilePath(record.FilePath)

	_, err := db.DB.Exec(`
		INSERT INTO history (id, title, url, format_id, file_path, file_size, duration, completed_at, status, thumbnail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.Title, record.URL, record.FormatID, normalizedPath, record.FileSize, record.Duration, record.CompletedAt, "completed", record.Thumbnail)

	return err
}

// NormalizeFilePath 规范化文件路径
// 1. 转换为绝对路径
// 2. 统一路径分隔符
// 3. 清理路径（移除多余的 . 和 ..）
func NormalizeFilePath(path string) string {
	if path == "" {
		return ""
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		// 如果转换失败，使用原始路径
		logger.Debugf("[历史] 转换为绝对路径失败: %v, 使用原始路径: %s", err, path)
		absPath = path
	}

	// 清理路径（移除多余的 . 和 ..，统一分隔符）
	cleanPath := filepath.Clean(absPath)

	// 统一使用系统路径分隔符（filepath.Clean 已经处理）
	// 但为了跨平台兼容性，我们确保路径是规范化的
	return cleanPath
}

// ValidateFilePath 验证文件路径是否存在
func ValidateFilePath(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// GetAll 获取所有记录 (Legacay, keep for compatibility if needed, or remove)
func GetAll() []HistoryRecord {
	records, _, _ := GetHistory(0, 100)
	return records
}

// GetHistory 分页获取历史记录
func GetHistory(offset, limit int) ([]HistoryRecord, int64, error) {
	if db.DB == nil {
		return nil, 0, nil
	}

	// 1. Get total count
	var total int64
	err := db.DB.QueryRow("SELECT COUNT(*) FROM history").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 2. Get records
	rows, err := db.DB.Query(`
		SELECT id, title, url, format_id, file_path, file_size, duration, completed_at, status, thumbnail
		FROM history 
		ORDER BY completed_at DESC 
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []HistoryRecord
	for rows.Next() {
		var r HistoryRecord
		var completedAt time.Time
		var thumbnail sql.NullString

		err := rows.Scan(&r.ID, &r.Title, &r.URL, &r.FormatID, &r.FilePath, &r.FileSize, &r.Duration, &completedAt, &r.Status, &thumbnail)
		if err != nil {
			logger.Errorf("[历史] 扫描错误: %v", err)
			continue
		}
		r.CompletedAt = completedAt
		r.Thumbnail = thumbnail.String
		records = append(records, r)
	}

	return records, total, nil
}

// Delete 删除记录
func Delete(id string) error {
	if db.DB == nil {
		return nil
	}
	_, err := db.DB.Exec("DELETE FROM history WHERE id = ?", id)
	return err
}

// Clear 清空历史
func Clear() error {
	if db.DB == nil {
		return nil
	}
	_, err := db.DB.Exec("DELETE FROM history")
	return err
}

// GetAllRecords 获取所有历史记录（用于批量操作）
func GetAllRecords() ([]HistoryRecord, error) {
	if db.DB == nil {
		return nil, nil
	}

	rows, err := db.DB.Query(`
		SELECT id, title, url, format_id, file_path, file_size, duration, completed_at, status, thumbnail
		FROM history 
		ORDER BY completed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []HistoryRecord
	for rows.Next() {
		var r HistoryRecord
		var completedAt time.Time
		var thumbnail sql.NullString

		err := rows.Scan(&r.ID, &r.Title, &r.URL, &r.FormatID, &r.FilePath, &r.FileSize, &r.Duration, &completedAt, &r.Status, &thumbnail)
		if err != nil {
			logger.Errorf("[历史] 扫描错误: %v", err)
			continue
		}
		r.CompletedAt = completedAt
		r.Thumbnail = thumbnail.String
		records = append(records, r)
	}

	return records, nil
}

// DeleteMissingFiles 删除文件不存在的历史记录
// 返回删除的记录数量和错误
func DeleteMissingFiles() (int, error) {
	if db.DB == nil {
		return 0, nil
	}

	// 获取所有记录
	records, err := GetAllRecords()
	if err != nil {
		return 0, err
	}

	deletedCount := 0
	for _, rec := range records {
		// 检查文件是否存在
		if !ValidateFilePath(rec.FilePath) {
			// 文件不存在，删除记录
			if err := Delete(rec.ID); err != nil {
				logger.Errorf("[历史] 删除文件不存在记录失败: %s: %v", rec.ID, err)
				continue
			}
			deletedCount++
		}
	}

	return deletedCount, nil
}
