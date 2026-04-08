//go:build linux

package helper

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

// ShowFileOpen 显示文件打开对话框
func ShowFileOpen(window fyne.Window, title string, filterExt []string, callback func(string, error)) {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			callback("", err)
			return
		}
		if reader == nil {
			// Cancelled
			return
		}
		callback(reader.URI().Path(), nil)
	}, window)

	if len(filterExt) > 0 {
		d.SetFilter(storage.NewExtensionFileFilter(filterExt))
	}
	// Note: Fyne dialog doesn't support setting Title easily for FileOpen in v2 public API easily without custom implementation,
	// but default is fine.
	d.Show()
}

// ShowFolderOpen 显示文件夹选择对话框
func ShowFolderOpen(window fyne.Window, title string, callback func(string, error)) {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			callback("", err)
			return
		}
		if uri == nil {
			return
		}
		callback(uri.Path(), nil)
	}, window)
}
