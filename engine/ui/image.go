package ui

import (
	"errors"

	"github.com/go-gl/mathgl/mgl32"
)

// 表示不可交互的 UI 图片
//
// 当前使用元素布局边界作为目标区域，通过 ImageSpec 描述纹理来源和绘制方式
type Image struct {
	// 提供父子关系、布局和树遍历能力
	*UIElement
	// 绘制用图片描述
	spec ImageSpec
}

// 创建图片控件
func NewImage(position, size mgl32.Vec2, spec ImageSpec) *Image {
	image := &Image{
		UIElement: NewUIElement(position, size),
		spec:      spec,
	}
	image.SetRenderUI(func(uiCtx *uiContext) error {
		if uiCtx == nil {
			return errors.New("ui context is nil")
		}
		return image.RenderSelf(uiCtx)
	})
	return image
}

// 返回图片描述
func (i *Image) Spec() ImageSpec {
	if i == nil {
		return ImageSpec{}
	}
	return i.spec
}

// 设置图片描述
func (i *Image) SetSpec(spec ImageSpec) {
	if i == nil {
		return
	}
	i.spec = spec
}

// 绘制图片自身
func (i *Image) RenderSelf(uiCtx *uiContext) error {
	if i == nil || i.UIElement == nil || !i.Visible() {
		return nil
	}
	rect := i.Bounds().RectToVec4()
	if rect.Z() <= 0 || rect.W() <= 0 {
		return nil
	}
	return drawImageSpec(uiCtx, i.spec, rect)
}
