package core

import (
	"log/slog"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// FPS 是当前主循环使用的帧率控制器。
//
// 当前版本明确使用“相对帧率”方案：
// 1. 每帧开始先记录当前时间。
// 2. 计算距离上一帧过去了多久。
// 3. 如果还没达到目标帧时长，就补足剩余等待时间。
// 4. 等待结束后，把这一帧的实际耗时作为 delta time 返回。
//
// 这版的目标是先把单主循环跑通，后续如果需要更强的长期稳定性，
// 再考虑切到“绝对时间对齐”的控帧方案。
type FPS struct {
	// 上一帧结束时记录的时间戳，单位纳秒。
	lastFrameTime uint64
	// 当前帧开始时的时间戳，单位纳秒。
	currentFrameStartTime uint64
	// 当前帧的 delta time，单位秒。
	deltaTime float64
	// 时间缩放系数，默认是 1.0。
	timeScale float64
	// 目标帧率，例如 60。
	targetFps int
	// 目标每帧时长，单位秒。
	targetFrameTime float64
}

// NewFPS 创建帧率控制器，并记录初始时间戳。
func NewFPS() *FPS {
	fps := &FPS{}
	fps.timeScale = 1.0
	fps.lastFrameTime = sdl.GetTicksNS()
	fps.currentFrameStartTime = fps.lastFrameTime

	slog.Debug("create fps controller", slog.Uint64("lastFrameTime", fps.lastFrameTime))

	return fps
}

// Update 在每帧开始时调用。
//
// 当前实现是相对帧率方案：
// - 先计算“这一帧距离上一帧过去了多久”
// - 如果没到目标帧长，就继续等待
// - 最后把本帧实际耗时写回 deltaTime
func (f *FPS) Update() {
	f.currentFrameStartTime = sdl.GetTicksNS()
	currentDeltaTime := float64(f.currentFrameStartTime-f.lastFrameTime) / 1e9
	if f.targetFrameTime > 0.0 {
		f.limitFrameRate(currentDeltaTime)
	} else {
		f.deltaTime = currentDeltaTime
	}
	f.lastFrameTime = sdl.GetTicksNS()
}

// limitFrameRate 在当前帧耗时不足目标帧长时补足等待时间。
// 这就是当前版本“相对帧率”方案的核心逻辑。
func (f *FPS) limitFrameRate(currentDeltaTime float64) {
	if currentDeltaTime < f.targetFrameTime {
		timeToWait := f.targetFrameTime - currentDeltaTime
		nsToWait := uint64(timeToWait * 1e9)
		sdl.DelayNS(nsToWait)
		f.deltaTime = float64(sdl.GetTicksNS()-f.lastFrameTime) / 1e9
		return
	}

	f.deltaTime = currentDeltaTime
}

// GetDeltaTime 返回带时间缩放的 delta time。
func (f *FPS) GetDeltaTime() float64 {
	return f.deltaTime * f.timeScale
}

// GetUnscaledDeltaTime 返回未缩放的 delta time。
func (f *FPS) GetUnscaledDeltaTime() float64 {
	return f.deltaTime
}

// SetTimeScale 设置时间缩放系数。
func (f *FPS) SetTimeScale(timeScale float64) {
	f.timeScale = timeScale
}

// GetTimeScale 返回当前时间缩放系数。
func (f *FPS) GetTimeScale() float64 {
	return f.timeScale
}

// SetTargetFps 设置目标帧率。
func (f *FPS) SetTargetFps(targetFps int) {
	if targetFps < 0 {
		slog.Error("target fps must be greater than 0, set to 0")
		f.targetFps = 0
		return
	}

	f.targetFps = targetFps
	if f.targetFps > 0 {
		f.targetFrameTime = 1.0 / float64(f.targetFps)
	}

	slog.Info("set target fps", slog.Int("targetFps", f.targetFps), slog.Float64("targetFrameTime", f.targetFrameTime))
}

// GetTargetFps 返回当前目标帧率。
func (f *FPS) GetTargetFps() int {
	return f.targetFps
}
