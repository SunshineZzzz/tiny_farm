package abstract

import (
	"tiny_farm/engine/utils/defs"

	"github.com/go-gl/mathgl/mgl32"
)

type GameStateType int

const (
	// 标题界面
	GameStateTitle GameStateType = iota
	// 游戏进行中
	GameStatePlaying
	// 游戏暂停
	GameStatePaused
	// 游戏结束
	GameStateGameOver
	// 关卡过关界面
	GameStateLevelClear
)

// 游戏状态接口
type IGameState interface {
	// 获取当前游戏状态
	GetState() GameStateType
	// 设置当前游戏状态
	SetState(GameStateType)
	// 获取窗口大小
	GetWindowSize() mgl32.Vec2
	// 设置窗口大小
	SetWindowSize(mgl32.Vec2)
	// 获取逻辑分辨率
	GetLogicalSize() mgl32.Vec2
	// 设置逻辑分辨率
	SetLogicalSize(mgl32.Vec2)
	// 判断是否在标题界面
	IsInTitle() bool
	// 判断是否在游戏进行中
	IsPlaying() bool
	// 判断是否在游戏暂停
	IsPaused() bool
	// 判断是否在游戏结束
	IsGameOver() bool
	// 判断是否在关卡过关界面
	IsLevelClear() bool
}

// 资源管理器接口
type IResourceManager interface {
	// 加载字体资源，如果命中缓存则直接返回已有字体
	LoadFont(key defs.ResourceKey, pixelSize int, paths ...string) (IFont, error)
}

// 字符光栅化后写入atlas的缓存条目接口
type IFontGlyph interface {
	// 返回 glyph 所在纹理对象
	GlyphTexture() any
	// 返回 glyph 像素尺寸
	GlyphSize() mgl32.Vec2
	// 返回 glyph 相对基线原点的偏移
	GlyphBearing() mgl32.Vec2
	// 返回绘制后光标前进距离，单位像素
	GlyphAdvance() float32
	// 返回 glyph 在 atlas 纹理中的 UV 范围
	GlyphUVRect() mgl32.Vec4
}

// 单个字体资源实例接口
type IFont interface {
	// 返回字体像素字号
	PixelSize() int
	// 返回字体上升高度，单位像素
	Ascender() float32
	// 返回字体下降深度，单位像素
	Descender() float32
	// 返回字体默认行高，单位像素
	LineHeight() float32
	// 按需获取 rune 对应的 glyph
	TextGlyph(rune) (IFontGlyph, error)
}
