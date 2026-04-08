package fonts

import (
	"os"
	"runtime"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/hankmor/vdd/core/logger"
)

var (
	// 缓存已加载的字体防止重复 IO
	loadedFont fyne.Resource
	fontOnce   sync.Once
)

// LoadNativeFont 尝试加载系统原生字体 (优先保证 CJK 支持)
// macOS: PingFang SC / Hiragino
// Windows: Microsoft YaHei (msyh.ttc)
func LoadNativeFont() fyne.Resource {
	fontOnce.Do(func() {
		loadedFont = loadFontInternal()
	})
	return loadedFont
}

func loadFontInternal() fyne.Resource {
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			// "/Users/hank/Library/Fonts/Inter-Regular.ttf",
			// "/Users/hank/Library/Fonts/NotoSansSC-Black.ttf",
			// "/Users/hank/Library/Fonts/JetBrainsMonoNerdFontMono-Regular.ttf",
			// "/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			// "/System/Library/Fonts/Arial Unicode.ttf",
		}
	case "windows":
		paths = []string{
			"C:\\Windows\\Fonts\\msyh.ttf",    // Microsoft YaHei (TTF)
			"C:\\Windows\\Fonts\\simhei.ttf",  // SimHei (TTF)
		}
	case "linux":
		paths = []string{
			// Droid Sans Fallback (Common safe TTF)
			"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
			"/usr/share/fonts/droid/DroidSansFallbackFull.ttf",
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			logger.Infof("Loading system font: %s", path)
			// fyne.LoadResourceFromPath handles file reading
			res, err := fyne.LoadResourceFromPath(path)
			if err == nil {
				return res
			}
			logger.Warnf("Failed to load font %s: %v", path, err)
		}
	}

	logger.Infof("Using default embedded font (Noto Sans)")
	return theme.DefaultTheme().Font(fyne.TextStyle{})
}
