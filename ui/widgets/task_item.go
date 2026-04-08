package widgets

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/hankmor/vdd/assets"
	"github.com/hankmor/vdd/core/parser"
	"github.com/hankmor/vdd/core/tasks"
	"github.com/hankmor/vdd/ui/helper"
	"github.com/hankmor/vdd/ui/icons"
	"github.com/hankmor/vdd/utils"
)

// TaskItemWidget 封装单个任务列表项的 UI 组件
type TaskItemWidget struct {
	Container   *fyne.Container
	StatusLabel *widget.Label
	ProgressBar *ThinProgressBar
	Actions     *fyne.Container

	TaskID string
}

// TaskActions 定义任务操作的回调接口
type TaskActions struct {
	OnStart      func()
	OnCancel     func()
	OnOpenFolder func()
	OnDelete     func()
}

// NewTaskItemWidget 创建新的任务项组件
func NewTaskItemWidget(task *tasks.Task, actions TaskActions) *TaskItemWidget {
	// 1. 图标/缩略图 (异步加载)
	iconImg := canvas.NewImageFromResource(assets.DefaultThumbnail)
	iconImg.FillMode = canvas.ImageFillContain
	iconImg.SetMinSize(fyne.NewSize(100, 45))

	// 2. 标题与状态
	var titleObj fyne.CanvasObject
	titleObj = whenFileNotFound(task)

	// 分辨率徽章和来源图标
	var leftObjects []fyne.CanvasObject

	// 来源图标
	source := parser.SourceFromURL(task.URL)
	if icon := icons.SourceIcons.Get(source); icon != nil {
		sourceIcon := NewIconWithTooltip(icon, source)
		leftObjects = append(leftObjects, container.NewCenter(sourceIcon))
	}

	// 分辨率徽章
	var rightBadgeContainer *fyne.Container
	label, col := GetResolutionColor(task.ResolutionDimension())
	if label != "" && col != nil {
		badge := NewBadge(label, col)
		rightBadgeContainer = container.NewCenter(badge.Container)
	} else {
		rightBadgeContainer = container.NewHBox()
	}

	// 左侧容器
	var leftContainer *fyne.Container
	if len(leftObjects) > 0 {
		leftContainer = container.NewHBox(leftObjects...)
	} else {
		leftContainer = container.NewHBox()
	}

	titleContainer := container.NewBorder(nil, nil, leftContainer, rightBadgeContainer, titleObj)

	statusText := fmt.Sprintf("%s • %s", task.Status.Name(), utils.FormatBytes(task.TotalSize))
	statusLabel := widget.NewLabelWithStyle(statusText, fyne.TextAlignLeading, fyne.TextStyle{TabWidth: 2})
	statusLabel.SizeName = theme.SizeNameCaptionText

	// 3. 进度条
	progressBar := NewThinProgressBar()
	progressBar.SetValue(task.Progress / 100.0)

	// 4. 操作容器
	actionsHBox := container.NewHBox()
	actionsContainer := container.NewCenter(actionsHBox)

	// 5. 状态栏和操作按钮的组合
	statusWithActions := container.NewBorder(nil, nil, statusLabel, actionsContainer, nil)

	// 布局
	textInfo := container.New(NewCompactVBoxLayout(0), titleContainer, statusWithActions, progressBar)

	card := container.NewBorder(
		nil, nil,
		container.NewPadded(iconImg),
		nil,
		textInfo,
	)

	// 边距
	leftSpacer := canvas.NewRectangle(color.Transparent)
	leftSpacer.SetMinSize(fyne.NewSize(12, 0))
	rightSpacer := canvas.NewRectangle(color.Transparent)
	rightSpacer.SetMinSize(fyne.NewSize(12, 0))

	finalContainer := container.NewBorder(nil, nil, leftSpacer, rightSpacer, card)

	w := &TaskItemWidget{
		Container:   finalContainer,
		StatusLabel: statusLabel,
		ProgressBar: progressBar,
		Actions:     actionsHBox,
		TaskID:      task.ID,
	}

	// 初始化按钮
	w.RefreshActions(task, actions)

	// 异步加载缩略图
	go func() {
		if task.Thumbnail != "" {
			helper.SharedThumbnailManager().LoadThumbnail(task.Thumbnail, func(res fyne.Resource) {
				fyne.Do(func() {
					iconImg.Resource = res
					iconImg.Refresh()
				})
			})
		}
	}()

	return w
}

