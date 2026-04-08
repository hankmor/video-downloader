package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hankmor/vdd/core/auth"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/download"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/utils"
)

type Manager struct {
	downloader *download.Downloader
	ytdlpPath  string

	cronMetrics *sync.Map // ID -> *Metrics (runtime stats)

	stopChan chan struct{}

	scanManager *ScanManager // 扫描管理器

	// 初始化过程跟踪（用于取消正在添加的订阅）
	initializingMu sync.RWMutex
	initializing   map[string]context.CancelFunc // URL -> CancelFunc
}

type Metrics struct {
	NewCount int
}

// 支持的订阅URL模式
const (
	// YouTube 频道和播放列表
	youtubeChannelPattern  = "youtube.com/channel/" // youtube.com/channel/UCXXX
	youtubeHandlePattern   = "youtube.com/@"        // youtube.com/@username
	youtubePlaylistPattern = "youtube.com/playlist" // youtube.com/playlist?list=PLxxx
	youtubeListParam       = "list="                // ?list=PLxxx
	youtubeDomain          = "youtube.com"
	youtubeShortDomain     = "youtu.be"

	// Bilibili 用户空间和合集
	bilibiliSpacePattern  = "space.bilibili.com"   // space.bilibili.com/xxxxx
	bilibiliVideoPattern  = "bilibili.com/video"   // bilibili.com/video/BVxxx?p=1 (多P视频)
	bilibiliSeasonPattern = "bilibili.com/bangumi" // bilibili.com/bangumi/play/ss/ep (番剧/影视)
	bilibiliDomain        = "bilibili.com"
	bilibiliTVDomain      = "bilibili.tv"

	// 其他平台
	tiktokDomain    = "tiktok.com"
	instagramDomain = "instagram.com"
	twitterDomain   = "twitter.com"
	xDomain         = "x.com"
	twitchDomain    = "twitch.tv"
)

// ValidateSubscriptionURL 快速验证URL是否为有效的订阅地址（播放列表/频道）
// 只做基础格式检查，不调用 yt-dlp
func (m *Manager) ValidateSubscriptionURL(url string) error {
	if url == "" {
		return fmt.Errorf("URL不能为空")
	}

	url = strings.TrimSpace(strings.ToLower(url))
	if !(strings.Index(url, "http://") == 0 || strings.Index(url, "https://") == 0) {
		return fmt.Errorf("URL格式不正确")
	}

	// 检查YouTube
	if strings.Contains(url, youtubeDomain) || strings.Contains(url, youtubeShortDomain) {
		// 有效的YouTube订阅类型
		if strings.Contains(url, youtubeChannelPattern) ||
			strings.Contains(url, youtubeHandlePattern) ||
			strings.Contains(url, youtubePlaylistPattern) ||
			strings.Contains(url, youtubeListParam) {
			return nil
		}
		return fmt.Errorf("YouTube URL必须是频道或播放列表\n支持格式：\n• 频道：youtube.com/channel/XXX 或 youtube.com/@username\n• 播放列表：youtube.com/playlist?list=XXX")
	}

	// 检查Bilibili
	if strings.Contains(url, bilibiliDomain) || strings.Contains(url, bilibiliTVDomain) {
		// 有效的Bilibili订阅类型
		if strings.Contains(url, bilibiliSpacePattern) ||
			(strings.Contains(url, bilibiliVideoPattern) && strings.Contains(url, "?p=")) ||
			strings.Contains(url, bilibiliSeasonPattern) {
			return nil
		}
		return fmt.Errorf("Bilibili URL必须是用户空间、多P视频或番剧合集\n支持格式：\n• 用户空间：space.bilibili.com/xxxxx\n• 多P视频：bilibili.com/video/BVxxx?p=1\n• 番剧合集：bilibili.com/bangumi/play/ssxxxxx")
	}

	// 检查TikTok (用户主页)
	if strings.Contains(url, tiktokDomain) {
		if strings.Contains(url, "@") {
			return nil
		}
		return fmt.Errorf("TikTok URL必须是用户主页\n格式：tiktok.com/@username")
	}

	// 检查Instagram (用户主页)
	if strings.Contains(url, instagramDomain) {
		// Instagram URL结构比较通用，只要不是单条帖子链接通常就是主页
		// 单条帖子通常包含 /p/ 或 /reel/
		if !strings.Contains(url, "/p/") && !strings.Contains(url, "/reel/") {
			return nil
		}
		return fmt.Errorf("Instagram URL必须是用户主页\n格式：instagram.com/username")
	}

	// 检查Twitter/X (用户主页)
	if strings.Contains(url, twitterDomain) || strings.Contains(url, xDomain) {
		// 排除单条推文 /status/
		if !strings.Contains(url, "/status/") {
			return nil
		}
		return fmt.Errorf("Twitter/X URL必须是用户主页\n格式：x.com/username")
	}

	// 检查Twitch (频道)
	if strings.Contains(url, twitchDomain) {
		// 排除具体的视频 /videos/W 或片段 /clip/
		if !strings.Contains(url, "/videos/") && !strings.Contains(url, "/clip/") {
			return nil
		}
		return fmt.Errorf("Twitch URL必须是频道主页\n格式：twitch.tv/username")
	}

	return fmt.Errorf("暂不支持该平台或非订阅链接\n目前支持：YouTube, Bilibili, TikTok, Instagram, Twitter/X, Twitch")
}

