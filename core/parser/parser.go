package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/consts"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/osx"
	"github.com/hankmor/vdd/utils"
)

// VideoParser 视频解析器
type VideoParser struct {
	ytdlpPath string
}

// New 创建新的视频解析器
func New(ytdlpPath string) *VideoParser {
	return &VideoParser{
		ytdlpPath: ytdlpPath,
	}
}

// VideoInfo 视频信息
type VideoInfo struct {
	ID          string   `json:"id"`
	WebpageURL  string   `json:"webpage_url"` // 原视频链接
	Title       string   `json:"title"`
	Uploader    string   `json:"uploader"`
	UploadDate  string   `json:"upload_date"` // 上传日期 (YYYYMMDD)
	Duration    float64  `json:"duration"`    // 改为 float64，yt-dlp 返回浮点数
	Thumbnail   string   `json:"thumbnail"`
	Description string   `json:"description"`
	Formats     []Format `json:"formats"`
}

// Format 视频格式信息
type Format struct {
	FormatID       string  `json:"format_id"`
	Extension      string  `json:"ext"`
	Resolution     string  `json:"resolution"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FPS            float64 `json:"fps"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	ABR            float64 `json:"abr"`
	VBR            float64 `json:"vbr"`
	FileSize       int64   `json:"filesize"`
	FileSizeApprox int64   `json:"filesize_approx"`

	// 扩展字段
	HasVideo    bool
	HasAudio    bool
	Recommended bool
	Limited     bool
}

// ResolutionDimension 获取分辨率维度 (取宽高中的较小值，以适配横竖屏)
func (f Format) ResolutionDimension() int {
	return utils.Min(f.Width, f.Height)
}

// ParseVideo 解析视频信息 (兼容旧接口)
func (p *VideoParser) ParseVideo(url string) (*VideoInfo, error) {
	return p.ParseVideoWithContext(context.Background(), url)
}

// ParseVideoWithContext 解析视频信息 (支持 Context 取消)
func (p *VideoParser) ParseVideoWithContext(ctx context.Context, url string) (*VideoInfo, error) {
	args := []string{"--dump-json"}

	// 检查代理设置
	cfg := config.Get()
	if cfg.ProxyURL != "" {
		args = append(args, "--proxy", cfg.ProxyURL)
	}

	// 检查 Cookie 设置
	// 检查 Cookie 设置
	// 优先级 1: Context 中的覆盖 (任务级)
	// 优先级 2: 全局浏览器配置
	// 优先级 3: (旧) 全局文件配置 (Config.CookiesPath 现在仅作为记忆，不再自动生效，除非通过 Context 传入)
	// 注意：Context 传递的 cookieFile 来自于 UI 上用户的明确选择
	
	if cookieFile, ok := ctx.Value(consts.CtxKeyCookieFile).(string); ok && cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
	} else if cfg.EnableBrowserCookie && cfg.BrowserName != "" {
		args = append(args, "--cookies-from-browser", cfg.BrowserName)
	}

	// 移除旧的直接读取 cfg.CookiesPath 逻辑
	// if cfg.CookiesPath != "" { ... }

	args = append(args, url)

	// 使用 CommandContext
	cmd := exec.CommandContext(ctx, p.ytdlpPath, args...)

	// 设置进程组，以便能更好地控制进程终止
	osx.SetProcessGroup(cmd)
	osx.SetCmdHideWindow(cmd)

	// 同时捕获 stdout 和 stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Debugf("[解析] 执行 yt-dlp: %s %v", p.ytdlpPath, args)

	// 使用 Start + Wait 的异步模式，而不是 Run
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 yt-dlp 失败: %w", err)
	}

	// 使用 WaitGroup 确保监控 goroutine 完成
	var wg sync.WaitGroup
	wg.Add(1)

	// 通道用于通知监控协程命令已完成
	done := make(chan struct{})

	// 监控 context 取消，如果取消则强制终止进程
	go func() {
		defer wg.Done()
		select {
		case <-done:
			// 命令正常结束，无需操作
			return
		case <-ctx.Done():
			// Context 被取消，使用进程树终止
			if cmd.Process != nil {
				logger.Debugf("[解析] Context 已取消，终止进程树: %s", url)
				if err := osx.KillProcessTree(cmd); err != nil {
					logger.Warnf("[解析] 终止进程树失败: %v", err)
				}
			}
		case <-time.After(3 * time.Minute):
			// 超时保护：强制终止进程
			logger.Warnf("[解析] 解析视频超时 (3分钟)，正在终止进程: %s", url)
			if cmd.Process != nil {
				if err := osx.KillProcessTree(cmd); err != nil {
					logger.Warnf("[解析] 超时终止进程失败: %v", err)
				}
			}
		}
	}()

	logger.Debugf("[解析] 等待进程完成: %s", url)
	// 等待进程完成
	err := cmd.Wait()
	close(done) // 通知监控协程退出
	wg.Wait()   // 等待监控 goroutine 完成

	if err != nil {
		// 检查由于 Context 取消导致的错误
		if ctx.Err() == context.Canceled {
			logger.Infof("[解析] 解析视频被取消: %s", url)
			return nil, context.Canceled
		}

		// 提供详细的错误信息
		errMsg := fmt.Sprintf("解析视频失败: %v", err)
		if stderr.Len() > 0 {
			errMsg += fmt.Sprintf("\nyt-dlp 错误: %s", stderr.String())
		}
		logger.Errorf("[解析] 解析视频失败: %s: %s", url, errMsg)
		return nil, fmt.Errorf("解析失败: %s", err.Error())
	}

	output := stdout.Bytes()
	if len(output) == 0 {
		logger.Errorf("[解析] 解析视频失败: %s: yt-dlp 未返回任何数据，stderr: %s", url, stderr.String())
		return nil, fmt.Errorf("yt-dlp 未返回任何数据，stderr: %s", stderr.String())
	}

	var info VideoInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w\n原始输出前100字符: %s", err, string(output[:min(100, len(output))]))
	}

	// 处理格式信息
	for i := range info.Formats {
		format := &info.Formats[i]

		// 使用 resolution 字段作为主要判断依据（更可靠）
		// "audio only" 表示只有音频
		// 有分辨率值（如 "1920x1080"）表示有视频
		isAudioOnly := strings.Contains(strings.ToLower(format.Resolution), "audio only")

		// 判断是否有视频
		// 1. resolution 不是 "audio only"
		// 2. 有 vcodec 且不为 "none"
		// 3. 或者 video_ext 不为 "none"
		format.HasVideo = !isAudioOnly &&
			(format.VCodec != "" && format.VCodec != "none") &&
			format.ResolutionDimension() > 0

		// 判断是否有音频
		// 1. resolution 是 "audio only"，或
		// 2. 有 acodec 且不为 "none"
		format.HasAudio = isAudioOnly ||
			(format.ACodec != "" && format.ACodec != "none")

		// 如果没有文件大小，使用近似大小
		if format.FileSize == 0 && format.FileSizeApprox > 0 {
			format.FileSize = format.FileSizeApprox
		}

		// 如果仍然是0，尝试估算（使用时长和比特率）
		// 注意：某些流媒体格式可能无法提前知道大小
		if format.FileSize == 0 {
			// 使用 tbr (总比特率) 或 vbr+abr 估算
			// 大小 = (比特率 kbps / 8) * 时长秒数
			// 这只是估算，实际大小可能不同
		}
	}

	logger.Infof("[解析] 成功解析视频: %s (时长: %.2fs)", info.Title, info.Duration)
	return &info, nil
}

