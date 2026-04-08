package themes

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/hankmor/vdd/core/fonts"
)

// VDDTheme VDD 应用主题 (Default/Auto)
type VDDTheme struct{}

var _ fyne.Theme = (*VDDTheme)(nil)

// =================================================================================
// 🎨 配色方案定义 (Design System)
// =================================================================================

type Palette struct {
	Background      color.Color
	Surface         color.Color
	Primary         color.Color
	Text            color.Color
	SubText         color.Color
	Button          color.Color
	ButtonHighlight color.Color
	Disabled        color.Color
	Input           color.Color
}

// ---------------------------------------------------------------------------------
// 🌙 Dark Theme 1: Titanium (钛金深空) - **默认深色**
// 风格：专业、沉稳、类似 VS Code 或 Linear 的高级灰蓝色调
// ---------------------------------------------------------------------------------
var ThemeTitanium = Palette{
	Background:      color.RGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF}, // Slate 900
	Surface:         color.RGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF}, // Slate 800
	Primary:         color.RGBA{R: 0x38, G: 0xBD, B: 0xF8, A: 0xFF}, // Sky 400 (Vibrant Blue)
	Text:            color.RGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}, // Slate 100
	SubText:         color.RGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF}, // Slate 400
	Button:          color.RGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF}, // Slate 800
	ButtonHighlight: color.RGBA{R: 0x33, G: 0x41, B: 0x55, A: 0xFF}, // Slate 700
	Disabled:        color.RGBA{R: 0x33, G: 0x41, B: 0x55, A: 0xFF}, // Slate 700
	Input:           color.RGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF}, // Darker input
}

// ---------------------------------------------------------------------------------
// 🌙 Dark Theme 2: Cyberpunk (赛博霓虹) - **备选深色**
// 风格：高对比度、深黑背景、霓虹紫/青色点缀
// ---------------------------------------------------------------------------------
var ThemeCyberpunk = Palette{
	Background:      color.RGBA{R: 0x05, G: 0x05, B: 0x05, A: 0xFF}, // Pure Black-ish
	Surface:         color.RGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}, // Lighter Black (Zinc 925ish) for contrast
	Primary:         color.RGBA{R: 0x8B, G: 0x5C, B: 0xF6, A: 0xFF}, // Violet 500 (Neon Violet) - Distinct from Pink
	Text:            color.RGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF}, // High conrast white
	SubText:         color.RGBA{R: 0xA1, G: 0xA1, B: 0xAA, A: 0xFF}, // Zinc 400
	Button:          color.RGBA{R: 0x27, G: 0x27, B: 0x2A, A: 0xFF}, // Zinc 800 (Lighter than bg)
	ButtonHighlight: color.RGBA{R: 0x3F, G: 0x3F, B: 0x46, A: 0xFF}, // Zinc 700
	Disabled:        color.RGBA{R: 0x27, G: 0x27, B: 0x2A, A: 0xFF},
	Input:           color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF},
}

// ---------------------------------------------------------------------------------
// ☀️ Light Theme 1: Polar (极地净白) - **默认浅色**
// 风格：干净、通透、类似 Apple 设计风格，极简且高可读性
// ---------------------------------------------------------------------------------
var ThemePolar = Palette{
	Background:      color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // Pure White
	Surface:         color.RGBA{R: 0xF8, G: 0xFA, B: 0xFC, A: 0xFF}, // Slate 50 (Very light gray)
	Primary:         color.RGBA{R: 0x02, G: 0x84, B: 0xC7, A: 0xFF}, // Sky 600 (Solid Blue)
	Text:            color.RGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF}, // Slate 900 (Ink Black)
	SubText:         color.RGBA{R: 0x64, G: 0x74, B: 0x8B, A: 0xFF}, // Slate 500
	Button:          color.RGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}, // Slate 100
	ButtonHighlight: color.RGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}, // Slate 200
	Disabled:        color.RGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}, // Slate 200
	Input:           color.RGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}, // Light gray input
}

// ---------------------------------------------------------------------------------
// ☀️ Light Theme 2: Latte (暖调拿铁) - **备选浅色**
// 风格：温暖、舒适、米色调，适合长时间阅读
// ---------------------------------------------------------------------------------
var ThemeLatte = Palette{
	Background:      color.RGBA{R: 0xFD, G: 0xFB, B: 0xF7, A: 0xFF}, // Warm Cream
	Surface:         color.RGBA{R: 0xF6, G: 0xF3, B: 0xEA, A: 0xFF}, // Darker Cream
	Primary:         color.RGBA{R: 0xD9, G: 0x77, B: 0x06, A: 0xFF}, // Amber 600 (Warm Chrome)
	Text:            color.RGBA{R: 0x45, G: 0x1A, B: 0x03, A: 0xFF}, // Dark Brown
	SubText:         color.RGBA{R: 0x78, G: 0x35, B: 0x0F, A: 0xFF}, // Lighter Brown
	Button:          color.RGBA{R: 0xED, G: 0xE9, B: 0xFE, A: 0xFF}, // Wash
	ButtonHighlight: color.RGBA{R: 0xE7, G: 0xE5, B: 0xE4, A: 0xFF}, // Warm Gray
	Disabled:        color.RGBA{R: 0xD6, G: 0xD3, B: 0xD1, A: 0xFF},
	Input:           color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
}