func New(dl *download.Downloader) *Manager {
	m := &Manager{
		downloader:   dl,
		ytdlpPath:    utils.GetYtDlpPath(),
		cronMetrics:  &sync.Map{},
		stopChan:     make(chan struct{}),
		initializing: make(map[string]context.CancelFunc),
	}
	m.scanManager = NewScanManager(m) // 初始化扫描管理器
	return m
}

func (m *Manager) GetDownloader() *download.Downloader {
	return m.downloader
}

// GetScanManager 获取扫描管理器 (供 UI 使用)
func (m *Manager) GetScanManager() *ScanManager {
	return m.scanManager
}

// AddSubscription 添加订阅
func (m *Manager) AddSubscription(url string) (*Subscription, error) {
	// 创建可取消的context并注册到初始化跟踪
	ctx, cancel := context.WithCancel(context.Background())
	m.initializingMu.Lock()
	m.initializing[url] = cancel
	m.initializingMu.Unlock()

	// 确保完成后清理
	defer func() {
		m.initializingMu.Lock()
		delete(m.initializing, url)
		m.initializingMu.Unlock()
		cancel()
	}()

	// 3. 尝试获取频道信息 (作为验证)
	info, err := m.fetchPlaylistInfo(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("无法解析该订阅地址: %v", err)
	}

	// 4. 写入数据库
	sub := &Subscription{
		Name:        info.Title,
		URL:         info.OriginalURL,
		Status:      StatusActive,
		Interval:    3600,        // 默认 1 小时
		LastCheckAt: time.Time{}, // 从未检查
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Thumbnail:   info.Thumbnail, // 设置封面
	}

	// 如果 ytdlp 返回了 webpage_url，优先使用它作为规范 URL
	// 但需防止: 用户输入的是 Playlist (带list参数)，但 ytdlp 返回了 Channel URL 或 Single Video URL
	if info.WebpageURL != "" {
		isInputPlaylist := strings.Contains(url, "list=")
		isWebpageChannel := strings.Contains(info.WebpageURL, "/@") || strings.Contains(info.WebpageURL, "/channel/")

		// 如果输入是列表，但解析出的是频道，则保留原始 URL (除非原始 URL 也有问题)
		if isInputPlaylist && isWebpageChannel {
			logger.Warnf("[订阅] 解析到的 WebpageURL (%s) 疑似频道主页，而输入包含 list 参数。强制使用经过验证的输入 URL。", info.WebpageURL)
			sub.URL = url
		} else {
			sub.URL = info.WebpageURL
		}
	} else {
		sub.URL = url
	}

	if err := DAO.Create(sub); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("该订阅已存在")
		}
		return nil, err
	}

	logger.Infof("[订阅] 成功添加订阅: %s", sub.Name)

	// 立即触发首次检查，使用 ScanManager 以便可以管理和取消
	go m.scanManager.ScanOne(sub.ID)

	return sub, nil
}

// AddSubscriptionAsync 异步添加订阅（新增接口，推荐使用）
// onProgress: 进度回调函数
// onSuccess: 成功回调函数
// onError: 错误回调函数
func (m *Manager) AddSubscriptionAsync(url string,
	onProgress func(msg string),
	onSuccess func(sub *Subscription),
	onError func(err error)) {

	go func() {
		if onProgress != nil {
			onProgress("正在解析播放列表...")
		}

		sub, err := m.AddSubscription(url)
		if err != nil {
			if onError != nil {
				onError(err)
			}
			return
		}

		if onSuccess != nil {
			onSuccess(sub)
		}
	}()
}

