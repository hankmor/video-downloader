package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ReleaseInfo 代表 GitHub Release 信息
type ReleaseInfo struct {
	TagName string `json:"tag_name"` // Git Tag (e.g., v1.0.1)
	Name    string `json:"name"`     // Release 标题
	Body    string `json:"body"`     // Release 说明
	HTMLURL string `json:"html_url"` // Release 页面链接
}

// CheckForUpdates 检查更新
// currentVersion: 当前版本号 (e.g., v1.0.1)
// repo: GitHub 仓库 (e.g., hankmor/vdd)
// 返回:
// - *ReleaseInfo: 如果有新版本，返回新版本信息；否则返回 nil
// - error: 请求错误
func CheckForUpdates(currentVersion, repo string, progress func(float64)) (*ReleaseInfo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	progress(0.1)

	// 创建 context 用于控制进度更新 goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动进度更新 goroutine：从 0.1 逐步增加到 0.9
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond) // 每 100ms 更新一次
		defer ticker.Stop()

		currentProgress := 0.1
		increment := 0.02 // 每次增加 2%

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentProgress += increment
				if currentProgress >= 0.9 {
					currentProgress = 0.9 // 保持在 0.9，等待请求完成
				}
				progress(currentProgress)
			}
		}
	}()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(apiURL)

	if err != nil {
		progress(1.0)
		return nil, err
	}
	defer resp.Body.Close()

	progress(1.0)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	if release.TagName > currentVersion {
		return &release, nil
	}

	return nil, nil
}
