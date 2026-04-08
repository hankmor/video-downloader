package download

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/hankmor/vdd/core/auth"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/osx"
	"github.com/hankmor/vdd/utils"
)

type core struct {
	ytdlpPath  string
	ffmpegPath string

	mu sync.Mutex
}

func newCore(ytdlpPath, ffmpegPath string) *core {
	return &core{
		ytdlpPath:  ytdlpPath,
		ffmpegPath: ffmpegPath,
	}
}

func (c *core) isMergeToMP4() bool {
	return c.ffmpegPath != ""
}

func (c *core) buildCmd(ctx *DownloadContext, authInfo *auth.AuthInfo) (*exec.Cmd, error) {
	logger.Debugf("[下载] 正在构建 yt-dlp 参数 %s", ctx.Task.ID)
	cfg := config.Get()

	// 修复旧版本或错误逻辑导致的 FormatID 异常 (user report: +bestaudio//best)
	// 如果 FormatID 为空，或者看起来像之前的错误格式，则使用当前配置的默认值
	if ctx.Task.FormatID == "" || strings.Contains(ctx.Task.FormatID, "+bestaudio") {
		ctx.Task.FormatID = cfg.GetResolutionFormatID()
		logger.Warnf("[下载] 检测到无效或为空的 FormatID，已自动修正为: %s", ctx.Task.FormatID)
	}

	// 按配置执行画质上限
	if maxQ := authInfo.UserMaxQuality(); maxQ > 0 {
		// e.g. bestvideo[height<=720]
		currentFormat := ctx.Task.FormatID
		// 如果是 bestvideo，则追加限制
		if currentFormat == "bestvideo" {
			ctx.Task.FormatID = fmt.Sprintf("bestvideo[height<=%d]", maxQ)
		} else if !strings.Contains(currentFormat, "height<=") {
			// 如果是其他格式且没指定高度限制，强制覆盖?
			// 简单起见，直接覆盖为限制格式，或者解析现有的...
			// 最稳妥的方式: 强制使用限制格式
			ctx.Task.FormatID = fmt.Sprintf("bestvideo[height<=%d]", maxQ)
		} else {
			// 已经有限制，假设它符合要求 (或者解析并替换，比较复杂，暂略)
			// 这里简单处理：如果包含 height<=，我们还需要检查具体的数值吗？
			// 鉴于 UserMaxQuality 是强制限制，最好是强制覆盖
			ctx.Task.FormatID = fmt.Sprintf("bestvideo[height<=%d]", maxQ)
		}
		logger.Infof("[下载] 画质限制为 <=%dp", maxQ)
	}

	// 同样检查 TemplatePath，防止 -o 为空
	if ctx.Task.TemplatePath == "" {
		ctx.Task.TemplatePath = cfg.GetFilenameTemplate()
		logger.Warnf("[下载] 检测到为空的 TemplatePath，已自动修正为: %s", ctx.Task.TemplatePath)
	}

	formatSelector := ctx.Task.FormatID + "+bestaudio/" + ctx.Task.FormatID + "/best"
	ytdlpPath := c.ytdlpPath

	args := []string{
		"-f", formatSelector,
		"-o", ctx.Task.TemplatePath,
		"--newline",
	}

	// 支持自定义输出目录 (例如订阅文件夹)
	if ctx.Task.OutputFolder != "" {
		args = append(args, "-P", ctx.Task.OutputFolder)
	}

	if cfg.ProxyURL != "" {
		args = append(args, "--proxy", cfg.ProxyURL)
	}

	if cfg.Subtitle {
		// 下载自动字幕和人工字幕
		args = append(args, "--write-subs", "--write-auto-subs")
		langs := cfg.SubtitleLangs
		if langs == "" {
			langs = "zh-Hans,en"
		}
		args = append(args, "--sub-lang", langs)
	}

	// 速率限制
	if authInfo.UserRateLimit() != "" {
		args = append(args, "--limit-rate", authInfo.UserRateLimit())
	}

	// Cookie 设置
	// 从 Task 获取 CookieFile (需要先将 Task 结构体中新增的字段 plumb 进来)
	// Downloader 的参数 task 是 *tasks.Task，已经包含了 CookieFile 字段
	if utils.FileExists(ctx.Task.CookieFile) {
		logger.Debugf("[下载] 使用 Cookie 文件 %s", ctx.Task.CookieFile)
		args = append(args, "--cookies", ctx.Task.CookieFile)
	} else if cfg.EnableBrowserCookie && cfg.BrowserName != "" {
		logger.Debugf("[下载] 使用浏览器 Cookie %s", cfg.BrowserName)
		args = append(args, "--cookies-from-browser", cfg.BrowserName)
	}
	// 旧逻辑移除: if cfg.CookiesPath != "" ...

	logger.Debugf("[下载] 正在解析 ffmpeg 路径 %s", ctx.Task.ID)

	if c.ffmpegPath != "" {
		args = append(args, "--ffmpeg-location", c.ffmpegPath)
		// 合并为 mp4 格式
		args = append(args, "--merge-output-format", "mp4")
	} else {
		// 没有找到 ffmpeg，让 yt-dlp 自己尝试查找
		logger.Warnf("[下载] 未找到有效的 ffmpeg 路径，合并功能可能不可用")
	}

	logger.Infof("[下载] 开始下载任务 %s", ctx.Task.ID)

	args = append(args, ctx.Task.URL)
	cmd := exec.CommandContext(ctx.Context, ytdlpPath, args...)

	logger.Debugf("[下载] 正在执行 yt-dlp 命令: %s", cmd.String())
	logger.Debugf("[下载] 正在设置进程组，以便能暂停整个进程树 %s", ctx.Task.ID)

	// 设置进程组，以便能暂停整个进程树
	osx.SetProcessGroup(cmd)
	osx.SetCmdHideWindow(cmd)

	return cmd, nil
}