// CancelInitialization 取消正在进行的订阅初始化
func (m *Manager) CancelInitialization(url string) {
	m.initializingMu.Lock()
	cancel, ok := m.initializing[url]
	m.initializingMu.Unlock()

	if ok {
		logger.Infof("[订阅] 取消订阅初始化: %s", url)
		cancel()
	}
}

// AddSubscriptionWithPlaceholder 立即创建占位订阅记录，然后异步解析更新
// 提供即时UI反馈，让用户立即看到订阅已添加
// onCreate: 创建占位记录后立即回调，传入占位记录
// onSuccess: 解析成功后回调，传入完整记录
// onError: 解析失败后回调
func (m *Manager) AddSubscriptionWithPlaceholder(url string,
	onCreate func(placeholder *Subscription),
	onSuccess func(sub *Subscription),
	onError func(err error)) {

	// 1. 快速创建占位记录
	placeholder := &Subscription{
		Name:     "正在解析 " + url,
		URL:      url,
		Status:   StatusActive,
		Interval: 3600,
	}

	if err := DAO.Create(placeholder); err != nil {
		if onError != nil {
			onError(err)
		}
		return
	}

	// 2. 立即回调，让UI显示占位记录
	if onCreate != nil {
		onCreate(placeholder)
	}

	// 3. 异步解析并更新
	go func() {
		sub, err := m.AddSubscription(url)
		if err != nil {
			// 解析失败，删除占位记录
			DAO.Delete(placeholder.ID)
			if onError != nil {
				onError(err)
			}
			return
		}

		// 解析成功回调
		if onSuccess != nil {
			onSuccess(sub)
		}
	}()
}

// CheckNow 立即检查更新
func (m *Manager) CheckNow(id uint) error {
	// 通过 ScanManager 进行扫描，确保状态统一管理
	m.scanManager.ScanOne(id)
	return nil
}

// StartBackgroundPolling 启动后台轮询
func (m *Manager) StartBackgroundPolling() {
	m.UpdatePolling()
}

// UpdatePolling 更新轮询策略 (在此次会话中停止旧ticker启动新ticker)
// 注意：每次调用都会重置计时器
func (m *Manager) UpdatePolling() {
	cfg := config.Get()

	// 先停止可能存在的旧循环
	m.Stop()
	m.stopChan = make(chan struct{}) // 重置 stopChan

	if !cfg.AutoBackgroundScan {
		logger.Info("[订阅] 后台自动扫描已禁用")
		return
	}

	interval := cfg.BackgroundScanInterval
	if interval <= 0 {
		interval = 10 // 默认 10 分钟
	}

	logger.Infof("[订阅] 启动后台轮询，间隔: %d 分钟", interval)

	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.pollAll()
			case <-m.stopChan:
				ticker.Stop()
				logger.Info("[订阅] 后台轮询已停止")
				return
			}
		}
	}()
}

func (m *Manager) Stop() {
	select {
	case <-m.stopChan:
		// 已经关闭
	default:
		close(m.stopChan)
	}
}

func (m *Manager) pollAll() {
	// 使用 ScanManager 的 ScanAll (它会自动获取所有活跃订阅并批量执行)
	m.scanManager.ScanAll()
}

