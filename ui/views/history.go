package views

import (
	"fmt"
	"math"
	"net/url"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/hankmor/vdd/core/history"
	"github.com/hankmor/vdd/core/logger"
	"github.com/hankmor/vdd/ui/helper"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/ui/widgets"
	"github.com/hankmor/vdd/utils"
)

// HistoryView 历史记录视图
type HistoryView struct {
	Container *fyne.Container
	list      *fyne.Container

	// Pagination Controls
	prevBtn   *widget.Button
	nextBtn   *widget.Button
	pageLabel *widget.Label

	clearAllBtn     *widgets.ButtonWithTooltip
	clearMissingBtn *widgets.ButtonWithTooltip

	app    fyne.App
	window fyne.Window

	currentPage int
	pageSize    int
	totalCount  int64
}

func NewHistoryView(app fyne.App, window fyne.Window) *HistoryView {
	v := &HistoryView{
		app:         app,
		window:      window,
		list:        container.NewVBox(),
		currentPage: 1,
		pageSize:    10, // 每页 10 条
	}

	// ===== 列表区域 =====
	scroll := container.NewVScroll(v.list)

	// ===== 分页栏 =====
	// 上一页
	v.prevBtn = widget.NewButtonWithIcon("", icons.ThemedPrevIcon, func() {
		if v.currentPage > 1 {
			v.currentPage--
			v.Refresh()
		}
	})
	// 下一页
	v.nextBtn = widget.NewButtonWithIcon("", icons.ThemedNextIcon, func() {
		totalPages := int(math.Ceil(float64(v.totalCount) / float64(v.pageSize)))
		if v.currentPage < totalPages {
			v.currentPage++
			v.Refresh()
		}
	})
	v.pageLabel = widget.NewLabel("第 1 页")

	paginationBar := container.NewHBox(
		layout.NewSpacer(),
		v.prevBtn,
		v.pageLabel,
		v.nextBtn,
		layout.NewSpacer(),
	)

	// ===== 操作按钮栏（右下角）
	// 清除所有历史记录 - 使用 ViewRefreshIcon（刷新图标）表示清空重置
	v.clearAllBtn = widgets.NewButtonWithTooltip("", icons.ThemedClearAllIcon, func() {
		if v.totalCount > 0 {
			dialog.ShowConfirm("清除所有历史记录", "确定要清除所有历史记录吗？\n此操作将删除所有历史记录。", func(ok bool) {
				if ok {
					v.clearAllHistory()
				}
			}, v.window)
		}
	}, "清除所有历史记录")
	v.clearAllBtn.Disable()

	// 清除文件不存在的记录 - 使用 CancelIcon（取消图标）表示清理无效项
	v.clearMissingBtn = widgets.NewButtonWithTooltip("", icons.ThemedClearMissingIcon, func() {
		if v.totalCount > 0 {
			dialog.ShowConfirm("清除文件不存在的记录", "确定要清除所有文件不存在的历史记录吗？\n此操作将删除文件已被删除的历史记录。", func(ok bool) {
				if ok {
					v.clearMissingFiles()
				}
			}, v.window)
		}
	}, "清除文件不存在的记录")
	v.clearMissingBtn.Disable()

	actionBar := container.NewHBox(
		layout.NewSpacer(),
		v.clearMissingBtn,
		v.clearAllBtn,
	)

	// 底部栏：操作按钮 + 分页
	bottomBar := container.NewHBox(
		layout.NewSpacer(),
		paginationBar,
		layout.NewSpacer(),
		actionBar,
	)

	// 主布局: 列表 + 底部栏
	v.Container = container.NewBorder(nil, bottomBar, nil, nil, scroll)

	v.Refresh()

	// 自动刷新 (2秒一次即可)
	// go func() {
	// 	for {
	// 		if v.Container.Visible() {
	// 			// Debug refresh
	// 			// fmt.Println("[History] Auto Refresh Triggered")
	// 			// fyne.Do(v.Refresh)
	// 		}
	// 		time.Sleep(2 * time.Second)
	// 	}
	// }()

	return v
}

