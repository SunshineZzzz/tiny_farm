package ui

import (
	"errors"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils/defs"

	"github.com/go-gl/mathgl/mgl32"
)

// 表示带四态视觉和文字的交互按钮
//
// 当前根据交互状态选择图片和文字覆盖，并复用 UIInteractive 处理输入与声音
type Button struct {
	// 提供交互状态、鼠标事件和声音播放能力
	*UIInteractive
	// 按钮各状态的视觉和声音配置
	skin *ButtonSkin
	// 可选的内部文字控件
	label *Label
}

// 创建按钮并安装状态、渲染和声音事件处理
//
// 普通状态图片必须提供有效纹理键和正数源区域
func NewButton(position, size mgl32.Vec2, skin *ButtonSkin, audio abstract.IAudioPlayer) (*Button, error) {
	if skin == nil || skin.Normal == nil || skin.Normal.ResolvedTextureKey() == "" || skin.Normal.SourceRect.Z() <= 0 || skin.Normal.SourceRect.W() <= 0 {
		return nil, errors.New("button normal image is invalid")
	}
	button := &Button{
		UIInteractive: NewUIInteractive(position, size, audio),
		skin:          skin,
	}
	button.AddImage(UIStateNormal, *skin.Normal)
	if skin.Hover != nil {
		button.AddImage(UIStateHover, *skin.Hover)
	}
	if skin.Pressed != nil {
		button.AddImage(UIStatePressed, *skin.Pressed)
	}
	if skin.Disabled != nil {
		button.AddImage(UIStateDisabled, *skin.Disabled)
	}
	if style := skin.Label; style != nil {
		fontSize := style.FontSize
		if fontSize <= 0 {
			fontSize = 16
		}
		button.label = NewLabel(mgl32.Vec2{}, style.Text, defs.ResourceKey(style.FontPath), fontSize)
		button.label.SetFont(defs.ResourceKey(style.FontPath), style.FontPath, fontSize)
		button.label.SetColor(resolvedLabelColor(style.Color))
		button.label.SetOffset(style.Offset)
	}
	button.SetStateCallback(button.applyState)
	button.SetRenderUI(button.render)
	button.applyState(UIStateNormal)
	for event, value := range skin.Sounds {
		if value == nil || *value == "" {
			button.DisableSoundEvent(event)
			continue
		}
		button.SetSoundEvent(event, defs.ResourceKey(*value), *value)
	}
	return button, nil
}

// 设置按钮文字，未配置文字控件时忽略
func (b *Button) SetText(text string) {
	if b != nil && b.label != nil {
		b.label.SetText(text)
	}
}

// 返回按钮文字，未配置文字控件时返回空字符串
func (b *Button) Text() string {
	if b == nil || b.label == nil {
		return ""
	}
	return b.label.Text()
}

// 根据交互状态应用文字颜色和偏移覆盖
func (b *Button) applyState(state UIState) {
	if b == nil || b.label == nil || b.skin.Label == nil {
		return
	}
	style := b.skin.Label
	color := resolvedLabelColor(style.Color)
	offset := style.Offset
	var override *ButtonLabelStyleOverride
	switch state {
	case UIStateHover:
		override = b.skin.HoverLabel
	case UIStatePressed:
		override = b.skin.PressedLabel
	case UIStateDisabled:
		override = b.skin.DisabledLabel
	}
	if override != nil {
		if override.Color != nil {
			color = *override.Color
		}
		if override.Offset != nil {
			offset = *override.Offset
		}
	}
	b.label.SetColor(color)
	b.label.SetOffset(offset)
}

// 绘制当前状态图片，并将可选文字居中绘制在按钮内
func (b *Button) render(uiCtx *uiContext) error {
	if b == nil || uiCtx == nil {
		return errors.New("button or ui context is nil")
	}
	if err := b.drawStateImage(uiCtx); err != nil {
		return err
	}
	if b.label == nil {
		return nil
	}
	size, err := b.label.Measure(uiCtx)
	if err != nil {
		return err
	}
	position := b.ScreenPosition().Add(b.LayoutSize().Sub(size).Mul(0.5))
	b.label.SetPosition(position)
	return b.label.RenderSelf(uiCtx)
}

// 返回按钮文字实际使用的颜色，零值按白色处理
func resolvedLabelColor(color mgl32.Vec4) mgl32.Vec4 {
	if color == (mgl32.Vec4{}) {
		return mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	return color
}
