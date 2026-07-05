package abstract

import (
	"tiny_farm/engine/utils/defs"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/gopxl/beep/v2"
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

// 持有已解码音频数据的稳定句柄接口
type IAudioBufferHandle interface {
	// 返回音频采样格式
	Format() beep.Format
	// 返回一个从头播放到结尾的新音频流
	Streamer() (beep.StreamSeeker, bool)
}

// opengl纹理接口
type IOpenGLTexture interface {
	// 返回底层 OpenGL texture 句柄
	ID() uint32
	// 返回纹理像素尺寸
	Size() mgl32.Vec2
	// 释放纹理资源
	Close()
	// 更新纹理指定区域的 RGBA 像素，x 和 y 使用左上原点语义
	//
	// - x, y：要写入的区域左上角，按项目里的“左上角为原点”语义
	// - width, height：要更新的区域大小
	// - pixels：RGBA 字节数组，长度必须是 width * height * 4
	UpdateRGBA(int32, int32, int32, int32, []byte) error
}

// 纹理接口
type ITexture interface {
	// 返回纹理像素尺寸
	Size() mgl32.Vec2
	// 释放纹理资源
	Close()
	// 更新纹理指定区域的 RGBA 像素，x 和 y 使用左上原点语义
	//
	// - x, y：要写入的区域左上角，按项目里的“左上角为原点”语义
	// - width, height：要更新的区域大小
	// - pixels：RGBA 字节数组，长度必须是 width * height * 4
	UpdateRGBA(int32, int32, int32, int32, []byte) error
	// 返回 opengl 纹理句柄
	OpenGLTexture() IOpenGLTexture
}

// 资源管理器接口
type IResourceManager interface {
	// 加载纹理资源，如果命中缓存则直接返回已有纹理
	LoadTexture(key defs.ResourceKey, paths ...string) (ITexture, error)
	// 加载字体资源，如果命中缓存则直接返回已有字体
	LoadFont(defs.ResourceKey, int, ...string) (IFont, error)
	// 加载音效资源，如果命中缓存则直接返回已有 buffer
	LoadSound(defs.ResourceKey, ...string) (IAudioBufferHandle, error)
	// 加载音乐资源，如果命中缓存则直接返回已有 buffer
	LoadMusic(defs.ResourceKey, ...string) (IAudioBufferHandle, error)
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
	// 按需获取 glyph index 对应的 glyph
	TextGlyphByIndex(uint32) (IFontGlyph, error)
	// 返回底层 shaping 字体对象，由渲染层按具体 shaping 库解释
	TextShapingFont() any
}

// 动作输入接口
type IActionInput interface {
	// 返回动作当前是否处于按下或持续按下状态
	IsActionDown(defs.ActionID) bool
	// 返回动作是否在本帧刚按下
	IsActionPressed(defs.ActionID) bool
	// 返回动作是否在本帧刚释放
	IsActionReleased(defs.ActionID) bool
	// 返回游戏逻辑坐标系中的鼠标位置
	LogicalMousePosition() mgl32.Vec2
	// 返回指定动作状态的回调注册入口
	//
	// Inactive 不是可触发阶段；传入 Inactive 时会回退到 Pressed，避免调用方绑定到无效列表
	OnAction(actionID defs.ActionID, state defs.ActionState) *defs.ActionSink
}

// 音效播放器接口
type IAudioPlayer interface {
	// 播放全局音效
	PlaySound(defs.ResourceKey, ...string) error
	// 播放带 2D 空间参数的音效
	PlaySound2D(defs.ResourceKey, mgl32.Vec2, mgl32.Vec2, ...string) error
}

// 相机接口
type ICamera interface {
	// 返回当前位置
	Position() mgl32.Vec2
}

// 渲染器接口
type IRenderer interface {
	// 绘制 UI 逻辑坐标系下的纯色矩形
	DrawUIRect(mgl32.Vec4, mgl32.Vec4) error
	// 使用纹理像素源矩形和单色调制绘制 UI 贴图
	//
	// srcRect 格式为 {x, y, width, height}，flipped 为 true 时水平翻转采样结果
	DrawUITextureSourceRectColor(ITexture, mgl32.Vec4, mgl32.Vec4, mgl32.Vec4, bool) error
}

// 文本渲染可选参数接口
type ITextArg interface {
	// 保证传参合法而已，不需要具体实现
	Dummy()
}

// 确保FontPath实现iTextArg接口
var _ ITextArg = defs.FontPath("")

// 确保TextRenderParams实现iTextArg接口
var _ ITextArg = (*defs.TextRenderParams)(nil)

// 确保TextStyleKey实现iTextArg接口
var _ ITextArg = defs.TextStyleKey("")

// 确保TextRenderOverrides实现iTextArg接口
var _ ITextArg = (*defs.TextRenderOverrides)(nil)

// 文本渲染器接口
type ITextRenderer interface {
	// 返回样式或布局配置变更版本
	LayoutRevision() uint64
	// 测量已加载字体下的文本包围尺寸
	MeasureText(text string, fontKey defs.ResourceKey, pixelSize int, otherArgs ...ITextArg) (mgl32.Vec2, error)
	// 绘制 UI 逻辑坐标系文本
	DrawUIText(text string, fontKey defs.ResourceKey, pixelSize int, position mgl32.Vec2, otherArgs ...ITextArg) error
}
