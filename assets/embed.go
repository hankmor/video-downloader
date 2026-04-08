package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconData []byte

var StartupIcon = fyne.NewStaticResource("icon.png", iconData)

//go:embed logo.png
var logoData []byte
var LogoPNG = fyne.NewStaticResource("logo.png", logoData)

//go:embed logo-w.png
var logoWhiteData []byte
var LogoWhite = fyne.NewStaticResource("logo-w.png", logoWhiteData)

//go:embed logo.svg
var logoSVGData []byte
var LogoSVG = fyne.NewStaticResource("logo.svg", logoSVGData)

//go:embed default_thumbnail.png
var defaultThumbnailData []byte

// DefaultThumbnail 返回默认的视频封面图（用于没有缩略图的视频）
var DefaultThumbnail = fyne.NewStaticResource("default_thumbnail.png", defaultThumbnailData)
