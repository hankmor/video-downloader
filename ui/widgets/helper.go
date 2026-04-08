package widgets

import (
	"fmt"
	"net/url"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"github.com/hankmor/vdd/core/osx"
)

// SafeOpenURL 打开 URL，针对 Windows 优化以避免控制台闪烁
func SafeOpenURL(app fyne.App, u *url.URL) {
	if runtime.GOOS == "windows" {
		// 尝试使用 utils.OpenURL (rundll32)
		// 注意: u.String() 可能包含特殊字符，但 rundll32 通常处理得很好
		err := osx.OpenURL(u.String())
		if err == nil {
			return
		}
		// 如果失败，记录日志并回退
		fmt.Printf("Custom OpenURL failed: %v, falling back to app.OpenURL\n", err)
	}

	app.OpenURL(u)
}

// ShowSimpleConfirm 显示简单的确认对话框 (Helper)
func ShowSimpleConfirm(title, message string, callback func(bool), parent fyne.Window) {
	dialog.ShowConfirm(title, message, callback, parent)
}

// WeekdayToChinese 将 time.Weekday 转换为中文星期
func WeekdayToChinese(d time.Weekday) string {
	switch d {
	case time.Sunday:
		return "星期日"
	case time.Monday:
		return "星期一"
	case time.Tuesday:
		return "星期二"
	case time.Wednesday:
		return "星期三"
	case time.Thursday:
		return "星期四"
	case time.Friday:
		return "星期五"
	case time.Saturday:
		return "星期六"
	default:
		return ""
	}
}
