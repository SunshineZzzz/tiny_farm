package core

import (
	"log/slog"
	"runtime"

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
// 当前不会切到绝对时间对齐方案，只优化等待策略本身。
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
	// 允许传给上层的最大 delta time，单位秒。
	maxDeltaTime float64
}

// NewFPS 创建帧率控制器，并记录初始时间戳。
func NewFPS() *FPS {
	fps := &FPS{}
	fps.timeScale = 1.0
	fps.maxDeltaTime = 0.1
	fps.lastFrameTime = sdl.GetTicksNS()
	fps.currentFrameStartTime = fps.lastFrameTime

	slog.Debug("create fps controller", slog.Uint64("lastFrameTime", fps.lastFrameTime))

	return fps
}

// Update 在每帧开始时调用。
//
// 当前实现仍然是相对帧率方案：
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

	f.clampDeltaTime()
	f.lastFrameTime = sdl.GetTicksNS()
}

// limitFrameRate 在当前帧耗时不足目标帧长时补足等待时间。
//
// 当前保留相对帧率方案，但等待策略改成“两段等待”：
// 1. 先用 DelayNS 做粗等待，避免空转吃满 CPU。
// 2. 最后不到 1ms 的尾差不再直接睡满，而是短自旋贴近目标时刻。
func (f *FPS) limitFrameRate(currentDeltaTime float64) {
	if currentDeltaTime < f.targetFrameTime {
		timeToWait := f.targetFrameTime - currentDeltaTime
		f.waitRemainingTime(timeToWait)
		f.deltaTime = float64(sdl.GetTicksNS()-f.lastFrameTime) / 1e9
		return
	}

	f.deltaTime = currentDeltaTime
}

func (f *FPS) waitRemainingTime(timeToWait float64) {
	const spinThresholdNS = uint64(1_000_000)

	// 目标时刻仍然基于“上一帧开始时刻 + 目标帧长”来算。
	targetTime := f.lastFrameTime + uint64(f.targetFrameTime*1e9)
	remainingNS := uint64(timeToWait * 1e9)

	// 剩余时间较长时，先交给 SDL 做一次粗等待，避免空转吃满 CPU。
	if remainingNS > spinThresholdNS {
		sdl.DelayNS(remainingNS - spinThresholdNS)
	}

	// 最后 1ms 不再直接 sleep，改成短自旋去贴近目标时刻，
	// 这样可以减少 DelayNS 过冲带来的稳定性损失。
	for sdl.GetTicksNS() < targetTime {
		runtime.Gosched()
	}
}

// clampDeltaTime 限制传给上层的 delta time，避免偶发卡顿直接冲击逻辑层。
func (f *FPS) clampDeltaTime() {
	if f.maxDeltaTime > 0.0 && f.deltaTime > f.maxDeltaTime {
		slog.Warn(
			"delta time exceeds max limit, clamp applied",
			slog.Float64("rawDeltaTime", f.deltaTime),
			slog.Float64("maxDeltaTime", f.maxDeltaTime),
		)
		f.deltaTime = f.maxDeltaTime
	}
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
		f.targetFrameTime = 0.0
		return
	}

	f.targetFps = targetFps
	if f.targetFps > 0 {
		f.targetFrameTime = 1.0 / float64(f.targetFps)
	} else {
		f.targetFrameTime = 0.0
	}

	slog.Info("set target fps", slog.Int("targetFps", f.targetFps), slog.Float64("targetFrameTime", f.targetFrameTime))
}

// GetTargetFps 返回当前目标帧率。
func (f *FPS) GetTargetFps() int {
	return f.targetFps
}
