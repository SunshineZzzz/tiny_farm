package ui

import (
	"errors"

	"github.com/go-gl/mathgl/mgl32"
)

// 表示不可交互的 UI 面板
//
// 当前支持纯色背景和可选背景图，用于作为 root 或普通容器
type Panel struct {
	// 提供父子关系、布局和树遍历能力
	*UIElement
	// 背景色
	color mgl32.Vec4
	// 是否绘制背景色
	colorEnabled bool
	// 可选背景图
	image *ImageSpec
}

// 创建面板控件
func NewPanel(position, size mgl32.Vec2) *Panel {
	panel := &Panel{
		UIElement: NewUIElement(position, size),
	}
	panel.SetRenderUI(func(uiCtx *uiContext) error {
		if uiCtx == nil {
			return errors.New("ui context is nil")
		}
		return panel.RenderSelf(uiCtx)
	})
	return panel
}

// 设置纯色背景
func (p *Panel) SetColor(color mgl32.Vec4) {
	if p == nil {
		return
	}
	p.color = color
	p.colorEnabled = true
}

// 清除纯色背景
func (p *Panel) ClearColor() {
	if p == nil {
		return
	}
	p.color = mgl32.Vec4{}
	p.colorEnabled = false
}

// 设置背景图
func (p *Panel) SetImage(image ImageSpec) {
	if p == nil {
		return
	}
	p.image = &image
}

// 清除背景图
func (p *Panel) ClearImage() {
	if p == nil {
		return
	}
	p.image = nil
}

// 绘制面板自身
func (p *Panel) RenderSelf(uiCtx *uiContext) error {
	if p == nil || p.UIElement == nil || !p.Visible() {
		return nil
	}
	rect := p.Bounds().RectToVec4()
	if rect.Z() <= 0 || rect.W() <= 0 {
		return nil
	}
	if p.colorEnabled {
		if uiCtx.renderer == nil {
			return errors.New("renderer is nil")
		}
		if err := uiCtx.renderer.DrawUIRect(rect, p.color); err != nil {
			return err
		}
	}
	if p.image != nil {
		return drawImageSpec(uiCtx, *p.image, rect)
	}
	return nil
}
