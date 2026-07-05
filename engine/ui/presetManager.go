package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils"
	"tiny_farm/engine/utils/defs"
	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
)

// 描述按钮默认状态下的文字内容和样式
type ButtonLabelStyle struct {
	// 显示的文字内容
	Text string
	// 字体资源路径，同时作为默认资源键
	FontPath string
	// 字体像素字号，非正数时使用 16
	FontSize int
	// 文字颜色，零值使用白色
	Color mgl32.Vec4
	// 文字相对按钮居中位置的偏移
	Offset mgl32.Vec2
}

// 描述按钮特定状态对默认文字样式的可选覆盖
type ButtonLabelStyleOverride struct {
	// 覆盖文字颜色，nil 表示沿用默认颜色
	Color *mgl32.Vec4
	// 覆盖文字偏移，nil 表示沿用默认偏移
	Offset *mgl32.Vec2
}

// 描述按钮各状态使用的图片、文字和声音资源
type ButtonSkin struct {
	// 普通状态图片，也是其他状态未配置图片时的回退值
	Normal *ImageSpec
	// 鼠标悬停状态图片，nil 表示使用普通状态图片
	Hover *ImageSpec
	// 按下状态图片，nil 表示使用普通状态图片
	Pressed *ImageSpec
	// 禁用状态图片，nil 表示使用普通状态图片
	Disabled *ImageSpec
	// 可选的按钮文字基础样式
	Label *ButtonLabelStyle
	// 悬停状态文字覆盖
	HoverLabel *ButtonLabelStyleOverride
	// 按下状态文字覆盖
	PressedLabel *ButtonLabelStyleOverride
	// 禁用状态文字覆盖
	DisabledLabel *ButtonLabelStyleOverride
	// 交互事件到声音资源路径的映射，nil 或空路径表示禁用对应事件声音
	Sounds map[string]*string
}

// 保存图片预设的原始 JSON 字段
type rawImageSpec struct {
	// 纹理资源路径
	Path string `json:"path"`
	// 可选的纹理资源键
	ID string `json:"id"`
	// 源图片区域，依次为 x、y、宽度和高度
	Source []float32 `json:"source"`
	// 是否沿水平方向翻转纹理
	Flipped bool `json:"flipped"`
	// 图片自身的九宫格边距，优先于外部继承值
	NineSlice *NineSliceMargins `json:"nine_slice"`
}

// 校验原始图片字段并转换为运行时图片描述
//
// 图片未配置九宫格边距时使用 inherited
func (r *rawImageSpec) toSpec(inherited *NineSliceMargins) (*ImageSpec, error) {
	if r.Path == "" && r.ID == "" {
		return nil, errors.New("path or id is required")
	}
	if len(r.Source) != 4 || r.Source[2] <= 0 || r.Source[3] <= 0 {
		return nil, errors.New("source must contain positive x/y/width/height")
	}
	key := defs.ResourceKey(r.ID)
	if key == "" {
		key = defs.ResourceKey(r.Path)
	}
	margins := r.NineSlice
	if margins == nil {
		margins = inherited
	}
	return &ImageSpec{
		TextureKey: key,
		Path:       r.Path,
		SourceRect: mgl32.Vec4{r.Source[0], r.Source[1], r.Source[2], r.Source[3]},
		Color:      mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
		Flipped:    r.Flipped,
		NineSlice:  margins,
	}, nil
}

// 保存按钮特定状态文字覆盖的原始 JSON 字段
type rawLabelOverride struct {
	// 可选的 RGBA 颜色分量
	Color []float32 `json:"color"`
	// 可选的二维偏移
	Offset []float32 `json:"offset"`
}

// 将可选的原始文字覆盖转换为运行时配置
func (r *rawLabelOverride) toOverride() *ButtonLabelStyleOverride {
	if r == nil {
		return nil
	}
	result := &ButtonLabelStyleOverride{}
	if len(r.Color) == 4 {
		value := emath.Vec4FromFloat32s(r.Color, mgl32.Vec4{})
		result.Color = &value
	}
	if len(r.Offset) == 2 {
		value := emath.Vec2FromFloat32s(r.Offset, mgl32.Vec2{})
		result.Offset = &value
	}
	return result
}

// 保存按钮文字基础样式的原始 JSON 字段
type rawLabelStyle struct {
	// 文字内容
	Text string `json:"text"`
	// 字体资源路径
	FontPath string `json:"font_path"`
	// 字体像素字号
	FontSize int `json:"font_size"`
	// RGBA 颜色分量
	Color []float32 `json:"color"`
	// 相对居中位置的二维偏移
	Offset []float32 `json:"offset"`
}

// 将可选的原始文字样式转换为运行时配置
func (r *rawLabelStyle) toStyle() *ButtonLabelStyle {
	if r == nil {
		return nil
	}
	return &ButtonLabelStyle{
		Text:     r.Text,
		FontPath: r.FontPath,
		FontSize: r.FontSize,
		Color:    emath.Vec4FromFloat32s(r.Color, mgl32.Vec4{1.0, 1.0, 1.0, 1.0}),
		Offset:   emath.Vec2FromFloat32s(r.Offset, mgl32.Vec2{}),
	}
}

