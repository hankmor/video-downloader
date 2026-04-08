package utils

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/hankmor/vdd/core/logger"
	_ "golang.org/x/image/webp" // 支持 WebP 格式
)

// DownloadImage 下载图片数据，支持代理
func DownloadImage(imageURL string, proxyURL string) ([]byte, error) {
	// 创建 HTTP Client
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	// 如果配置了代理，设置 Transport
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("解析代理地址失败: %v", err)
		}

		transport := &http.Transport{
			Proxy:           http.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 宽松一点，防止证书问题
		}
		client.Transport = transport
	}

	// 创建请求
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, err
	}
	// 伪装成浏览器，防止被服务器 (如 Twitter) 拒绝
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 发起请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载图片失败，状态码: %d", resp.StatusCode)
	}

	// 读取数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// LoadOrDownloadThumbnail 加载或下载缩略图
// 如果本地缓存存在，直接返回文件路径；不存在则下载并保存到本地
// 返回: 本地文件路径, 是否从缓存加载, 错误
func LoadOrDownloadThumbnail(imageURL string, proxyURL string) (string, bool, error) {
	if imageURL == "" {
		return "", false, fmt.Errorf("图片URL为空")
	}

	// 生成缓存文件路径
	cachePath := GetThumbnailCachePath(imageURL)

	// 检查本地缓存是否存在
	if fileExists(cachePath) {
		return cachePath, true, nil
	}

	// 本地不存在，下载图片
	data, err := DownloadImage(imageURL, proxyURL)
	if err != nil {
		return "", false, fmt.Errorf("下载图片失败: %w", err)
	}

	// 压缩图片到100px
	compressedData, err := CompressImage(data, 150)
	if err != nil {
		// 压缩失败时使用原始数据（降级处理）
		logger.Errorf("[缩略图] 图片压缩失败，使用原始数据: %v", err.Error())
		compressedData = data
	}

	// 确保缓存目录存在
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", false, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 保存压缩后的图片到本地
	if err := os.WriteFile(cachePath, compressedData, 0644); err != nil {
		return "", false, fmt.Errorf("保存缓存文件失败: %w", err)
	}

	return cachePath, false, nil
}
