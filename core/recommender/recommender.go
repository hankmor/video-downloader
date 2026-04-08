package recommender

import (
	"sort"
	"strings"

	"github.com/hankmor/vdd/core/parser"
)

// FormatRecommender 格式推荐引擎
type FormatRecommender struct{}

// New 创建新的推荐引擎
func New() *FormatRecommender {
	return &FormatRecommender{}
}

// ScoredFormat 带评分的格式
type ScoredFormat struct {
	Format parser.Format
	Score  float64
}

// Recommend 推荐最佳格式
// maxQuality: 最高画质限制（0表示无限制）
func (r *FormatRecommender) Recommend(formats []parser.Format, maxQuality int) []parser.Format {
	// 1. 过滤：保留所有视频格式 (不再强制要求有音频，下载时会自动合并)
	combined := filterVideoFormats(formats)

	if len(combined) == 0 {
		// 如果没有合并格式，返回所有格式
		combined = formats
	}

	// 2. 评分
	scored := make([]ScoredFormat, 0, len(combined))
	for _, f := range combined {
		score := calculateScore(f)
		scored = append(scored, ScoredFormat{
			Format: f,
			Score:  score,
		})
	}

	// 3. 排序（优先分辨率由高到低，其次按分数）
	sort.Slice(scored, func(i, j int) bool {
		// 优先比较分辨率 (使用短边)
		dimI := scored[i].Format.ResolutionDimension()
		dimJ := scored[j].Format.ResolutionDimension()
		if dimI != dimJ {
			return dimI > dimJ
		}
		// 分辨率相同则比较综合评分 (编码、格式等)
		return scored[i].Score > scored[j].Score
	})

	// 4. 标记推荐（第一项，且必须符合画质限制）
	result := make([]parser.Format, len(scored))
	foundRecommended := false

	// 先拷贝结果
	for i, sf := range scored {
		result[i] = sf.Format
	}

	// 再次遍历找到第一个符合条件的标记为推荐
	for i := range result {
		// 如果有限制，跳过超标的
		dim := result[i].ResolutionDimension()
		if maxQuality > 0 && dim > maxQuality {
			result[i].Limited = true
			continue
		}
		result[i].Recommended = true
		result[i].Limited = false
		foundRecommended = true
		break
	}

	// 如果所有都超标（极少见），但这不应该发生，除非只有 4K 资源
	// 这种情况下由于我们只是"推荐"，用户还是无法下载，所以这里不强制标记推荐也可以，
	// 或者标记第一个（虽然它会被锁住）
	if !foundRecommended && len(result) > 0 {
		result[0].Recommended = true
	}

	return result
}

// filterVideoFormats 过滤出包含视频的格式 (无论是否有音频)
func filterVideoFormats(formats []parser.Format) []parser.Format {
	combined := make([]parser.Format, 0)
	for _, f := range formats {
		// 只要有视频流就保留
		if f.HasVideo {
			combined = append(combined, f)
		}
	}
	return combined
}

// calculateScore 计算格式评分
// 总分 100 分，各项权重：
// - 音视频完整性：40%（最重要，确保推荐完整格式）
// - 分辨率：30%
// - 编码质量：20%
// - 文件格式：10%
func calculateScore(f parser.Format) float64 {
	score := 0.0

	// 1. 媒体类型评分 (40分)
	// 只要有视频，就给与高分 (因为下载器会自动处理音频合并)
	if f.HasVideo {
		score += 40.0
	} else if f.HasAudio {
		score += 10.0 // 仅音频
	}
	// 如果是原生合并格式，额外加一点点分 (0.5)，优先于同分辨率的非合并格式？
	// 或者反过来：非合并格式通常画质更好(更高码率)，所以这里不加分，完全看分辨率和编码。
	if f.HasVideo && f.HasAudio {
		score += 0.5 // 稍微优先原生合并，如果是同分辨率同编码
	}

	// 2. 分辨率评分 (满分60分)
	score += resolutionScore(f.ResolutionDimension())

	// 3. 编码评分 (20分)
	score += codecScore(f.VCodec, f.ACodec)

	// 4. 格式评分 (10分)
	score += formatScore(f.Extension)

	return score
}

// resolutionScore 分辨率评分
func resolutionScore(dim int) float64 {
	switch {
	case dim >= 4320: // 8K
		return 60.0
	case dim >= 2160: // 4K
		return 50.0 // 必须远高于 1080p + 优质编码
	case dim >= 1440: // 2K
		return 40.0 // 必须远高于 1080p + 优质编码
	case dim >= 1080: // 1080p
		return 30.0
	case dim >= 720: // 720p
		return 20.0
	case dim >= 480: // 480p
		return 10.0
	case dim > 0: // 其他分辨率
		return 5.0
	default: // 音频或未知
		return 0.0
	}
}

// formatScore 格式评分 (满分10分)
func formatScore(ext string) float64 {
	switch strings.ToLower(ext) {
	case "mp4":
		return 10.0
	case "mkv":
		return 9.0
	case "webm":
		return 7.5
	case "m4a":
		return 6.0
	default:
		return 5.0
	}
}

// codecScore 编码评分
func codecScore(vcodec, acodec string) float64 {
	score := 0.0

	// 视频编码
	vcodec = strings.ToLower(vcodec)
	if strings.Contains(vcodec, "h264") || strings.Contains(vcodec, "avc") {
		score += 10.0
	} else if strings.Contains(vcodec, "h265") || strings.Contains(vcodec, "hevc") {
		score += 9.0
	} else if strings.Contains(vcodec, "vp9") {
		score += 8.0
	} else if vcodec != "" && vcodec != "none" {
		score += 5.0
	}

	// 音频编码
	acodec = strings.ToLower(acodec)
	if strings.Contains(acodec, "aac") {
		score += 10.0
	} else if strings.Contains(acodec, "opus") {
		score += 9.0
	} else if strings.Contains(acodec, "mp3") {
		score += 8.0
	} else if acodec != "" && acodec != "none" {
		score += 5.0
	}

	return score
}
