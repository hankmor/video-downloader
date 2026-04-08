package config

import (
	"database/sql"
	"strconv"

	"github.com/hankmor/vdd/core/db"
	"github.com/hankmor/vdd/core/logger"
)

var DAO = &dao{}	

type dao struct {
}

// ConfigModel 配置模型
type ConfigModel struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

// TableName 指定表名
func (ConfigModel) TableName() string {
	return "configs"
}

// 辅助函数：获取字符串配置
func (dao *dao) GetString(d *sql.DB, key, defaultValue string) string {
	if db.GormDB == nil {
		return defaultValue
	}
	var cfg ConfigModel
	result := db.GormDB.First(&cfg, "key = ?", key)
	if result.Error != nil {
		return defaultValue // RecordNotFound or other error
	}
	return cfg.Value
}

// 辅助函数：获取整数配置
func (dao *dao) GetInt(d *sql.DB, key string, defaultValue int) int {
	strVal := dao.GetString(d, key, "")
	if strVal == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(strVal)
	if err != nil {
		logger.Errorf("error: %v", err)
		return defaultValue
	}
	return val
}

// 辅助函数：获取布尔配置
func (dao *dao) GetBool(d *sql.DB, key string, defaultValue bool) bool {
	strVal := dao.GetString(d, key, "")
	if strVal == "" {
		return defaultValue
	}
	val, err := strconv.ParseBool(strVal)
	if err != nil {
		return defaultValue
	}
	return val
}

// 辅助函数：保存配置
func (dao *dao) Set(tx *sql.Tx, key, value string) {
	if db.GormDB == nil {
		return
	}
	
	// 使用 Upsert (Save)
	// 如果主键Key存在则更新，否则插入
	err := db.GormDB.Save(&ConfigModel{Key: key, Value: value}).Error
	if err != nil {
		logger.Errorf("保存配置失败 [%s]: %v", key, err)
	}
}
