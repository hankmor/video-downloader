package config

import (
	"fmt"
	"os"
	"path/filepath"

	"time"

	"github.com/hankmor/vdd/core/db"
	"github.com/hankmor/vdd/core/logger"
)

const (
	ThemeAuto  = "auto"
	ThemeLight = "light"
	ThemeDark  = "dark"

	MaxConcurrent = 20
)

// Config 定义应用程序配置结构
type Config struct {
	DownloadDir   string
	MaxConcurrent int
	Theme         string
	FFmpegPath    string
	AutoDownload  bool
	ProxyURL      string // 代理地址 (http://... socks5://...)
	Subtitle      bool   // 是否下载字幕
	SubtitleLangs string // 字幕语言 (zh-Hans,en)

	// 自动扫描订阅
	AutoScanSubscriptions  bool // 是否在启动时自动扫描订阅
	AutoBackgroundScan     bool // 是否启用后台自动扫描
	BackgroundScanInterval int  // 后台扫描间隔 (分钟)

	// Cookie 设置
	EnableBrowserCookie bool   // 是否启用浏览器 Cookie
	BrowserName         string // 浏览器名称 (chrome, edge, firefox, etc.)

	CookiesPath        string // Cookies 文件路径 (Netscape 格式, 兼作"上次使用的文件"记忆)
	ClipboardMonitor   bool   // 是否开启剪贴板监听
	FilenameFormat     string // 文件名格式 (title, title_uploader, etc.)
	FirstRunDate       string // 首次运行日期 (2006-01-02)
	ResolutionStrategy string // 分辨率策略 (best, saver)

	// 主题样式配置
	DarkStyle  string // dark mode 下的具体样式 (titanium, cyberpunk)
	LightStyle string // light mode 下的具体样式 (polar, latte)

	StartupTime time.Time // 程序启动时间
}

var globalConfig *Config

// Load 加载配置
// 如果数据库中不存在，使用默认值
func Load() error {
	// 确保 DB 已初始化
	if db.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	defaults := Default()
	cfg := &Config{}

	// 读取各个字段
	cfg.DownloadDir = DAO.GetString(db.DB, "download_dir", defaults.DownloadDir)
	cfg.MaxConcurrent = DAO.GetInt(db.DB, "max_concurrent", defaults.MaxConcurrent)
	cfg.Theme = DAO.GetString(db.DB, "theme", defaults.Theme)
	cfg.FFmpegPath = DAO.GetString(db.DB, "ffmpeg_path", defaults.FFmpegPath)
	cfg.AutoDownload = DAO.GetBool(db.DB, "auto_download", defaults.AutoDownload)
	cfg.ProxyURL = DAO.GetString(db.DB, "proxy_url", defaults.ProxyURL)
	cfg.Subtitle = DAO.GetBool(db.DB, "subtitle", defaults.Subtitle)
	cfg.SubtitleLangs = DAO.GetString(db.DB, "subtitle_langs", defaults.SubtitleLangs)
	cfg.AutoScanSubscriptions = DAO.GetBool(db.DB, "auto_scan_subscriptions", defaults.AutoScanSubscriptions)
	cfg.AutoBackgroundScan = DAO.GetBool(db.DB, "auto_background_scan", defaults.AutoBackgroundScan)
	cfg.BackgroundScanInterval = DAO.GetInt(db.DB, "background_scan_interval", defaults.BackgroundScanInterval)

	cfg.EnableBrowserCookie = DAO.GetBool(db.DB, "enable_browser_cookie", defaults.EnableBrowserCookie)
	cfg.BrowserName = DAO.GetString(db.DB, "browser_name", defaults.BrowserName)

	cfg.CookiesPath = DAO.GetString(db.DB, "cookies_path", defaults.CookiesPath)
	cfg.ClipboardMonitor = DAO.GetBool(db.DB, "clipboard_monitor", defaults.ClipboardMonitor)
	cfg.FilenameFormat = DAO.GetString(db.DB, "filename_format", defaults.FilenameFormat)
	cfg.ResolutionStrategy = DAO.GetString(db.DB, "resolution_strategy", defaults.ResolutionStrategy)
	cfg.DarkStyle = DAO.GetString(db.DB, "dark_style", defaults.DarkStyle)
	cfg.LightStyle = DAO.GetString(db.DB, "light_style", defaults.LightStyle)
	cfg.FirstRunDate = checkAndSetFirstRunDate()

	cfg.StartupTime = time.Now() // 记录程序启动时间

	globalConfig = cfg
	return nil
}

