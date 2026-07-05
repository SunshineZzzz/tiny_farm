package ui

import (
	"sort"

	"tiny_farm/engine/abstract"
	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
)

// 汇总一次 UI 渲染所需的外部能力
type uiContext struct {
	// 图形绘制入口
	renderer abstract.IRenderer
	// 资源查询入口
	resManager abstract.IResourceManager
	// 文本绘制入口
	textRenderer abstract.ITextRenderer
}

// 创建一次 UI 绘制使用的渲染上下文
func NewUIContext(renderer abstract.IRenderer, resManager abstract.IResourceManager, textRenderer abstract.ITextRenderer) *uiContext {
	return &uiContext{
		renderer:     renderer,
		resManager:   resManager,
		textRenderer: textRenderer,
	}
}

// 表示 UI 元素四边的厚度
//
// 当前用于 margin 和 padding，单位都是逻辑坐标
type Thickness struct {
	// 左侧厚度
	Left float32
	// 上侧厚度
	Top float32
	// 右侧厚度
	Right float32
	// 下侧厚度
	Bottom float32
}

// 返回左右厚度之和
func (t Thickness) Width() float32 {
	return t.Left + t.Right
}

// 返回上下厚度之和
func (t Thickness) Height() float32 {
	return t.Top + t.Bottom
}

// 表示 UI 元素树里的基础节点(组合模式)
//
// 当前只包含纯逻辑能力：父子关系、布局计算、递归 update/render 和命中测试
type UIElement struct {
	// 相对父内容区域锚点的局部位置
	position mgl32.Vec2
	// 元素请求尺寸
	size mgl32.Vec2
	// 是否参与 update、render 和 hit test
	visible bool
	// 是否参与 UIManager 的交互命中
	interactive bool
	// 是否等待被父节点批量移除
	needRemove bool
	// 同级元素排序值，值越大越晚渲染也越先命中
	orderIndex int
	// 可选业务标识，用于从父节点移除或查找
	id string

	// 父节点，不拥有
	parent *UIElement
	// 子节点，由父节点拥有
	children []*UIElement

	// 锚点最小值，基于父内容区域
	anchorMin mgl32.Vec2
	// 锚点最大值，和 anchorMin 不同时会产生拉伸
	anchorMax mgl32.Vec2
	// 枢轴，基于自身尺寸
	// 元素的定位基准点，使用 0.0～1.0 的相对坐标表示
	pivot mgl32.Vec2
	// 内边距，影响子元素布局区域
	padding Thickness
	// 外边距，影响自身相对父内容区域的位置和拉伸尺寸
	margin Thickness

	// 布局是否需要重新计算
	layoutDirty bool
	// 计算后的屏幕坐标
	layoutPosition mgl32.Vec2
	// 计算后的最终尺寸
	layoutSize mgl32.Vec2

	// 自身 update hook，给后续控件在不改树遍历逻辑的情况下挂接行为
	updateSelf func(*UIElement, float64)
	// 带 UI 渲染上下文的自身绘制 hook
	renderUI func(*uiContext) error
	// 鼠标交互事件入口，由 UIInteractive 安装
	mouseEnter    func()
	mouseExit     func()
	mousePressed  func()
	mouseReleased func(bool)
	// 布局完成后的 hook，供后续 layout 容器调整子元素
	onLayout func(*UIElement)
}

// 创建一个可见 UI 元素
func NewUIElement(position, size mgl32.Vec2) *UIElement {
	return &UIElement{
		position:    position,
		size:        size,
		visible:     true,
		layoutDirty: true,
	}
}

// 返回局部位置
func (e *UIElement) Position() mgl32.Vec2 {
	if e == nil {
		return mgl32.Vec2{}
	}
	return e.position
}

// 返回请求尺寸
func (e *UIElement) Size() mgl32.Vec2 {
	if e == nil {
		return mgl32.Vec2{}
	}
	return e.size
}

// 返回元素是否可见
func (e *UIElement) Visible() bool {
	return e != nil && e.visible
}

// 返回元素是否参与交互命中
func (e *UIElement) Interactive() bool {
	return e != nil && e.interactive
}

// 返回元素是否等待移除
func (e *UIElement) NeedRemove() bool {
	return e != nil && e.needRemove
}

// 返回同级排序值
func (e *UIElement) OrderIndex() int {
	if e == nil {
		return 0
	}
	return e.orderIndex
}

// 返回元素标识
func (e *UIElement) ID() string {
	if e == nil {
		return ""
	}
	return e.id
}

// 返回父元素
func (e *UIElement) Parent() *UIElement {
	if e == nil {
		return nil
	}
	return e.parent
}