// ProcessSubscription 处理单个订阅的核心逻辑 (统一入口)
// ctx: 用于取消扫描 (必须)
// onNewVideo: 发现新视频时的回调 (可选)
// 返回: 新增视频数量, 错误
func (m *Manager) ProcessSubscription(ctx context.Context, sub *Subscription, onNewVideo func(entry *playlistEntry, i, total int)) (int, error) {
	// logger.Infof("[订阅] 开始检查更新: %s", sub.Name)

	// // 更新检查时间
	// DAO.UpdateLastCheck(sub.ID)

	// // 1. 获取视频列表 (flat-playlist)
	// entries, err := m.fetchPlaylistEntries(ctx, sub.URL)
	// if err != nil {
	// 	logger.Errorf("[订阅] 拉取列表失败 %s: %v", sub.Name, err)
	// 	return 0, err
	// }

	// newCount := 0
	// existsCnt := 0
	// quitMaxExists := 5 // 连续N个已存在则停止

	// // 记录第一个视频ID（用于下次快速判断）
	// var firstVideoID string
	// if len(entries) > 0 && entries[0].ID != "" {
	// 	firstVideoID = entries[0].ID
	// }

	// // 2. 快速判断：如果第一个视频ID与上次相同，说明无更新
	// if sub.LastVideoID != "" && firstVideoID == sub.LastVideoID {
	// 	logger.Debugf("[订阅] %s 无更新 (LastVideoID 匹配)", sub.Name)
	// 	return 0, nil
	// }

	// // 3. 遍历并去重
	// for i, entry := range entries {
	// 	// 检查 context 是否已取消
	// 	select {
	// 	case <-ctx.Done():
	// 		logger.Infof("[订阅] 扫描已取消: %s", sub.Name)
	// 		return newCount, ctx.Err()
	// 	default:
	// 	}

	// 	// 跳过无效条目
	// 	if entry.ID == "" {
	// 		continue
	// 	}

	// 	// 检查是否已存在于 SubscriptionVideo
	// 	exists, err := DAO.ExistsVideo(sub.ID, entry.ID)
	// 	if err != nil {
	// 		logger.Errorf("DB Error: %v", err)
	// 		continue
	// 	}

	// 	if exists {
	// 		// 如果遇到上次记录的LastVideoID，说明后面的都是旧视频，直接退出
	// 		if entry.ID == sub.LastVideoID {
	// 			logger.Debugf("[订阅] 遇到 LastVideoID，停止扫描")
	// 			break
	// 		}

	// 		// 容错机制：连续N个已存在，认为扫描结束
	// 		existsCnt++
	// 		if existsCnt >= quitMaxExists {
	// 			logger.Debugf("[订阅] 连续 %d 个视频已存在，停止扫描", quitMaxExists)
	// 			break
	// 		} else {
	// 			continue
	// 		}
	// 	} else {
	// 		// 重置连续存在数量
	// 		existsCnt = 0
	// 	}

	// 	// 4. 创建新任务
	// 	logger.Infof("[订阅] 发现新视频: %s (%s)", entry.Title, entry.ID)

	// 	// 复用 createTaskFromEntry 逻辑
	// 	if err := m.createTaskFromEntry(sub, &entry); err != nil {
	// 		logger.Errorf("[订阅] 创建任务失败: %v", err)
	// 		continue
	// 	}

	// 	newCount++

	// 	// 执行回调
	// 	if onNewVideo != nil {
	// 		onNewVideo(&entry, i, len(entries))
	// 	}
	// }

	// // 更新本次会话的新增计数
	// if val, ok := m.cronMetrics.Load(sub.ID); ok {
	// 	metric := val.(*Metrics)
	// 	metric.NewCount += newCount
	// } else {
	// 	m.cronMetrics.Store(sub.ID, &Metrics{NewCount: newCount})
	// }

	// // 4. 更新 LastVideoID
	// if firstVideoID != "" {
	// 	DAO.UpdateLastVideoID(sub.ID, firstVideoID)
	// }

	// logger.Infof("[订阅] 检查结束 %s，新增 %d 个视频", sub.Name, newCount)
	// return newCount, nil
	return rand.IntN(10), nil
}

// createTaskFromEntry 从视频条目创建下载任务 (供 ScanManager 复用)
func (m *Manager) createTaskFromEntry(sub *Subscription, entry *playlistEntry) error {
	logger.Infof("[订阅] 为订阅 %s 创建任务: %s (%s)", sub.Name, entry.Title, entry.ID)

	// 提取封面
	thumb := ""
	if len(entry.Thumbnails) > 0 {
		thumb = entry.Thumbnails[len(entry.Thumbnails)-1].URL
	}

	// 构造 Task
	taskID := uuid.New().String()
	videoURL := entry.URL
	if videoURL == "" {
		return fmt.Errorf("视频条目缺少 URL")
	}

	// 确定下载目录
	cfg := config.Get()
	saveDir := filepath.Join(cfg.DownloadDir, utils.SanitizeFileName(sub.Name))

	newTask := &tasks.Task{
		ID:             taskID,
		URL:            videoURL,
		Status:         tasks.StatusQueued,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Title:          entry.Title,
		Duration:       entry.Duration,
		OutputFolder:   saveDir,
		SubscriptionID: &sub.ID,
		Thumbnail:      thumb,
		FormatID:       config.Get().GetResolutionFormatID(),
		TemplatePath:   config.Get().GetFilenameTemplate(),
	}

	// 入库 Task
	if err := tasks.DAO.CreateTask(newTask); err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	// 记录到 SubscriptionVideo
	subVideo := &SubscriptionVideo{
		SubscriptionID: sub.ID,
		VideoID:        entry.ID,
		Title:          entry.Title,
		URL:            videoURL,
		Duration:       entry.Duration,
		Thumbnail:      thumb,
		CreatedAt:      time.Now(),
	}
	if err := DAO.AddVideo(subVideo); err != nil {
		return fmt.Errorf("记录视频失败: %w", err)
	}

	// 提交给 Downloader 调度
	m.downloader.Schedule(newTask, auth.GetAutherization())

	return nil
}

