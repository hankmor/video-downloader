package widgets

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

// NotificationBadge 通知角标组件（红色圆圈+白色数字）
type NotificationBadge struct {
	widget.BaseWidget
	count     binding.Int
	visible   bool
	OnUpdated func() // 回调：当数字或可见性变化时触发
}

// NewNotificationBadge 创建新的通知角标
func NewNotificationBadge() *NotificationBadge {
	b := &NotificationBadge{
		count:   binding.NewInt(),
		visible: false,
	}
	b.ExtendBaseWidget(b)
	return b
}

// SetCount 设置角标数字
func (b *NotificationBadge) SetCount(count int) {
	b.count.Set(count)
	b.visible = count > 0 || count == -1 // -1 也是可见状态
	b.Refresh()
	if b.OnUpdated != nil {
		b.OnUpdated()
	}
}

// GetCount 获取当前数字
func (b *NotificationBadge) GetCount() int {
	val, _ := b.count.Get()
	return val
}

// IsVisible 是否可见
func (b *NotificationBadge) IsVisible() bool {
	return b.visible
}

// CreateRenderer 实现 Widget 接口
func (b *NotificationBadge) CreateRenderer() fyne.WidgetRenderer {
	// 背景圆圈
	circle := canvas.NewCircle(color.RGBA{R: 255, G: 59, B: 48, A: 255}) // iOS 红色

	// 文字
	text := canvas.NewText("", color.White)
	text.TextSize = 8
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.Alignment = fyne.TextAlignCenter

	// 容器
	c := container.NewStack(circle, container.NewCenter(text))

	r := &notificationBadgeRenderer{
		badge:  b,
		circle: circle,
		text:   text,
		cont:   c,
	}

	// 监听数据变化
	b.count.AddListener(binding.NewDataListener(r.refresh))

	r.refresh()
	return r
}

// NotificationBadgeRenderer 自定义渲染器
type notificationBadgeRenderer struct {
	badge  *NotificationBadge
	circle *canvas.Circle
	text   *canvas.Text
	cont   *fyne.Container
}

func (r *notificationBadgeRenderer) refresh() {
	count, _ := r.badge.count.Get()

	if count == 0 { // 0 隐藏
		r.text.Text = ""
		r.cont.Hide()
	} else if count == -1 { // -1 显示锁图标（保留扩展语义）
		r.text.Text = "🔒"
		r.text.TextSize = 14   // 增大 Emoji 尺寸
		r.circle.Hidden = true // 隐藏红色背景圆圈，只显示 Emoji
		r.circle.Refresh()
		r.cont.Show()
	} else if count <= 99 {
		r.text.Text = fmt.Sprintf("%d", count)
		r.text.TextSize = 8 // 恢复默认尺寸
		r.circle.Hidden = false
		r.circle.Refresh()
		r.cont.Show()
	} else {
		r.text.Text = "99+"
		r.text.TextSize = 6
		r.circle.Hidden = false
		r.circle.Refresh()
		r.cont.Show()
	}

	r.text.Refresh()
	r.Layout(r.cont.Size())
	canvas.Refresh(r.badge)
}

func (r *notificationBadgeRenderer) Layout(size fyne.Size) {
	r.cont.Resize(size)
	r.cont.Move(fyne.NewPos(0, 0))
}

func (r *notificationBadgeRenderer) MinSize() fyne.Size {
	count, _ := r.badge.count.Get()
	if count == 0 {
		return fyne.NewSize(0, 0)
	}

	// 文字大小
	textSize := r.text.MinSize()

	// 圆圈直径至少12px
	diameter := float32(12)
	// 如果是 Emoji (锁)，可能需要稍大一点
	if count == -1 {
		diameter = 20 // 适配 TextSize 14
	}

	if textSize.Width+4 > diameter {
		diameter = textSize.Width + 4 // 减少 padding
	}

	return fyne.NewSize(diameter, diameter)
}

func (r *notificationBadgeRenderer) Refresh() {
	r.refresh()
}

func (r *notificationBadgeRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.cont}
}

func (r *notificationBadgeRenderer) Destroy() {}

// BadgeContainer 将原始组件和角标组合在一起的容器
type BadgeContainer struct {
	widget.BaseWidget
	child fyne.CanvasObject
	badge *NotificationBadge
}

// NewBadgeContainer 创建带角标的容器
func NewBadgeContainer(child fyne.CanvasObject, badge *NotificationBadge) *BadgeContainer {
	b := &BadgeContainer{
		child: child,
		badge: badge,
	}
	b.ExtendBaseWidget(b)

	// 监听角标变化，强制刷新容器布局
	badge.OnUpdated = func() {
		b.Refresh()
	}

	return b
}

// CreateRenderer 实现 Widget 接口
func (b *BadgeContainer) CreateRenderer() fyne.WidgetRenderer {
	return &badgeContainerRenderer{
		container: b,
		child:     b.child,
		badge:     b.badge,
	}
}

type badgeContainerRenderer struct {
	container *BadgeContainer
	child     fyne.CanvasObject
	badge     *NotificationBadge
}

func (r *badgeContainerRenderer) Layout(size fyne.Size) {
	// 子组件占据全部空间
	r.child.Resize(size)
	r.child.Move(fyne.NewPos(0, 0))

	// 角标放在右上角，稍微向内以防被裁切，但又不遮挡太多
	// 角标放在右上角
	if r.badge.IsVisible() {
		badgeSize := r.badge.MinSize()
		// x = bound.Width - badge.Width / 2 (让它看起来悬挂在右上角)
		// 但为了不超出父容器太多导致被裁剪，我们适度向内

		var x, y float32

		// 针对不同的角标类型做不同的定位
		if r.badge.GetCount() == -1 {
			// 锁图标：稍微放大且向外突出一点
			x = size.Width - badgeSize.Width + 4
			y = -4
		} else {
			// 普通数字角标：贴合边缘
			x = size.Width - badgeSize.Width + 4
			y = -4
		}

		r.badge.Resize(badgeSize)
		r.badge.Move(fyne.NewPos(x, y))
	}
}

func (r *badgeContainerRenderer) MinSize() fyne.Size {
	// 最小尺寸由子组件决定
	return r.child.MinSize()
}

func (r *badgeContainerRenderer) Refresh() {
	r.child.Refresh()
	r.badge.Refresh()

	// 强制重新布局以更新角标位置
	r.Layout(r.container.Size())

	canvas.Refresh(r.container)
}

func (r *badgeContainerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.child, r.badge}
}

func (r *badgeContainerRenderer) Destroy() {}