// 返回子元素快照
func (e *UIElement) Children() []*UIElement {
	if e == nil || len(e.children) == 0 {
		return nil
	}
	children := make([]*UIElement, len(e.children))
	copy(children, e.children)
	return children
}

// 返回锚点最小值
func (e *UIElement) AnchorMin() mgl32.Vec2 {
	if e == nil {
		return mgl32.Vec2{}
	}
	return e.anchorMin
}

// 返回锚点最大值
func (e *UIElement) AnchorMax() mgl32.Vec2 {
	if e == nil {
		return mgl32.Vec2{}
	}
	return e.anchorMax
}

// 返回枢轴
func (e *UIElement) Pivot() mgl32.Vec2 {
	if e == nil {
		return mgl32.Vec2{}
	}
	return e.pivot
}

// 返回内边距
func (e *UIElement) Padding() Thickness {
	if e == nil {
		return Thickness{}
	}
	return e.padding
}

// 返回外边距
func (e *UIElement) Margin() Thickness {
	if e == nil {
		return Thickness{}
	}
	return e.margin
}

// 返回计算后的屏幕坐标
func (e *UIElement) ScreenPosition() mgl32.Vec2 {
	if e == nil {
		return mgl32.Vec2{}
	}
	e.ensureLayout()
	return e.layoutPosition
}

// 返回计算后的最终尺寸
func (e *UIElement) LayoutSize() mgl32.Vec2 {
	if e == nil {
		return mgl32.Vec2{}
	}
	e.ensureLayout()
	return e.layoutSize
}

// 返回屏幕坐标下的元素边界
func (e *UIElement) Bounds() emath.Rect {
	if e == nil {
		return emath.Rect{}
	}
	e.ensureLayout()
	return emath.Rect{
		Position: e.layoutPosition,
		Size:     e.layoutSize,
	}
}

// 返回屏幕坐标下的内容区域
func (e *UIElement) ContentBounds() emath.Rect {
	if e == nil {
		return emath.Rect{}
	}
	bounds := e.Bounds()
	width := max(bounds.Size.X()-e.padding.Width(), 0)
	height := max(bounds.Size.Y()-e.padding.Height(), 0)
	return emath.Rect{
		Position: bounds.Position.Add(mgl32.Vec2{e.padding.Left, e.padding.Top}),
		Size:     mgl32.Vec2{width, height},
	}
}

// 设置局部位置并标记布局失效
func (e *UIElement) SetPosition(position mgl32.Vec2) {
	if e == nil {
		return
	}
	e.position = position
	e.invalidateLayout()
}

// 设置请求尺寸并标记布局失效
func (e *UIElement) SetSize(size mgl32.Vec2) {
	if e == nil {
		return
	}
	e.size = size
	e.invalidateLayout()
	if e.parent != nil {
		// 只有会改变"父容器怎么排孩子们"的属性才需要 parent.invalidateLayout()，目前就是 size 和 visible，orderIndex。
		// position、anchor、pivot、margin，都属于"父已经给我划好格子，我来决定自己怎么站进去"
		e.parent.invalidateLayout()
	}
}

// 设置是否可见
func (e *UIElement) SetVisible(visible bool) {
	if e == nil {
		return
	}
	e.visible = visible
	if e.parent != nil {
		e.parent.invalidateLayout()
	}
}

// 设置是否参与交互命中
func (e *UIElement) SetInteractive(interactive bool) {
	if e == nil {
		return
	}
	e.interactive = interactive
}

// 设置鼠标交互事件入口
func (e *UIElement) SetMouseHandlers(enter, exit, pressed func(), released func(bool)) {
	if e == nil {
		return
	}
	e.mouseEnter = enter
	e.mouseExit = exit
	e.mousePressed = pressed
	e.mouseReleased = released
}

// 触发鼠标进入回调
func (e *UIElement) handleMouseEnter() {
	if e != nil && e.mouseEnter != nil {
		e.mouseEnter()
	}
}

// 触发鼠标离开回调
func (e *UIElement) handleMouseExit() {
	if e != nil && e.mouseExit != nil {
		e.mouseExit()
	}
}

// 触发鼠标按下回调
func (e *UIElement) handleMousePressed() {
	if e != nil && e.mousePressed != nil {
		e.mousePressed()
	}
}

// 触发鼠标释放回调，并传递释放位置是否仍在元素内
func (e *UIElement) handleMouseReleased(inside bool) {
	if e != nil && e.mouseReleased != nil {
		e.mouseReleased(inside)
	}
}

// 设置是否等待移除
func (e *UIElement) SetNeedRemove(needRemove bool) {
	if e == nil {
		return
	}
	e.needRemove = needRemove
}