// 保存按钮预设的原始 JSON 字段
type rawButtonPreset struct {
	// 各交互状态使用的图片配置
	Images struct {
		// 必需的普通状态图片
		Normal rawImageSpec `json:"normal"`
		// 可选的悬停状态图片
		Hover *rawImageSpec `json:"hover"`
		// 可选的按下状态图片
		Pressed *rawImageSpec `json:"pressed"`
		// 可选的禁用状态图片
		Disabled *rawImageSpec `json:"disabled"`
	} `json:"images"`
	// 按钮图片默认继承的九宫格边距
	NineSlice *NineSliceMargins `json:"nine_slice"`
	// 可选的文字基础样式
	Label *rawLabelStyle `json:"label"`
	// 各交互状态对文字基础样式的覆盖
	Overrides struct {
		// 悬停状态文字覆盖
		Hover *rawLabelOverride `json:"hover"`
		// 按下状态文字覆盖
		Pressed *rawLabelOverride `json:"pressed"`
		// 禁用状态文字覆盖
		Disabled *rawLabelOverride `json:"disabled"`
	} `json:"overrides"`
	// 交互事件到声音资源路径的映射
	Sounds map[string]*string `json:"sounds"`
}

// 将原始按钮预设转换为运行时皮肤配置
func (r *rawButtonPreset) toSkin() (*ButtonSkin, error) {
	normal, err := r.Images.Normal.toSpec(r.NineSlice)
	if err != nil {
		return nil, err
	}
	skin := &ButtonSkin{Normal: normal, Sounds: r.Sounds}
	if skin.Hover, err = optionalImage(r.Images.Hover, r.NineSlice); err != nil {
		return nil, err
	}
	if skin.Pressed, err = optionalImage(r.Images.Pressed, r.NineSlice); err != nil {
		return nil, err
	}
	if skin.Disabled, err = optionalImage(r.Images.Disabled, r.NineSlice); err != nil {
		return nil, err
	}
	skin.Label = r.Label.toStyle()
	skin.HoverLabel = r.Overrides.Hover.toOverride()
	skin.PressedLabel = r.Overrides.Pressed.toOverride()
	skin.DisabledLabel = r.Overrides.Disabled.toOverride()
	return skin, nil
}

// 管理从 JSON 文件加载的按钮和图片预设
//
// 当前按字符串键保存解析结果，同名预设在后续加载时会被覆盖
type UIPresetManager struct {
	// 按钮预设表
	buttons map[string]*ButtonSkin
	// 图片预设表
	images map[string]*ImageSpec
}

// 创建空的 UI 预设管理器
func NewUIPresetManager() *UIPresetManager {
	return &UIPresetManager{
		buttons: make(map[string]*ButtonSkin),
		images:  make(map[string]*ImageSpec),
	}
}

// 从 JSON 文件加载按钮预设并合并到现有预设表
func (m *UIPresetManager) LoadButtonPresets(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("load button presets %q: %w", path, err)
	}
	var raw map[string]rawButtonPreset
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse button presets %q: %w", path, err)
	}
	for key, value := range raw {
		skin, err := value.toSkin()
		if err != nil {
			return fmt.Errorf("parse button preset %q: %w", key, err)
		}
		m.buttons[key] = skin
	}
	return nil
}

// 从 JSON 文件加载图片预设并合并到现有预设表
func (m *UIPresetManager) LoadImagePresets(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("load image presets %q: %w", path, err)
	}
	var raw map[string]rawImageSpec
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse image presets %q: %w", path, err)
	}
	for key, value := range raw {
		spec, err := value.toSpec(nil)
		if err != nil {
			return fmt.Errorf("parse image preset %q: %w", key, err)
		}
		m.images[key] = spec
	}
	return nil
}

// 按键查询按钮预设
func (m *UIPresetManager) ButtonPreset(key string) (*ButtonSkin, bool) {
	value, ok := m.buttons[key]
	return value, ok
}

// 按键查询图片预设
func (m *UIPresetManager) ImagePreset(key string) (*ImageSpec, bool) {
	value, ok := m.images[key]
	return value, ok
}

// 返回按字典序排列的全部按钮预设键
func (m *UIPresetManager) ButtonKeys() []string {
	return utils.SortedKeys(m.buttons)
}

// 返回按字典序排列的全部图片预设键
func (m *UIPresetManager) ImageKeys() []string {
	return utils.SortedKeys(m.images)
}

// 使用指定按钮预设创建按钮
func (m *UIPresetManager) NewButton(key string, position, size mgl32.Vec2, audio abstract.IAudioPlayer) (*Button, error) {
	skin, ok := m.ButtonPreset(key)
	if !ok {
		return nil, fmt.Errorf("button preset %q not found", key)
	}
	return NewButton(position, size, skin, audio)
}

// 转换可选图片配置，nil 输入保持为 nil
func optionalImage(raw *rawImageSpec, margins *NineSliceMargins) (*ImageSpec, error) {
	if raw == nil {
		return nil, nil
	}
	return raw.toSpec(margins)
}
