//go:build !linux

package helper

import (
	"fyne.io/fyne/v2"
	ndialog "github.com/sqweek/dialog"
)

// ShowFileOpen 显示文件打开对话框 (Native)
func ShowFileOpen(window fyne.Window, title string, filterExt []string, callback func(string, error)) {
	// 异步执行以避免阻塞 UI 线程 (尽管 native dialog 通常会阻塞直到返回)
	go func() {
		builder := ndialog.File().Title(title)
		if len(filterExt) > 0 {
			// sqweek/dialog filter format: "Description", "ext1", "ext2"...
			// 这里简单处理，假设 filterExt 只有后缀
			builder = builder.Filter("Supported Files", filterExt...)
		}
		filename, err := builder.Load()
		if err != nil && err != ndialog.ErrCancelled {
			// callback with error
			// window.Canvas().Refresh() // Removed invalid call
			callback("", err)
			return
		}
		if err == ndialog.ErrCancelled {
			return
		}

		// Success
		callback(filename, nil)
	}()
}

// ShowFolderOpen 显示文件夹选择对话框 (Native)
func ShowFolderOpen(window fyne.Window, title string, callback func(string, error)) {
	go func() {
		dir, err := ndialog.Directory().Title(title).Browse()
		if err != nil && err != ndialog.ErrCancelled {
			callback("", err)
			return
		}
		if err == ndialog.ErrCancelled {
			return
		}
		callback(dir, nil)
	}()
}
