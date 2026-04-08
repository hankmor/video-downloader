package utils

import (
	"strings"
)

// GetUserFriendlyError 将底层错误转换为用户友好的提示信息
// GetUserFriendlyError 将底层错误转换为用户友好的提示信息
func GetUserFriendlyError(err error) string {
	if err == nil {
		return ""
	}
	
	msg := err.Error()

	// 1. 预处理：移除我们自己添加的技术前缀，只保留核心错误
	msg = strings.ReplaceAll(msg, "解析视频失败: ", "")
	msg = strings.ReplaceAll(msg, "解析 JSON 失败: ", "")
	msg = strings.TrimSpace(msg)

	// 2. 匹配具体错误场景

	// JSON 解析错误 (通常意味着 yt-dlp 输出格式变了，或者反爬虫导致返回了 HTML 而不是 JSON)
	if strings.Contains(msg, "json: cannot unmarshal") || strings.Contains(msg, "invalid character") {
		return "解析视频数据失败，可能是网站接口变更或反爬虫拦截"
	}

	// 网页加载失败 (网络或 Geo 限制)
	if strings.Contains(msg, "Unable to download webpage") || strings.Contains(msg, "Name or service not known") {
		return "无法加载网页，请检查链接是否正确，或是否需要代理"
	}

	// 核心算法失效 (需要更新 yt-dlp)
	if strings.Contains(msg, "nsig extraction failed") {
		return "解析核心失效 (nsig)，请等待 VDD 软件升级以修复"
	}

	// 通用解析错误 (ExtractorError)
	if strings.Contains(msg, "ExtractorError") {
		return "无法识别该页面内容，可能是因为网站结构已变更"
	}

	// 网络连接问题
	if strings.Contains(msg, "dial tcp") || strings.Contains(msg, "timeout") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "client check failed") {
		return "网络连接失败，请检查网络或代理设置"
	}
	
	// 403 Forbidden / 私密视频
	if strings.Contains(msg, "HTTP Error 403") || strings.Contains(msg, "Private video") || strings.Contains(msg, "This video is private") {
		return "无法访问视频，可能是私密视频或需要登录/会员权限"
	}

	// 视频不可用 / 已删除
	if strings.Contains(msg, "Video unavailable") || strings.Contains(msg, "uploader has not made this video available") || strings.Contains(msg, "This video has been removed") {
		return "该视频已被发布者删除或暂不可用"
	}

	// 年龄限制
	if strings.Contains(msg, "Sign in to confirm your age") || strings.Contains(msg, "age-gated") {
		return "该视频有年龄限制，请尝试在浏览器登录并提取 Cookies"
	}

	// 缺少组件
	if strings.Contains(msg, "ffmpeg not found") || strings.Contains(msg, "executable file not found") {
		if strings.Contains(msg, "ffmpeg") {
			return "未找到 FFmpeg 组件，无法合并音视频"
		}
		return "找不到核心组件 (yt-dlp)，请重新安装软件"
	}
	
	// 直播中
	if strings.Contains(msg, "is live") {
		return "该视频正在直播中，暂时无法下载"
	}

	// Python/yt-dlp 退出状态
	if strings.Contains(msg, "exit status") {
		// 尝试从 msg 中提取更有用的信息，如果只是 exit status 1，那就只能报未知
		if len(msg) < 20 { // "exit status 1" is short
			return "解析服务异常退出，请重试"
		}
	}

	// 默认返回原始错误（截断过长信息）
	// 移除可能存在的换行符，让显示更紧凑
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 80 {
		return "未知错误: " + msg[:77] + "..."
	}
	return "错误: " + msg
}