// GetFormats 获取所有可用格式
func (p *VideoParser) GetFormats(url string) ([]Format, error) {
	info, err := p.ParseVideo(url)
	if err != nil {
		return nil, err
	}

	return info.Formats, nil
}

// CheckVersion 检查 yt-dlp 版本
func (p *VideoParser) CheckVersion() (string, error) {
	cmd := exec.Command(p.ytdlpPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("无法获取版本信息: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// FormatDuration 格式化时长
func FormatDuration(seconds int) string {
	duration := time.Duration(seconds) * time.Second
	h := int(duration.Hours())
	m := int(duration.Minutes()) % 60
	s := int(duration.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// SourceFromURL 从 URL 中解析出视频源渠道
func SourceFromURL(url string) string {
	url = strings.ToLower(url)

	switch {
	case strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be"):
		return consts.SourceYoutube
	case strings.Contains(url, "bilibili.com") || strings.Contains(url, "b23.tv"):
		return consts.SourceBilibili
	case strings.Contains(url, "youku.com"):
		return consts.SourceYouku
	case strings.Contains(url, "facebook.com") || strings.Contains(url, "fb.watch"):
		return consts.SourceFacebook
	case strings.Contains(url, "twitter.com") || strings.Contains(url, "x.com"):
		return consts.SourceTwitter
	case strings.Contains(url, "tiktok.com"):
		return consts.SourceTiktok
	case strings.Contains(url, "instagram.com"):
		return consts.SourceInstagram
	case strings.Contains(url, "douyin.com") || strings.Contains(url, "iesdouyin.com"):
		return consts.SourceDouyin
	case strings.Contains(url, "vimeo.com"):
		return consts.SourceVimeo
	case strings.Contains(url, "twitch.tv"):
		return consts.SourceTwitch
	case strings.Contains(url, "reddit.com"):
		return consts.SourceReddit
	case strings.Contains(url, "kuaishou.com") || strings.Contains(url, "kwai.com"):
		return consts.SourceKuaishou
	case strings.Contains(url, "ixigua.com"):
		return consts.SourceIxigua
	case strings.Contains(url, "v.qq.com"):
		return consts.SourceTencent
	case strings.Contains(url, "iqiyi.com"):
		return consts.SourceIqiyi
	case strings.Contains(url, "xiaohongshu.com") || strings.Contains(url, "xhslink.com"):
		return consts.SourceXiaohongshu
	default:
		return consts.SourceUnknown
	}
}
