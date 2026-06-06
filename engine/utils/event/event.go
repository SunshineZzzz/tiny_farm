package event

import (
	"tiny_farm/engine/utils/defs"
)

// --- 控制流事件（推荐 trigger<T>() 立即分发） ---
// 这些事件往往会改变主循环控制流/场景栈结构，通常希望尽快触达监听者。
// 注意：即使使用 trigger，同步监听者也可以选择“延迟落地”（例如 SceneManager 记录 pending action，在 update 末尾统一改栈）。

// 退出事件，推荐使用 trigger<QuitEvent>() 立即分发
type QuitEvent struct{}

// 窗口/渲染相关事件(推荐 trigger<T>() 立即分发)
type WindowResizedEvent struct {
	Width  int32
	Height int32
	// true 表示像素大小(高DPI)，false 表示窗口坐标大小
	PixelSizeChanged bool
}

// 字体资源卸载事件，推荐使用 trigger<T>() 立即分发
type FontUnloadedEvent struct {
	Key       defs.ResourceKey
	PixelSize int
}

// 字体资源清空事件，推荐使用 trigger<T>() 立即分发
type FontsClearedEvent struct{}

// --- 动画/音频等反馈事件（通常推荐 enqueue + dispatcher.update()） ---
