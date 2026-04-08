package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// ColumnDef 列定义
type ColumnDef struct {
	// WidthPercent 列宽百分比（0-100），例如 20 表示 20%
	WidthPercent float32
	// Alignment 对齐方式
	Alignment fyne.TextAlign
}

// ResponsiveTable 响应式表格容器
type ResponsiveTable struct {
	widget.BaseWidget
	*widget.Table
	columnDefs  []ColumnDef
	lastWidth   float32
}

// NewResponsiveTable 创建响应式表格容器
// columnDefs: 列定义数组，如果为空则均分列宽
func NewResponsiveTable(table *widget.Table, columnDefs []ColumnDef) *ResponsiveTable {
	r := &ResponsiveTable{
		Table:      table,
		columnDefs: columnDefs,
	}
	r.ExtendBaseWidget(r)
	return r
}

// GetColumnAlignment 获取指定列的对齐方式
func (r *ResponsiveTable) GetColumnAlignment(col int) fyne.TextAlign {
	if col < 0 || col >= len(r.columnDefs) {
		return fyne.TextAlignLeading // 默认左对齐
	}
	alignment := r.columnDefs[col].Alignment
	if alignment == 0 {
		return fyne.TextAlignLeading // 如果未设置，默认左对齐
	}
	return alignment
}

// CreateRenderer 实现 Widget 接口
func (r *ResponsiveTable) CreateRenderer() fyne.WidgetRenderer {
	return &responsiveTableRenderer{
		table:     r.Table,
		container: r,
	}
}

// responsiveTableRenderer 渲染器
type responsiveTableRenderer struct {
	table     *widget.Table
	container *ResponsiveTable
}

func (r *responsiveTableRenderer) Layout(size fyne.Size) {
	r.table.Resize(size)

	// 当宽度变化时，重新计算列宽
	if r.container.lastWidth != size.Width {
		r.container.lastWidth = size.Width

		// 动态计算列宽，确保总和 = 100%
		// 预留足够空间给垂直滚动条，避免出现水平滚动条
		totalWidth := size.Width - 40

		if len(r.container.columnDefs) == 0 {
			// 如果没有列定义，均分列宽
			return
		}

		// 计算所有列宽百分比的总和
		var totalPercent float32 = 0
		for _, col := range r.container.columnDefs {
			totalPercent += col.WidthPercent
		}

		// 设置每列宽度
		for i, col := range r.container.columnDefs {
			if i == len(r.container.columnDefs)-1 {
				// 最后一列：自动调整以确保总和为 100%
				remainingPercent := 100.0 - (totalPercent - col.WidthPercent)
				if remainingPercent > 0 {
					r.table.SetColumnWidth(i, totalWidth*(remainingPercent/100.0))
				} else {
					// 如果总和已经超过100%，仍使用定义的宽度
					r.table.SetColumnWidth(i, totalWidth*(col.WidthPercent/100.0))
				}
			} else {
				// 其他列：按定义的百分比设置
				r.table.SetColumnWidth(i, totalWidth*(col.WidthPercent/100.0))
			}
		}
	}
}

func (r *responsiveTableRenderer) MinSize() fyne.Size {
	return r.table.MinSize()
}

func (r *responsiveTableRenderer) Refresh() {
	r.table.Refresh()
}

func (r *responsiveTableRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.table}
}

func (r *responsiveTableRenderer) Destroy() {}
