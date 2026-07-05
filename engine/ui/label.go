package ui

import (
	"errors"

	"tiny_farm/engine/utils/defs"

	"github.com/go-gl/mathgl/mgl32"
)

// 表示不可交互的 UI 文本
//
// 当前只负责文本内容、字体、字号、颜色、偏移和测量缓存
type Label struct {
	// 提供父子关系、布局和树遍历能力
	*UIElement
	// 文本内容
	text string
	// 字体资源 key
	fontKey defs.ResourceKey
	// 可选字体路径
	fontPath string
	// 字号
	fontSize int
	// 文本颜色
	color mgl32.Vec4
	// 相对元素左上角的文本偏移
	offset mgl32.Vec2
	// 最近一次测量得到的文本尺寸
	measuredSize mgl32.Vec2
	// 文本或字体参数变更后需要重新测量
	measureDirty bool
	// 最近一次测量时的文本布局版本
	lastLayoutRevision uint64
}

// 创建文本控件
func NewLabel(position mgl32.Vec2, text string, fontKey defs.ResourceKey, fontSize int) *Label {
	if fontSize <= 0 {
		fontSize = 16
	}
	label := &Label{
		UIElement:    NewUIElement(position, mgl32.Vec2{}),
		text:         text,
		fontKey:      fontKey,
		fontSize:     fontSize,
		color:        mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
		measureDirty: true,
	}
	label.SetRenderUI(func(uiCtx *uiContext) error {
		if uiCtx == nil {
			return errors.New("ui context is nil")
		}
		return label.RenderSelf(uiCtx)
	})
	return label
}

// 返回文本内容
func (l *Label) Text() string {
	if l == nil {
		return ""
	}
	return l.text
}

// 设置文本内容并标记需要重新测量
func (l *Label) SetText(text string) {
	if l == nil || l.text == text {
		return
	}
	l.text = text
	l.measureDirty = true
}

// 设置字体参数并标记需要重新测量
func (l *Label) SetFont(fontKey defs.ResourceKey, fontPath string, fontSize int) {
	if l == nil {
		return
	}
	if fontSize <= 0 {
		fontSize = 16
	}
	l.fontKey = fontKey
	l.fontPath = fontPath
	l.fontSize = fontSize
	l.measureDirty = true
}

// 设置文本颜色
func (l *Label) SetColor(color mgl32.Vec4) {
	if l == nil {
		return
	}
	l.color = color
}

// 设置文本偏移
func (l *Label) SetOffset(offset mgl32.Vec2) {
	if l == nil {
		return
	}
	l.offset = offset
}

// 返回最近一次测量得到的文本尺寸
func (l *Label) MeasuredSize() mgl32.Vec2 {
	if l == nil {
		return mgl32.Vec2{}
	}
	return l.measuredSize
}

// 测量文本，并同步元素请求尺寸
func (l *Label) Measure(uiCtx *uiContext) (mgl32.Vec2, error) {
	if l == nil {
		return mgl32.Vec2{}, nil
	}
	if !l.measureDirty {
		return l.measuredSize, nil
	}
	if uiCtx == nil {
		return mgl32.Vec2{}, errors.New("ui context is nil")
	}
	size, err := uiCtx.textRenderer.MeasureText(l.text, l.fontKey, l.fontSize, defs.FontPath(l.fontPath))
	if err != nil {
		return mgl32.Vec2{}, err
	}
	l.measuredSize = size
	l.measureDirty = false
	l.lastLayoutRevision = uiCtx.textRenderer.LayoutRevision()
	l.SetSize(size)
	return size, nil
}

// 绘制文本自身
func (l *Label) RenderSelf(uiCtx *uiContext) error {
	if l == nil || l.UIElement == nil || !l.Visible() || l.text == "" || uiCtx.textRenderer == nil {
		return nil
	}
	if uiCtx.textRenderer.LayoutRevision() != l.lastLayoutRevision {
		l.measureDirty = true
	}
	if _, err := l.Measure(uiCtx); err != nil {
		return err
	}
	overrides := &defs.TextRenderOverrides{
		Color: &defs.ColorOptions{
			StartColor: l.color,
			EndColor:   l.color,
		},
	}
	return uiCtx.textRenderer.DrawUIText(l.text, l.fontKey, l.fontSize, l.ScreenPosition().Add(l.offset), defs.FontPath(l.fontPath), overrides)
}
