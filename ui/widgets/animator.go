package widgets

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// SpinnerAnimator 在标签上显示终端风格的加载动画 (e.g. "Loading... \")
type SpinnerAnimator struct {
	label  *widget.Label
	prefix string
	stop   chan struct{}
	once   sync.Once
}

// NewSpinnerAnimator 创建新的动画控制器
func NewSpinnerAnimator(label *widget.Label, prefix string) *SpinnerAnimator {
	return &SpinnerAnimator{
		label:  label,
		prefix: prefix,
		stop:   make(chan struct{}),
	}
}

// Start 开始动画 (非阻塞)
func (s *SpinnerAnimator) Start() {
	go func() {
		// 经典终端旋转字符
		chars := []string{"|", "/", "-", "\\"}
		i := 0
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				i = (i + 1) % len(chars)
				// 构造文本: "前缀 -"
				text := s.prefix + " " + chars[i]
				fyne.Do(func() {
					s.label.SetText(text)
				})
			}
		}
	}()
}

// Stop 停止动画 (并发安全)
func (s *SpinnerAnimator) Stop() {
	s.once.Do(func() {
		close(s.stop)
	})
}
