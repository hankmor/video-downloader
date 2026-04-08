package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"github.com/hankmor/vdd/assets"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/consts"
	"github.com/hankmor/vdd/core/db"
	"github.com/hankmor/vdd/core/env"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/core/parser"
	"github.com/hankmor/vdd/core/subscription"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/ui/themes"
	"github.com/hankmor/vdd/ui/windows"

	// Register image decoders
	_ "image/jpeg"
	_ "image/png"

	// Register WebP decoder (critical for YouTube thumbnails)
	_ "golang.org/x/image/webp"
)

func main() {
	// 定义命令行参数
	logPath := flag.String("log", "", "日志文件路径(可选，不指定则使用默认路径)")
	flag.Parse()
	if env.IsDev {
		logger.SetLevel(logger.LevelDebug)
	}

	// 初始化日志系统
	if err := logger.Init(*logPath); err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// 捕获 Fyne 在 macOS 上关闭时可能出现的 panic
	// 这是一个已知的 GLFW 问题，不影响实际使用
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("程序出错: %v\n", r)

			if env.IsDev {
				debug.PrintStack()
			}
		}
	}()

	logger.Infof(`
 _ _  __   __  
| | ||  \ |  \ 
| V || o )| o )
 \_/ |__/ |__/ 
Version: %s
`, consts.AppVersion)
	logger.Infof("[主窗] 正在启动, 请稍后...")

	// 创建应用
	myApp := app.NewWithID("com.hankmo.vdd")

	// 初始化数据库
	// 注入需要自动迁移的模型
	if err := db.Init(
		&config.ConfigModel{},
		&tasks.Task{},
		&parser.ParseResult{},
		&subscription.Subscription{},
		&subscription.SubscriptionVideo{},
		&subscription.SubscriptionBadgeState{}, // 新增: 订阅角标状态表
	); err != nil {
		fmt.Printf("数据库初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 加载配置
	if err := config.Load(); err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 应用主题配置
	cfg := config.Get()
	
	// 先应用具体的样式偏好
	themes.SetLightStyle(cfg.LightStyle)
	themes.SetDarkStyle(cfg.DarkStyle)

	switch cfg.Theme {
	case config.ThemeLight:
		myApp.Settings().SetTheme(themes.NewLightTheme())
	case config.ThemeDark:
		myApp.Settings().SetTheme(themes.NewDarkTheme())
	default:
		// auto - 使用系统默认主题
		myApp.Settings().SetTheme(&themes.VDDTheme{})
	}

	// 设置应用图标
	icon := assets.LogoSVG
	myApp.SetIcon(icon)

	// 创建主窗口
	mainWindow := windows.NewMainWindow(myApp)

	// 系统托盘支持
	setupSystemTrayIcon(myApp, mainWindow)

	// 接管关闭事件：点击关闭时隐藏窗口而非退出 (如果支持托盘)
	// 注意：在 macOS 上，点击关闭通常是隐藏窗口，只有 Cmd+Q 才是退出
	mainWindow.SetCloseIntercept(func() {
		mainWindow.Hide()
	})
	
	logger.Infof("[主窗] 启动完成, 开启您的极速下载之旅吧！")

	// 显示并运行
	mainWindow.Run()
	mainWindow.CleanUp()
}

func setupSystemTrayIcon(myApp fyne.App, mainWindow *windows.MainWindow) {
	if desk, ok := myApp.(desktop.App); ok {
		// 设置托盘菜单
		var items []*fyne.MenuItem
		items = append(items, fyne.NewMenuItem("显示主界面", func() {
			mainWindow.Show()
		}))

		// Windows/Linux 不会自动添加退出菜单，需手动添加
		// macOS Fyne 驱动会自动添加 "Quit"，因此手动添加会导致重复
		if runtime.GOOS != "darwin" {
			items = append(items, fyne.NewMenuItemSeparator())
			items = append(items, fyne.NewMenuItem("退出", func() {
				myApp.Quit()
			}))
		}

		m := fyne.NewMenu("VDD", items...)
		desk.SetSystemTrayMenu(m)

		// 根据操作系统选择托盘图标
		var trayIcon fyne.Resource
		switch runtime.GOOS {
		case "darwin":
			// macOS: 使用SVG模板图标，系统会自动处理颜色
			// NewThemedResource 会让图标跟随系统亮色/暗色模式
			trayIcon = theme.NewThemedResource(assets.LogoSVG)
		case "windows":
			// Windows: 可以使用彩色图标，用户更习惯彩色托盘图标
			// 这里使用启动图标（彩色版）
			trayIcon = assets.StartupIcon
		default:
			// Linux: 使用SVG主题图标
			trayIcon = theme.NewThemedResource(assets.LogoSVG)
		}

		desk.SetSystemTrayIcon(trayIcon)
	}
}