func (c *core) startCmd(ctx *DownloadContext,
	onStage func(taskID string, stage string) error,
	onProgress func(taskID string, progress *Progress),
	onSuccess func(taskID string, filename string),
	onCancel func(taskID string),
	onError func(taskID string, err error)) {

	stdout, err := ctx.Cmd.StdoutPipe()
	if err != nil {
		onError(ctx.Task.ID, err)
		return
	}

	stderr, err := ctx.Cmd.StderrPipe()
	if err != nil {
		onError(ctx.Task.ID, err)
		return
	}

	logger.Debugf("[下载] 正在启动 yt-dlp 命令 %s", ctx.Task.ID)

	// 启动命令
	if err := ctx.Cmd.Start(); err != nil {
		onError(ctx.Task.ID, err)
		return
	}

	// 监控 stderr
	var stderrBuf strings.Builder
	go func() {
		logger.Debugf("[下载] 正在监控 stderr %s", ctx.Task.ID)

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// logger.Debugf("[下载] stderr: %s", line) // Optional verbose log

			// 收集错误信息 (限制大小)
			if stderrBuf.Len() < 4096 {
				stderrBuf.WriteString(line + "\n")
			}

			if strings.Contains(line, "[Merger]") {
				onStage(ctx.Task.ID, "正在合并音视频...")
			}
		}
	}()

	// 监控 stdout (进度)
	downloadCount := 0
	var finalFilename string

	logger.Debugf("[下载] 正在监控 stdout %s", ctx.Task.ID)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		// 如果 Context 已取消，提前退出循环
		if ctx.Context.Err() != nil {
			break
		}
		line := scanner.Text()

		logger.Debugf("[下载] stdout: %s", line)

		// 捕获文件名逻辑
		// 1. 合并模式: [Merger] Merging formats into "Filename"
		if strings.Contains(line, "[Merger] Merging formats into") {
			parts := strings.Split(line, "\"")
			if len(parts) >= 2 {
				finalFilename = parts[1]
			}
		}
		// 2. 直接下载: [download] Destination: Filename
		// 只有当 finalFilename 为空时才记录，因为 Merger 通常是最后一步
		if finalFilename == "" && strings.Contains(line, "[download] Destination:") {
			finalFilename = strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
		}
		// 3. 已存在: [download] Filename has already been downloaded
		if finalFilename == "" && strings.Contains(line, "has already been downloaded") {
			parts := strings.Split(line, "[download] ")
			if len(parts) >= 2 {
				finalFilename = strings.TrimSuffix(parts[1], " has already been downloaded")
			}
		}
		// 4. M3U8 Fixup: [FixupM3u8] Saving to: Filename
		if finalFilename == "" && strings.Contains(line, "[FixupM3u8] Saving to:") {
			finalFilename = strings.TrimSpace(strings.TrimPrefix(line, "[FixupM3u8] Saving to:"))
		}

		// 详细状态解析 - 实时反馈给用户
		var newStage string
		if strings.Contains(line, "Extracting cookies from chrome") {
			newStage = "正在解析Cookie"
		} else if strings.Contains(line, "Extracting URL:") {
			newStage = "正在解析视频地址"
		} else if strings.Contains(line, "Downloading webpage") {
			newStage = "正在读取视频信息"
		} else if strings.Contains(line, "Downloading m3u8 information") {
			newStage = "正在获取流媒体信息"
		} else if strings.Contains(line, "Downloading android player API JSON") {
			newStage = "正在获取播放接口"
		} else if strings.Contains(line, "[ExtractAudio]") {
			newStage = "正在提取音频"
		} else if strings.Contains(line, "[Merger]") {
			newStage = "正在合并音视频"
		} else if strings.Contains(line, "[Metadata]") {
			newStage = "正在写入元数据"
		} else if strings.Contains(line, "[EmbedSubtitle]") {
			newStage = "正在嵌入字幕"
		} else if strings.Contains(line, "[FixupM3u8]") {
			newStage = "正在修复流媒体片段"
		} else if strings.Contains(line, "Deleting original file") {
			newStage = "正在清理临时文件"
		} else if strings.Contains(line, "Got error:") {
			idx := strings.Index(line, "Got error:")
			newStage = utils.EclipseString(line[idx+10:], 50)
		}

		// 如果检测到新状态，更新 Status
		if newStage != "" {
			onStage(ctx.Task.ID, newStage)
		}

		// 检测新文件下载开始
		if strings.Contains(line, "[download] Destination:") {
			downloadCount++
			stage := fmt.Sprintf("正在下载流 %d...", downloadCount)
			// 通常第一个是视频（或者唯一的文件）
			if downloadCount == 1 {
				stage = "正在下载视频..."
			} else if downloadCount == 2 {
				stage = "正在下载音频..."
			}
			onStage(ctx.Task.ID, stage)
		}

		logger.Debugf("[下载] 正在解析进度 %s", ctx.Task.ID)

		// 解析进度
		progress := parseProgress(line)
		if progress != nil {
			onProgress(ctx.Task.ID, progress)
		}
	}

	// 等待命令完成
	if err := ctx.Cmd.Wait(); err != nil {
		if ctx.Context.Err() == context.Canceled {
			logger.Debugf("[下载] 任务 %s 被取消", ctx.Task.ID)

			onCancel(ctx.Task.ID)
			return
		}

		// 提取详细错误
		detailError := err.Error()
		if stderrBuf.Len() > 0 {
			lines := strings.Split(strings.TrimSpace(stderrBuf.String()), "\n")
			if len(lines) > 0 {
				// 优先找 ERROR: 开头的行
				foundError := false
				for i := len(lines) - 1; i >= 0; i-- {
					if strings.Contains(lines[i], "ERROR:") {
						detailError = lines[i]
						foundError = true
						break
					}
				}
				// 没找到 ERROR 则用最后一行
				if !foundError {
					detailError = lines[len(lines)-1]
				}
			}
		}

		logger.Debugf("[下载] 任务 %s 下载失败: %v, 详细: %s", ctx.Task.ID, err, detailError)

		onError(ctx.Task.ID, fmt.Errorf("下载失败: %v", detailError))

		return
	}

	onSuccess(ctx.Task.ID, finalFilename)
}

