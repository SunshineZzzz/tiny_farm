package event

// --- 控制流事件（推荐 trigger<T>() 立即分发） ---
// 这些事件往往会改变主循环控制流/场景栈结构，通常希望尽快触达监听者。
// 注意：即使使用 trigger，同步监听者也可以选择“延迟落地”（例如 SceneManager 记录 pending action，在 update 末尾统一改栈）。

/// @brief 退出事件（推荐 trigger<QuitEvent>()）
type QuitEvent struct{}

// 窗口/渲染相关事件(推荐 trigger<T>() 立即分发)
type WindowResizedEvent struct {
	Width  int32
	Height int32
	// true 表示像素大小(高DPI)，false 表示窗口坐标大小
	PixelSizeChanged bool
}

// --- 动画/音频等反馈事件（通常推荐 enqueue + dispatcher.update()） ---