// RefreshActions 刷新操作按钮
func (w *TaskItemWidget) RefreshActions(task *tasks.Task, actions TaskActions) {
	// 每次都重新创建按钮以确保状态正确，性能影响忽略不计
	// 注意：为了保持 UI 稳定，这里使用 GridWrap 固定大小
	buttonSize := fyne.NewSize(32, 32)

	startBtn := NewButtonWithTooltip("", icons.ThemedPlayIcon, actions.OnStart, "下载")
	cancelBtn := NewButtonWithTooltip("", icons.ThemedDeleteCircleIcon, actions.OnCancel, "取消")
	folderBtn := NewButtonWithTooltip("", icons.ThemedOpenFolderIcon, actions.OnOpenFolder, "打开")
	deleteBtn := NewButtonWithTooltip("", icons.ThemedClearAllHistoryIcon, actions.OnDelete, "删除")

	// 更新状态
	switch task.Status {
	case tasks.StatusDownloading, tasks.StatusQueued:
		startBtn.Disable()
		cancelBtn.Enable()
		folderBtn.Enable() // 允许在下载时查看（如果有部分文件）
		deleteBtn.Enable()
	case tasks.StatusFailed, tasks.StatusCanceled:
		startBtn.Enable()
		cancelBtn.Disable()
		folderBtn.Enable()
		deleteBtn.Enable()
	case tasks.StatusCompleted:
		startBtn.Disable()
		cancelBtn.Disable()
		folderBtn.Enable()
		deleteBtn.Enable()
	}

	objs := []fyne.CanvasObject{
		container.NewGridWrap(buttonSize, startBtn),
		container.NewGridWrap(buttonSize, cancelBtn),
		container.NewGridWrap(buttonSize, folderBtn),
		container.NewGridWrap(buttonSize, deleteBtn),
	}

	w.Actions.Objects = objs
	fyne.Do(func() {
		w.Actions.Refresh()
	})
}

// Internal Helper
func whenFileNotFound(task *tasks.Task) fyne.CanvasObject {
	isDeleted := false
	if task.Status == tasks.StatusCompleted {
		if task.ActualPath != "" && !utils.FileExists(task.ActualPath) {
			isDeleted = true
		}
	}

	if isDeleted {
		rt := widget.NewRichText(&widget.TextSegment{
			Text: task.Title + "(文件不存在)",
			Style: widget.RichTextStyle{
				Alignment: fyne.TextAlignLeading,
				TextStyle: fyne.TextStyle{Bold: true},
				ColorName: theme.ColorNameDisabled,
				SizeName:  theme.SizeNameText,
			},
		})
		rt.Truncation = fyne.TextTruncateEllipsis
		rt.Scroll = container.ScrollNone
		return rt
	} else {
		// titleLabel := widget.NewLabelWithStyle(task.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		// titleLabel.Truncation = fyne.TextTruncateEllipsis
		rt := widget.NewRichText(&widget.TextSegment{
			Text: task.Title,
			Style: widget.RichTextStyle{
				Alignment: fyne.TextAlignLeading,
				TextStyle: fyne.TextStyle{Bold: true},
				ColorName: theme.ColorNameForeground,
				SizeName:  theme.SizeNameText,
			},
		})
		rt.Truncation = fyne.TextTruncateEllipsis
		rt.Scroll = container.ScrollNone
		return rt
	}
}

// Helper to get resolution color (moved from tasks.go logic if it was there, or just keep it duplicated for now if it's simple)
// Actually GetResolutionColor is in widgets package? I need to check where it is defined.
// Assuming it is NOT in widgets package yet based on previous logs (it was called via widgets.GetResolutionColor in tasks.go).
// Wait, ui/views/tasks.go called widgets.GetResolutionColor. So it IS in widgets package.
// I can just call it.