// Progress, parseProgress, formatSpeedLog, parseSize, fileExists helper functions remain same
// We can copy them from previous version
type Progress struct {
	Percent    float64
	Speed      int64
	Downloaded int64
	Total      int64
	ETA        int
}

// 预编译正则，避免在循环中重复编译，这在 10 年经验的开发者看来是基本常识
var (
	// HLS/DASH 格式（带片段信息）: [download]   8.0% of ~  24.21MiB at   99.50KiB/s ETA 05:29 (frag 3/39)
	progressReWithFrag = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?\s*(\d+\.?\d*)\s*([KMG]?)iB\s+at\s+(\d+\.?\d*)\s*([KMG]?)iB/s\s+ETA\s+(\d+):(\d+)`)

	// 标准格式: [download]  45.2% of 234.5MiB at 1.2MiB/s ETA 02:34
	progressReStandard = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?\s*(\d+\.?\d*)([KMG]?)iB\s+at\s+(\d+\.?\d*)([KMG]?)iB/s\s+ETA\s+(\d+):(\d+)`)

	// 完成格式: [download] 100% of 234.5MiB in 02:34
	// 注意：完成格式通常只有 Total 和 Time，没有 Speed 和 ETA，需要特殊处理
	progressReCompleted = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?\s*(\d+\.?\d*)([KMG]?)iB\s+in\s+(\d+):(\d+)`)
)

func parseProgress(line string) *Progress {
	var matches []string

	if matches = progressReWithFrag.FindStringSubmatch(line); len(matches) >= 6 {
		return extractProgress(matches, true)
	}
	if matches = progressReStandard.FindStringSubmatch(line); len(matches) >= 6 {
		return extractProgress(matches, true)
	}
	if matches = progressReCompleted.FindStringSubmatch(line); len(matches) >= 6 {
		return extractProgress(matches, false)
	}

	return nil
}

func extractProgress(matches []string, hasSpeedAndETA bool) *Progress {
	percent, _ := strconv.ParseFloat(matches[1], 64)
	total, _ := strconv.ParseFloat(matches[2], 64)
	totalUnit := matches[3]

	var speed float64
	var speedUnit string
	var etaMin, etaSec int

	if hasSpeedAndETA {
		// 格式: ... at SPEED ETA MIN:SEC
		speed, _ = strconv.ParseFloat(matches[4], 64)
		speedUnit = matches[5]
		etaMin, _ = strconv.Atoi(matches[6])
		etaSec, _ = strconv.Atoi(matches[7])
	} else {
		// 完成格式: ... in MIN:SEC (这里把 "in" 的时间当做 ETA=0 或者 total time，但 Current Progress 结构体只有 ETA)
		// 完成时 ETA 为 0
		etaMin = 0
		etaSec = 0
	}

	return &Progress{
		Percent:    percent,
		Total:      parseSize(total, totalUnit),
		Speed:      parseSize(speed, speedUnit),
		Downloaded: int64(float64(parseSize(total, totalUnit)) * percent / 100),
		ETA:        etaMin*60 + etaSec,
	}
}

func parseSize(value float64, unit string) int64 {
	multiplier := 1.0
	switch unit {
	case "K":
		multiplier = 1024
	case "M":
		multiplier = 1024 * 1024
	case "G":
		multiplier = 1024 * 1024 * 1024
	}
	return int64(value * multiplier)
}
