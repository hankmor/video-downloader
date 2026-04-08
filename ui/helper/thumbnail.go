package helper

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/storage"
	"github.com/hankmor/vdd/assets"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/utils"
)

// ThumbnailManager 统一缩略图管理器
// 负责内存缓存、磁盘缓存和网络下载的协调
type ThumbnailManager struct {
	cache       sync.Map // map[string]fyne.Resource
	downloading sync.Map // map[string]bool
}

var instance *ThumbnailManager
var once sync.Once

// SharedThumbnailManager 获取单例实例
func SharedThumbnailManager() *ThumbnailManager {
	once.Do(func() {
		instance = &ThumbnailManager{}
	})
	return instance
}

// LoadThumbnail 异步加载缩略图
// url: 图片 URL
// onSuccess: 加载成功后的回调，会在 UI 线程执行（如果内部使用了 fyne.Do，或者由调用方保证？
// 为方便使用，我们在内部不做 fyne.Do，调用方通常在 fyne.Do 中更新 UI，
// 但因为这是个数据获取过程，回调最好是在数据准备好后调用。
// 为了通用性，我们约定 onSuccess 在任意 Goroutine 调用，由 UI 组件自己 wrap fyne.Do。
// 或者，为了方便 UI 组件，我们可以约定 onSuccess 可能在后台线程。
func (m *ThumbnailManager) LoadThumbnail(url string, onSuccess func(fyne.Resource)) {
	if url == "" {
		return
	}

	// 1. Check Memory Cache
	if res, ok := m.cache.Load(url); ok {
		onSuccess(res.(fyne.Resource))
		return
	}

	go func() {
		// 2. Check Disk Cache
		cachePath := utils.GetThumbnailCachePath(url)
		if utils.FileExists(cachePath) {
			fileURI := storage.NewFileURI(cachePath)
			if readRes, err := storage.LoadResourceFromURI(fileURI); err == nil {
				m.cache.Store(url, readRes)
				onSuccess(readRes)
				return
			}
		}

		// 3. Network Download
		// Prevent duplicate downloads
		if _, loading := m.downloading.LoadOrStore(url, true); loading {
			return // Already downloading, ignore this request or wait?
			// Ideally we should wait or register a callback, but for list scrolling, ignoring is fine (it will retry on refresh/scroll)
			// Or simple ignore.
		}
		defer m.downloading.Delete(url)

		localPath, _, err := utils.LoadOrDownloadThumbnail(url, config.Get().ProxyURL)
		if err == nil {
			fileURI := storage.NewFileURI(localPath)
			if readRes, err := storage.LoadResourceFromURI(fileURI); err == nil {
				m.cache.Store(url, readRes)
				onSuccess(readRes)
			}
		}
	}()
}

// GetDefaultIcon 获取默认视频图标 (Helper)
func GetDefaultIcon() fyne.CanvasObject {
	iconImg := canvas.NewImageFromResource(assets.DefaultThumbnail)
	iconImg.FillMode = canvas.ImageFillContain
	iconImg.SetMinSize(fyne.NewSize(80, 45))
	return iconImg
}
