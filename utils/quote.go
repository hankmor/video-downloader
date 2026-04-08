package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HitokotoResponse Hitokoto API 响应结构
type HitokotoResponse struct {
	Hitokoto string `json:"hitokoto"`
	From     string `json:"from"`
	FromWho  string `json:"from_who"`
}

// FetchDailyQuote 从 Hitokoto 获取每日一言
// 参数 timeout: 超时时间
func FetchDailyQuote(timeout time.Duration) (string, error) {
	client := http.Client{
		Timeout: timeout,
	}

	// 使用 v1.hitokoto.cn API
	// 参数 c=i (诗词), c=k (哲学), c=d (文学)
	// encode=json (默认)
	resp, err := client.Get("https://v1.hitokoto.cn/?c=i&c=k&c=d")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data HitokotoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	if data.Hitokoto == "" {
		return "", fmt.Errorf("empty quote")
	}

	// 格式化输出: "内容" —— 来源
	// quote := fmt.Sprintf("“%s” —— %s", data.Hitokoto, data.From)
	// 用户界面上可能只需要内容，因为空间有限
	return data.Hitokoto, nil
}