// Refresh 刷新列表
func (v *HistoryView) Refresh() {
	logger.Info("[历史] 刷新...")
	offset := (v.currentPage - 1) * v.pageSize
	if offset < 0 {
		offset = 0
	}

	records, total, err := history.GetHistory(offset, v.pageSize)

	if err != nil {
		logger.Errorf("[历史] 加载失败: %v", err)
		return
	}

	v.totalCount = total
	totalPages := int(math.Ceil(float64(total) / float64(v.pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	// 更新分页UI
	v.pageLabel.SetText(fmt.Sprintf("第 %d / %d 页 (共 %d 条)", v.currentPage, totalPages, total))

	if v.currentPage <= 1 {
		v.prevBtn.Disable()
	} else {
		v.prevBtn.Enable()
	}

	if v.currentPage >= totalPages {
		v.nextBtn.Disable()
	} else {
		v.nextBtn.Enable()
	}

	var items []fyne.CanvasObject
	for _, rec := range records {
		items = append(items, v.createHistoryItem(rec), widget.NewSeparator())
	}

	if len(items) == 0 {
		label := widget.NewLabelWithStyle("暂无历史记录", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
		label.Importance = widget.MediumImportance

		items = []fyne.CanvasObject{
			container.NewCenter(label),
		}
	}

	v.list.Objects = items
	v.list.Refresh()
}

// createHistoryItem 创建历史记录项
func (v *HistoryView) createHistoryItem(rec history.HistoryRecord) fyne.CanvasObject {
	r := rec // 捕获循环变量

	// ===== 视频缩略图 =====
	// 默认图标
	iconImg := canvas.NewImageFromResource(theme.FileVideoIcon())
	iconImg.FillMode = canvas.ImageFillContain
	iconImg.SetMinSize(fyne.NewSize(100, 45))

	// ===== 视频信息 =====
	infoTextStr := fmt.Sprintf("大小: %s  •  完成时间: %s",
		utils.FormatBytes(r.FileSize),
		r.CompletedAt.Format("2006-01-02 15:04"),
	)

	// 初始显示：假设文件存在 (Bold)，稍后异步检查更新
	titleLabel := widget.NewLabelWithStyle(r.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Truncation = fyne.TextTruncateEllipsis
	infoLabel := widget.NewLabel(infoTextStr)
	textInfo := container.New(widgets.NewCompactVBoxLayout(0), titleLabel, infoLabel)

	// ===== 操作 =====
	folderBtn := widgets.NewButtonWithTooltip("", icons.ThemedOpenFolderIcon, func() {
		if r.FilePath == "" {
			return
		}
		dir := filepath.Dir(r.FilePath)
		u := &url.URL{Scheme: "file", Path: dir}
		widgets.SafeOpenURL(v.app, u)
	}, "打开位置")
	deleteBtn := widgets.NewButtonWithTooltip("", icons.ThemedDeleteCircleIcon, func() {
		dialog.ShowConfirm("删除记录", "确定要删除这条历史记录吗？\n(文件不会被删除)", func(ok bool) {
			if ok {
				if err := history.Delete(r.ID); err != nil {
					logger.Errorf("[历史] 删除记录失败: %v", err)
				} else {
					if r.Thumbnail != "" {
						utils.DeleteThumbnailCache(r.Thumbnail) // Best effort
					}
					v.Refresh()
				}
			}
		}, v.window)
	}, "删除记录")

	actions := container.NewHBox(folderBtn, deleteBtn)

	// ===== 布局 =====	
	card := container.NewBorder(
		nil, nil,
		container.NewPadded(iconImg),
		actions,
		textInfo,
	)

	wrapper := container.NewPadded(card)

	// ===== 异步加载资源和状态检查 =====
	go func() {
		// A. 检查文件是否存在
		fileExists := history.ValidateFilePath(r.FilePath)

		// B. 加载缩略图
		if r.Thumbnail != "" {
			helper.SharedThumbnailManager().LoadThumbnail(r.Thumbnail, func(res fyne.Resource) {
				fyne.Do(func() {
					iconImg.Resource = res
					iconImg.Refresh()
				})
			})
		}

		// C. 更新 UI (文件状态)
		fyne.Do(func() {
			// Update File Status style
			if !fileExists {
				titleLabel.TextStyle = fyne.TextStyle{Bold: true}
				titleLabel.Importance = widget.MediumImportance
				infoLabel.Importance = widget.MediumImportance

				// 强制刷新 Label 样式
				titleLabel.Refresh()
				infoLabel.Refresh()
			}
		})
	}()

	return wrapper
}

// clearAllHistory 清除所有历史记录
func (v *HistoryView) clearAllHistory() {
	// 获取所有记录以删除缩略图缓存
	allRecords, err := history.GetAllRecords()
	if err != nil {
		logger.Errorf("[历史] 获取所有记录失败: %v", err.Error())
		dialog.ShowError(fmt.Errorf("获取历史记录失败: %v", err), v.window)
		return
	}

	// 删除所有缩略图缓存
	for _, rec := range allRecords {
		if rec.Thumbnail != "" {
			if err := utils.DeleteThumbnailCache(rec.Thumbnail); err != nil {
				logger.Errorf("[历史] 删除缩略图缓存失败: %v", err.Error())
			}
		}
	}

	// 清空数据库
	if err := history.Clear(); err != nil {
		logger.Errorf("[历史] 清除历史记录失败: %v", err.Error())
		dialog.ShowError(fmt.Errorf("清除历史记录失败: %v", err), v.window)
		return
	}

	// 重置到第一页并刷新
	v.currentPage = 1
	v.Refresh()

	dialog.ShowInformation("清除完成", "已清除所有历史记录", v.window)
}

// clearMissingFiles 清除文件不存在的历史记录
func (v *HistoryView) clearMissingFiles() {
	// 获取所有记录
	allRecords, err := history.GetAllRecords()
	if err != nil {
		logger.Errorf("[历史] 获取所有记录失败: %v", err.Error())
		dialog.ShowError(fmt.Errorf("获取历史记录失败: %v", err), v.window)
		return
	}

	deletedCount := 0
	// 删除文件不存在的记录及其缩略图缓存
	for _, rec := range allRecords {
		if !history.ValidateFilePath(rec.FilePath) {
			// 删除缩略图缓存
			if rec.Thumbnail != "" {
				if err := utils.DeleteThumbnailCache(rec.Thumbnail); err != nil {
					logger.Errorf("[历史] 删除缩略图缓存失败: %v", err.Error())
				}
			}

			// 删除历史记录
			if err := history.Delete(rec.ID); err != nil {
				logger.Errorf("[历史] 删除记录失败: %s: %v", rec.ID, err.Error())
				continue
			}
			deletedCount++
		}
	}

	// 重置到第一页并刷新
	v.currentPage = 1
	v.Refresh()

	if deletedCount > 0 {
		dialog.ShowInformation("清除完成", fmt.Sprintf("已清除 %d 条文件不存在的历史记录", deletedCount), v.window)
	}
}
