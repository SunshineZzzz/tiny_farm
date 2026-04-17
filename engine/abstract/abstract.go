package abstract

import (
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
