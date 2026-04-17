package core

import (
	"log/slog"

	"tiny_farm/engine/abstract"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

/**
 * @brief 管理和查询游戏的全局宏观状态。
 *
 * 提供一个中心点来确定游戏当前处于哪个主要模式，
 * 以便其他系统（输入、渲染、更新等）可以相应地调整其行为。
 */
type GameState struct {
	// SDL窗口，用于获取窗口大小
	sdlWindow *sdl.Window
	// 当前游戏状态
	currentState abstract.GameStateType
	// 当前逻辑分辨率
	logicalSize mgl32.Vec2
}

// 确保 GameState 实现 IGameState 接口
var _ abstract.IGameState = (*GameState)(nil)

// 构造函数，初始化游戏状态。
func NewGameState(window *sdl.Window, initialState abstract.GameStateType) *GameState {
	if window == nil {
		slog.Error("window is nil")
		return nil
	}

	var width, height int32
	sdl.GetWindowSize(window, &width, &height)
	slog.Debug("new game state")
	return &GameState{
		sdlWindow:    window,
		currentState: initialState,
		logicalSize:  mgl32.Vec2{max(1.0, float32(width)), max(1.0, float32(height))},
	}
}

// 获取当前游戏状态
func (gs *GameState) GetState() abstract.GameStateType {
	return gs.currentState
}

// 设置当前游戏状态
func (gs *GameState) SetState(newState abstract.GameStateType) {
	if gs.currentState == newState {
		return
	}
	gs.currentState = newState
}

// 获取窗口大小
func (gs *GameState) GetWindowSize() mgl32.Vec2 {
	var w, h int32
	sdl.GetWindowSize(gs.sdlWindow, &w, &h)
	return mgl32.Vec2{float32(w), float32(h)}
}

// 设置窗口大小
func (gs *GameState) SetWindowSize(size mgl32.Vec2) {
	sdl.SetWindowSize(gs.sdlWindow, int32(size.X()), int32(size.Y()))
}

// 获取逻辑分辨率
func (gs *GameState) GetLogicalSize() mgl32.Vec2 {
	return gs.logicalSize
}

// 设置逻辑分辨率
func (gs *GameState) SetLogicalSize(size mgl32.Vec2) {
	gs.logicalSize = size
}

// 判断是否在标题界面
func (gs *GameState) IsInTitle() bool {
	return gs.currentState == abstract.GameStateTitle
}

// 判断是否在游戏进行中
func (gs *GameState) IsPlaying() bool {
	return gs.currentState == abstract.GameStatePlaying
}

// 判断是否在游戏暂停
func (gs *GameState) IsPaused() bool {
	return gs.currentState == abstract.GameStatePaused
}

// 判断是否在游戏结束
func (gs *GameState) IsGameOver() bool {
	return gs.currentState == abstract.GameStateGameOver
}

// 判断是否在关卡过关界面
func (gs *GameState) IsLevelClear() bool {
	return gs.currentState == abstract.GameStateLevelClear
}
