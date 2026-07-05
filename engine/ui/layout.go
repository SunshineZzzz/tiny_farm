package ui

import (
	"github.com/go-gl/mathgl/mgl32"
)

// 表示布局使用的主轴方向
type Orientation int

const (
	// 子元素从上到下排列
	OrientationVertical Orientation = iota
	// 子元素从左到右排列
	OrientationHorizontal
)

// 表示内容在可用空间内的对齐方式
type AxisAlignment int

const (
	// 内容贴近当前轴的起点
	AxisAlignmentStart AxisAlignment = iota
	// 内容在当前轴上居中
	AxisAlignmentCenter
	// 内容贴近当前轴的终点
	AxisAlignmentEnd
)

// 流式排版，将可见子元素排列成单行或单列
//
// 选一个方向当主轴，沿主轴把可见子元素依次排开。
type StackLayout struct {
	*UIElement
	// 子元素按垂直方向还是水平方向排列
	orientation Orientation
	// 相邻可见子元素之间的间距
	spacing float32
	// 主轴，元素排列的方向
	mainAxis AxisAlignment
	// 交叉轴，与主轴垂直的方向
	crossAxis AxisAlignment
}

// 创建堆叠布局元素并注册布局回调
func NewStackLayout(position, size mgl32.Vec2) *StackLayout {
	layout := &StackLayout{UIElement: NewUIElement(position, size)}
	layout.SetOnLayout(func(*UIElement) { layout.applyLayout() })
	return layout
}

// 修改堆叠布局的排列方向并标记布局失效
func (l *StackLayout) SetOrientation(value Orientation) {
	l.orientation = value
	l.invalidateLayout()
}

// 修改可见子元素之间的间距，负数按 0 处理
func (l *StackLayout) SetSpacing(value float32) { l.spacing = max(value, 0); l.invalidateLayout() }

// 修改子元素在主轴和交叉轴上的对齐方式
func (l *StackLayout) SetAxisAlignment(main, cross AxisAlignment) {
	l.mainAxis, l.crossAxis = main, cross
	l.invalidateLayout()
}

// 根据堆叠布局配置摆放每个可见子元素
func (l *StackLayout) applyLayout() {
	children := visibleChildren(l.UIElement)
	if len(children) == 0 {
		return
	}

	// 当前 layout 自己扣掉 padding 后的可用区域
	content := l.ContentBounds()
	// 上所有子元素之间的间距总和
	total := l.spacing * float32(len(children)-1)
	// 垂直布局时累加高度；水平布局时累加宽度
	for _, child := range children {
		if l.orientation == OrientationVertical {
			total += child.Size().Y()
		} else {
			total += child.Size().X()
		}
	}
	// 当前布局方向上，可以用来摆放子元素的空间长度
	// 如果是水平排列，主轴是 X，空间长度就是内容宽度
	available := content.Size.X()
	if l.orientation == OrientationVertical {
		// 垂直排列，主轴是 Y，空间长度就是内容高度
		available = content.Size.Y()
	}
	// 第一个子元素在主轴上的起始位置
	// 根据主轴对齐方式算起始 cursor
	cursor := alignOffset(available, total, l.mainAxis)
	// 逐个设置子元素位置
	for _, child := range children {
		size := child.Size()
		if l.orientation == OrientationVertical {
			// 子元素从上到下排，所以主轴是 Y 轴，交叉轴是 X 轴

			// Y 用 cursor，因为垂直排列时元素一个接一个往下放，X 根据 crossAxis 决定左对齐、居中、还是右对齐
			child.SetPosition(mgl32.Vec2{alignOffset(content.Size.X(), size.X(), l.crossAxis), cursor})
			cursor += size.Y() + l.spacing
		} else {
			// 子元素从左到右排，所以主轴是 X 轴，交叉轴是 Y 轴

			// X 用 cursor，因为水平排列时元素一个接一个往右放，Y 根据 crossAxis 决定上对齐、居中、还是下对齐
			child.SetPosition(mgl32.Vec2{cursor, alignOffset(content.Size.Y(), size.Y(), l.crossAxis)})
			cursor += size.X() + l.spacing
		}
	}
}

// 将可见子元素排列到固定尺寸的网格单元中
//
// 子元素按行优先顺序摆放，只设置位置，不强制修改子元素尺寸
type GridLayout struct {
	*UIElement
	// 每行的网格单元数量，最小值为 1
	columns int
	// 每个网格单元使用的固定步进尺寸
	cellSize mgl32.Vec2
	// 相邻网格单元之间的水平和垂直间距
	spacing mgl32.Vec2
}

// 创建网格布局元素并注册布局回调
func NewGridLayout(position, size mgl32.Vec2, columns int, cellSize mgl32.Vec2) *GridLayout {
	layout := &GridLayout{UIElement: NewUIElement(position, size), columns: max(columns, 1), cellSize: cellSize}
	layout.SetOnLayout(func(*UIElement) { layout.applyLayout() })
	return layout
}

// 修改网格的水平和垂直间距，负数按 0 处理
func (l *GridLayout) SetSpacing(value mgl32.Vec2) {
	l.spacing = mgl32.Vec2{max(value.X(), 0), max(value.Y(), 0)}
	l.invalidateLayout()
}

// 按行优先顺序摆放每个可见子元素
func (l *GridLayout) applyLayout() {
	for index, child := range visibleChildren(l.UIElement) {
		column := index % l.columns
		row := index / l.columns
		child.SetPosition(mgl32.Vec2{
			float32(column) * (l.cellSize.X() + l.spacing.X()),
			float32(row) * (l.cellSize.Y() + l.spacing.Y()),
		})
	}
}

// 返回当前可见且非空的子元素列表
func visibleChildren(element *UIElement) []*UIElement {
	result := make([]*UIElement, 0, len(element.children))
	for _, child := range element.children {
		if child != nil && child.Visible() {
			result = append(result, child)
		}
	}
	return result
}

// 根据主轴对齐方式计算内容在可用空间内的起始偏移量
// available，可用空间长度
// used，实际要摆放的内容长度
func alignOffset(available, used float32, alignment AxisAlignment) float32 {
	switch alignment {
	case AxisAlignmentCenter:
		// 对齐到可用空间的中心
		return (available - used) * 0.5
	case AxisAlignmentEnd:
		// 对齐到可用空间的末尾
		return available - used
	default:
		return 0.0
	}
}
