package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/hankmor/vdd/core/logger"
	"gorm.io/gorm"
)

var (
	DB     *sql.DB  // Legacy raw SQL DB
	GormDB *gorm.DB // GORM DB
)

// Init 初始化数据库连接和 schema
// models: 需要自动迁移的 GORM 模型
func Init(models ...interface{}) error {
	dbPath := getDBPath()
	logger.Debugf("初始化数据库: %s", dbPath)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	var err error
	// 使用 GORM 连接
	GormDB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层的 *sql.DB 以兼容旧代码
	DB, err = GormDB.DB()
	if err != nil {
		return fmt.Errorf("获取底层 SQL DB 失败: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 自动迁移
	if len(models) > 0 {
		if err := GormDB.AutoMigrate(models...); err != nil {
			return fmt.Errorf("数据库自动迁移失败: %w", err)
		}
	}

	return nil
}

// getDBPath 获取数据库文件路径
// 优先使用 os.UserConfigDir
func getDBPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".vdd", "vdd.db")
	}
	return filepath.Join(configDir, "VDD", "vdd.db")
}

// Close 关闭数据库
func Close() {
	if DB != nil {
		DB.Close()
	}
}