// 设置同级排序值
func (e *UIElement) SetOrderIndex(orderIndex int) {
	if e == nil {
		return
	}
	e.orderIndex = orderIndex
	if e.parent != nil {
		e.parent.SortChildrenByOrderIndex()
		e.parent.invalidateLayout()
	}
}

// 设置元素标识
func (e *UIElement) SetID(id string) {
	if e == nil {
		return
	}
	e.id = id
}

// 设置锚点并标记布局失效
func (e *UIElement) SetAnchor(anchorMin, anchorMax mgl32.Vec2) {
	if e == nil {
		return
	}
	e.anchorMin = anchorMin
	e.anchorMax = anchorMax
	e.invalidateLayout()
}

// 设置枢轴并标记布局失效
func (e *UIElement) SetPivot(pivot mgl32.Vec2) {
	if e == nil {
		return
	}
	e.pivot = pivot
	e.invalidateLayout()
}

// 设置内边距并标记子布局失效
func (e *UIElement) SetPadding(padding Thickness) {
	if e == nil {
		return
	}
	e.padding = padding
	for _, child := range e.children {
		child.invalidateLayout()
	}
}

// 设置外边距并标记布局失效
func (e *UIElement) SetMargin(margin Thickness) {
	if e == nil {
		return
	}
	e.margin = margin
	e.invalidateLayout()
}

// 设置自身更新 hook
func (e *UIElement) SetUpdateSelf(callback func(*UIElement, float64)) {
	if e == nil {
		return
	}
	e.updateSelf = callback
}

// 设置 UI 绘制 hook
func (e *UIElement) SetRenderUI(callback func(*uiContext) error) {
	if e == nil {
		return
	}
	e.renderUI = callback
}

// 设置布局完成 hook
func (e *UIElement) SetOnLayout(callback func(*UIElement)) {
	if e == nil {
		return
	}
	e.onLayout = callback
}

// 添加子元素并建立父子关系
//
// 如果子元素已经有父节点，会先从旧父节点移除
func (e *UIElement) AddChild(child *UIElement, orderIndex ...int) bool {
	if e == nil || child == nil || child == e || e.hasAncestor(child) {
		return false
	}
	if len(orderIndex) > 0 {
		child.orderIndex = orderIndex[0]
	}
	if child.parent != nil {
		child.parent.RemoveChild(child)
	}
	child.parent = e
	e.children = append(e.children, child)
	e.SortChildrenByOrderIndex()
	e.invalidateLayout()
	return true
}

// 移除指定子元素并返回它
func (e *UIElement) RemoveChild(child *UIElement) *UIElement {
	if e == nil || child == nil {
		return nil
	}
	for index, current := range e.children {
		if current != child {
			continue
		}
		e.children = append(e.children[:index], e.children[index+1:]...)
		current.parent = nil
		current.invalidateLayout()
		e.invalidateLayout()
		return current
	}
	return nil
}

// 按标识移除子元素并返回它
func (e *UIElement) RemoveChildByID(id string) *UIElement {
	if e == nil || id == "" {
		return nil
	}
	for _, child := range e.children {
		if child.id == id {
			return e.RemoveChild(child)
		}
	}
	return nil
}

// 按标识查找直接子元素
func (e *UIElement) GetChildByID(id string) *UIElement {
	if e == nil || id == "" {
		return nil
	}
	for _, child := range e.children {
		if child != nil && child.id == id {
			return child
		}
	}
	return nil
}

// 移除全部子元素
func (e *UIElement) RemoveAllChildren() {
	if e == nil {
		return
	}
	for _, child := range e.children {
		child.parent = nil
		child.invalidateLayout()
	}
	e.children = nil
	e.invalidateLayout()
}

// 移除标记为 needRemove 的直接子元素
func (e *UIElement) RemoveMarkedChildren() {
	if e == nil || len(e.children) == 0 {
		return
	}
	kept := e.children[:0]
	for _, child := range e.children {
		if child == nil || child.needRemove {
			if child != nil {
				child.parent = nil
				child.invalidateLayout()
			}
			continue
		}
		kept = append(kept, child)
	}
	e.children = kept
}

// 根据 orderIndex 稳定排序子元素
func (e *UIElement) SortChildrenByOrderIndex() {
	if e == nil || len(e.children) < 2 {
		return
	}
	sort.SliceStable(e.children, func(i, j int) bool {
		return e.children[i].orderIndex < e.children[j].orderIndex
	})
}