// =================================================================================
// 主题实现
// =================================================================================

// 当前激活的配色方案 (可以通过配置修改这里来切换)
// 目前逻辑：Dark 使用 Titanium，Light 使用 Polar
// =================================================================================
// 主题实现
// =================================================================================

// 当前激活的配色方案 (可以通过配置修改这里来切换)
// 目前逻辑：Dark 使用 Titanium，Light 使用 Polar
var (
	currentDark  = ThemeTitanium
	currentLight = ThemePolar
	
	// 注册样式表
	darkStyles = map[string]Palette{
		"titanium":  ThemeTitanium,
		"cyberpunk": ThemeCyberpunk,
	}
	lightStyles = map[string]Palette{
		"polar": ThemePolar,
		"latte": ThemeLatte,
	}
	
	themeMu sync.RWMutex
)

// SetDarkStyle 设置深色模式样式
func SetDarkStyle(name string) {
	themeMu.Lock()
	defer themeMu.Unlock()
	if p, ok := darkStyles[name]; ok {
		currentDark = p
	}
}

// SetLightStyle 设置浅色模式样式
func SetLightStyle(name string) {
	themeMu.Lock()
	defer themeMu.Unlock()
	if p, ok := lightStyles[name]; ok {
		currentLight = p
	}
}

func (t *VDDTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	themeMu.RLock()
	defer themeMu.RUnlock()
	
	p := currentLight
	if variant == theme.VariantDark {
		p = currentDark
	}

	switch name {
	// 基础背景
	case theme.ColorNameBackground:
		return p.Background
	// 表面/面板背景 (Overlay, Menu, Header)
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground, theme.ColorNameInputBackground, theme.ColorNameHeaderBackground:
		return p.Surface // 输入框和头部使用 Surface 颜色，增加层次感
	
	// 强调色
	case theme.ColorNamePrimary, theme.ColorNameSelection, theme.ColorNameFocus:
		return p.Primary
	
	// 文本
	case theme.ColorNameForeground: 
		// 注意: HeaderBackground 在 Fyne 中有时用于文字背景，但 Foreground 是主要文字
		return p.Text
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return p.SubText
		
	// 按钮
	case theme.ColorNameButton:
		return p.Button
	case theme.ColorNameHover:
		return p.ButtonHighlight // Hover 状态
		
	case theme.ColorNameDisabledButton:
		return p.Disabled

	case theme.ColorNameScrollBar:
		// 滚动条使用 Primary 的半透明版本，或者 SubText
		r, g, b, _ := p.Primary.RGBA()
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0x66} 

	case theme.ColorNameShadow:
		return color.RGBA{R: 0, G: 0, B: 0, A: 0x33}
	}

	// 特殊处理：InputBackground 即使在 light 主题下也想要一点区分
	if name == theme.ColorNameInputBackground {
		return p.Input
	}

	return theme.DefaultTheme().Color(name, variant)
}

func (t *VDDTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *VDDTheme) Font(style fyne.TextStyle) fyne.Resource {
	// 简单起见，所有样式都使用同一个 CJK 字体 (通常包含了粗体等变体，或者我们只加载常规体)
	// 如果需要精确的 Bold/Italic 支持，需要 fonts.LoadNativeFont 接受 style 参数并加载不同文件
	// 目前先统一加载主字体以保证中文显示。
	return fonts.LoadNativeFont()
}

func (t *VDDTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 14 // Default is usually 14
	case theme.SizeNameCaptionText:
		return 13 // Default is usually 11
	case theme.SizeNameHeadingText:
		return 18 // Default is ~24
	case theme.SizeNameSubHeadingText:
		return 16 // Default is ~18
	case theme.SizeNameInlineIcon:
		return 20 // Match slightly larger text
	case theme.SizeNamePadding, theme.SizeNameInnerPadding:
		return 4 // Default is 4? Keep it slightly spacious but standard
	}
	return theme.DefaultTheme().Size(name)
}

// ForcedVariantTheme 强制指定亮色或暗色主题
type ForcedVariantTheme struct {
	*VDDTheme
	variant fyne.ThemeVariant
}

func (t *ForcedVariantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	// 忽略传入的 variant，使用强制的 variant
	return t.VDDTheme.Color(name, t.variant)
}

// NewLightTheme 返回强制亮色主题
func NewLightTheme() fyne.Theme {
	return &ForcedVariantTheme{
		VDDTheme: &VDDTheme{},
		variant:  theme.VariantLight,
	}
}

// NewDarkTheme 返回强制暗色主题
func NewDarkTheme() fyne.Theme {
	return &ForcedVariantTheme{
		VDDTheme: &VDDTheme{},
		variant:  theme.VariantDark,
	}
}