// Save 保存配置到数据库
func Save() error {
	if globalConfig == nil || db.DB == nil {
		return fmt.Errorf("配置或数据库未初始化")
	}

	tx, err := db.DB.Begin()
	if err != nil {
		logger.Errorf("error: %v", err)
		return err
	}

	DAO.Set(tx, "download_dir", globalConfig.DownloadDir)
	DAO.Set(tx, "max_concurrent", fmt.Sprintf("%d", globalConfig.MaxConcurrent))
	DAO.Set(tx, "theme", globalConfig.Theme)
	DAO.Set(tx, "ffmpeg_path", globalConfig.FFmpegPath)
	DAO.Set(tx, "auto_download", fmt.Sprintf("%v", globalConfig.AutoDownload))
	DAO.Set(tx, "proxy_url", globalConfig.ProxyURL)
	DAO.Set(tx, "subtitle", fmt.Sprintf("%v", globalConfig.Subtitle))
	DAO.Set(tx, "subtitle_langs", globalConfig.SubtitleLangs)
	DAO.Set(tx, "auto_scan_subscriptions", fmt.Sprintf("%v", globalConfig.AutoScanSubscriptions))
	DAO.Set(tx, "auto_background_scan", fmt.Sprintf("%v", globalConfig.AutoBackgroundScan))
	DAO.Set(tx, "background_scan_interval", fmt.Sprintf("%d", globalConfig.BackgroundScanInterval))

	DAO.Set(tx, "enable_browser_cookie", fmt.Sprintf("%v", globalConfig.EnableBrowserCookie))
	DAO.Set(tx, "browser_name", globalConfig.BrowserName)

	DAO.Set(tx, "cookies_path", globalConfig.CookiesPath)
	DAO.Set(tx, "clipboard_monitor", fmt.Sprintf("%v", globalConfig.ClipboardMonitor))
	DAO.Set(tx, "filename_format", globalConfig.FilenameFormat)
	DAO.Set(tx, "resolution_strategy", globalConfig.ResolutionStrategy)
	DAO.Set(tx, "dark_style", globalConfig.DarkStyle)
	DAO.Set(tx, "light_style", globalConfig.LightStyle)

	DAO.Set(tx, "first_run_date", globalConfig.FirstRunDate)

	return tx.Commit()
}

// Default 返回默认配置
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DownloadDir:            filepath.Join(home, "Downloads"),
		MaxConcurrent:          3,
		Theme:                  "auto",
		AutoDownload:           false,
		Subtitle:               true,
		SubtitleLangs:          "zh-Hans,en",
		AutoScanSubscriptions:  false,
		AutoBackgroundScan:     true,
		BackgroundScanInterval: 10,
		ProxyURL:               "",

		EnableBrowserCookie: true,
		BrowserName:         "chrome",
		CookiesPath:         "", // 默认不指定文件

		ClipboardMonitor:   true,
		FilenameFormat:     "title",
		ResolutionStrategy: "best",
		DarkStyle:          "titanium",
		LightStyle:         "polar",
		FirstRunDate:       time.Now().Format("2006-01-02"),
	}
}

// Get 获取当前配置单例
func Get() *Config {
	if globalConfig == nil {
		Load()
	}
	return globalConfig
}

// GetFilenameTemplate 获取 yt-dlp 文件名模板
func (c *Config) GetFilenameTemplate() string {
	switch c.FilenameFormat {
	case "title_uploader":
		return "%(title)s-%(uploader)s.%(ext)s"
	case "uploader_title":
		return "%(uploader)s-%(title)s.%(ext)s"
	case "date_title":
		return "%(upload_date)s-%(title)s.%(ext)s"
	default:
		// 默认: 标题.后缀
		return "%(title)s.%(ext)s"
	}
}

// GetResolutionFormatID 根据策略获取 yt-dlp 格式选择器
func (c *Config) GetResolutionFormatID() string {
	if c.ResolutionStrategy == "saver" {
		// 省空间模式: 最佳视频(不高于720p) + 最佳音频 / 最佳(不高于720p) / 最佳
		// bestvideo[height<=720] 会选择<=720p中最好的。
		return "bestvideo[height<=720]"
	}
	// 默认最高清
	return "bestvideo"
}

// checkAndSetFirstRunDate 检查并设置首次运行日期
func checkAndSetFirstRunDate() string {
	dbDate := DAO.GetString(db.DB, "first_run_date", "")
	if dbDate == "" {
		dbDate = time.Now().Format("2006-01-02")
		tx, err := db.DB.Begin()
		if err == nil {
			DAO.Set(tx, "first_run_date", dbDate)
			tx.Commit()
		} else {
			logger.Errorf("error: %v", err)
		}
	}
	return dbDate
}