// 递归更新自身和可见子元素
func (e *UIElement) Update(deltaTime float64) {
	if e == nil {
		return
	}
	// 确保布局是最新的
	e.ensureLayout()
	if !e.visible {
		return
	}
	if e.updateSelf != nil {
		e.updateSelf(e, deltaTime)
	}
	// 原地过滤切片写法，避免分配新的切片内存
	children := e.children[:0]
	for _, child := range e.children {
		if child == nil || child.needRemove {
			if child != nil {
				child.parent = nil
				child.invalidateLayout()
			}
			continue
		}
		child.Update(deltaTime)
		children = append(children, child)
	}
	e.children = children
}

// 递归渲染自身和可见子元素，并向控件传递 UI 渲染上下文
func (e *UIElement) RenderUI(uiCtx *uiContext) error {
	if e == nil {
		return nil
	}
	e.ensureLayout()
	if !e.visible {
		return nil
	}
	if e.renderUI != nil {
		if err := e.renderUI(uiCtx); err != nil {
			return err
		}
	}
	for _, child := range e.children {
		if err := child.RenderUI(uiCtx); err != nil {
			return err
		}
	}
	return nil
}

// 判断屏幕坐标点是否在元素边界内
func (e *UIElement) IsPointInside(point mgl32.Vec2) bool {
	if e == nil || !e.visible {
		return false
	}
	bounds := e.Bounds()
	return point.X() >= bounds.Position.X() &&
		point.Y() >= bounds.Position.Y() &&
		point.X() < bounds.Position.X()+bounds.Size.X() &&
		point.Y() < bounds.Position.Y()+bounds.Size.Y()
}

// 从当前元素开始查找命中的最上层交互元素
func (e *UIElement) FindInteractiveAt(point mgl32.Vec2) *UIElement {
	if e == nil || !e.visible {
		return nil
	}
	// 在寻找鼠标交互目标前，确保当前元素的实际位置和尺寸是最新的
	e.ensureLayout()
	for index := len(e.children) - 1; index >= 0; index-- {
		if hit := e.children[index].FindInteractiveAt(point); hit != nil {
			return hit
		}
	}
	if e.interactive && e.IsPointInside(point) {
		return e
	}
	return nil
}

// 标记当前元素及其子树需要重新计算布局
func (e *UIElement) invalidateLayout() {
	if e == nil {
		return
	}
	if !e.layoutDirty {
		e.layoutDirty = true
	}
	for _, child := range e.children {
		child.invalidateLayout()
	}
}

// 按需计算当前元素的屏幕位置和最终尺寸
func (e *UIElement) ensureLayout() {
	if e == nil || !e.layoutDirty {
		return
	}

	// 看看是否是根元素
	if e.parent == nil {
		e.layoutSize = emath.Mgl32Vec2Max(e.size, mgl32.Vec2{0.0, 0.0})
		e.layoutPosition = e.position
		e.layoutDirty = false
		if e.onLayout != nil {
			e.onLayout(e)
		}
		return
	}

	// 取得父元素的可用内容区域，即父元素边界扣除 padding 后的区域
	parentContent := e.parent.ContentBounds()
	// 在父内容区域中，根据 anchorMin 的比例找到锚点的屏幕坐标
	anchorPosition := parentContent.Position.Add(emath.Mgl32Vec2MulElem(parentContent.Size, e.anchorMin))
	// 计算两个锚点之间覆盖的实际尺寸，也就是元素可拉伸的区域大小
	// 固定点锚点，stretchSize 为 {0.0, 0.0}
	stretchSize := emath.Mgl32Vec2MulElem(parentContent.Size, e.anchorMax.Sub(e.anchorMin))
	// 默认使用元素自己设置的尺寸
	layoutSize := e.size
	// 锚点之间是否存在拉伸范围
	if stretchSize.X() != 0.0 || stretchSize.Y() != 0.0 {
		// 改用父区域提供的拉伸尺寸，并扣除左右、上下外边距
		layoutSize = stretchSize.Sub(mgl32.Vec2{e.margin.Width(), e.margin.Height()})
	}
	e.layoutSize = emath.Mgl32Vec2Max(layoutSize, mgl32.Vec2{0.0, 0.0})
	// 计算元素最终的左上角屏幕坐标
	e.layoutPosition = anchorPosition. // 父元素中的锚点位置
						Add(e.position).                                   // 相对锚点的手动偏移
						Add(mgl32.Vec2{e.margin.Left, e.margin.Top}).      // 外边距
						Sub(emath.Mgl32Vec2MulElem(e.layoutSize, e.pivot)) // 把 pivot 对齐到锚点
	e.layoutDirty = false
	if e.onLayout != nil {
		e.onLayout(e)
	}
}

// 判断目标元素是否位于当前元素的祖先链中
func (e *UIElement) hasAncestor(target *UIElement) bool {
	for current := e.parent; current != nil; current = current.parent {
		if current == target {
			return true
		}
	}
	return false
}