// --- yt-dlp helpers ---

type playlistEntry struct {
	ID         string  `json:"id"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Duration   float64 `json:"duration"`
	Thumbnails []struct {
		URL    string `json:"url"`
		Height int    `json:"height"`
		Width  int    `json:"width"`
	} `json:"thumbnails"`
}

type playlistInfo struct {
	Title       string `json:"title"`
	WebpageURL  string `json:"webpage_url"`
	OriginalURL string `json:"original_url"`
	Thumbnail   string `json:"thumbnail"` // Added
}

func (m *Manager) fetchPlaylistInfo(ctx context.Context, url string) (*playlistInfo, error) {
	// 获取播放列表元数据 (不含 entries)
	// 使用 --flat-playlist --dump-single-json --playlist-end 1 来获取基本信息
	// 注意：为了获取 Thumbnail，我们需要完整的信息。
	// playlist level json should have 'thumbnails' array or 'thumbnail' string.

	// cmd := exec.Command(m.ytdlpPath,
	// 	"--flat-playlist",
	// 	"--dump-single-json",
	// 	"--playlist-end", "0", // 这里的技巧：即使由 end=0，也能拿到 playlist 本身的信息吗？
	// 	// 实际上 --dump-json 会输出。
	// 	// 安全起见，为了获取标题，我们可能需要解析前几个
	// 	// 或者使用 --print "%(playlist_title)s"
	// 	// 无论是 dump-json 还是 print，都需要网络请求。
	// 	url,
	// )

	// 修正: 为了获取 Title，直接 dump-single-json 应该是最稳的，
	// 但如果列表巨大，能不能只获取元数据？
	// yt-dlp 对于 --dump-single-json 会一次性输出巨大的 JSON。
	// 为了验证 AddSubscription，我们最好只获取 Title。

	// 优化方案: 使用 --print
	// yt-dlp --flat-playlist --print "%(playlist_title)s" --playlist-end 1 URL

	// 为了拿到标准 URL，还是用 dump-json
	cmd := exec.CommandContext(ctx, m.ytdlpPath,
		"--flat-playlist",
		"--dump-single-json",
		"--playlist-end", "1", // 只取第一个，为了快
		url,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var info struct {
		Title       string `json:"title"`
		WebpageURL  string `json:"webpage_url"`
		OriginalURL string `json:"original_url"`
		Type        string `json:"_type"` // Added for validation
		Thumbnail   string `json:"thumbnail"`
		Thumbnails  []struct {
			URL string `json:"url"`
		} `json:"thumbnails"`
	}

	if err := json.Unmarshal(output, &info); err != nil {
		return nil, err
	}

	// Validation: Ensure it's a playlist or channel
	if info.Type == "video" {
		return nil, fmt.Errorf("URL points to a single video, not a playlist or channel")
	}
	// "playlist" is the standard type for channels/playlists.
	// Sometimes it might be empty or valid for other multi-video containers.
	// But "video" is definitely wrong for a subscription.

	// Also filter out generic titles if possible, but let's stick to _type first.
	// If title is "How to automatically...", and it's a video, checking Type="video" fixes it.

	thumb := info.Thumbnail
	if thumb == "" && len(info.Thumbnails) > 0 {
		thumb = info.Thumbnails[len(info.Thumbnails)-1].URL
	}

	return &playlistInfo{
		Title:      info.Title,
		WebpageURL: info.WebpageURL,
		Thumbnail:  thumb,
	}, nil
}

func (m *Manager) fetchPlaylistEntries(ctx context.Context, url string) ([]playlistEntry, error) {
	// yt-dlp --flat-playlist --dump-single-json URL
	// 注意: 如果列表非常大，这可能会返回巨大的 JSON。
	// 但这是 flat-playlist，应该还好。

	cmd := exec.CommandContext(ctx, m.ytdlpPath,
		"--flat-playlist",
		"--dump-single-json",
		url,
	)

	logger.Debugf("正在执行扫描命令: %s", cmd.String())
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Entries []playlistEntry `json:"entries"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	return result.Entries, nil
}
