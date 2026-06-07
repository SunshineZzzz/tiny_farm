package system

import (
	eabstract "tiny_farm/engine/abstract"
	ecomponent "tiny_farm/engine/ecs/component"
	edefs "tiny_farm/engine/utils/defs"
	gcomponent "tiny_farm/game/component"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

const (
	// 向左移动动作
	moveLeftAction edefs.ActionID = "move_left"
	// 向右移动动作
	moveRightAction edefs.ActionID = "move_right"
	// 向上移动动作
	moveUpAction edefs.ActionID = "move_up"
	// 向下移动动作
	moveDownAction edefs.ActionID = "move_down"
)

// 玩家控制系统，把玩家方向输入写入 Velocity
//
// 对角输入会归一化，确保斜向移动速度与轴向速度一致
type PlayerControlSystem struct {
	// 查询句柄，用于查询同时包含 PlayerTag 和 Velocity 的实体
	query *donburi.Query
	// 玩家基础移动速度，单位为世界单位每秒
	//
	// 当前是过渡配置，后续应由角色组件、蓝图或玩法配置提供
	speed float32
}

// 创建使用指定世界单位每秒速度的玩家控制系统
func NewPlayerControlSystem(speed float32) *PlayerControlSystem {
	return &PlayerControlSystem{
		query: donburi.NewQuery(filter.Contains(
			gcomponent.PlayerTag,
			ecomponent.Velocity,
		)),
		speed: speed,
	}
}

// 根据当前动作状态覆盖玩家实体速度
func (s *PlayerControlSystem) Update(world donburi.World, actions eabstract.IActionInput) {
	if s == nil || s.query == nil || world == nil || actions == nil {
		return
	}

	direction := mgl32.Vec2{}
	if actions.IsActionDown(moveLeftAction) {
		direction[0]--
	}
	if actions.IsActionDown(moveRightAction) {
		direction[0]++
	}
	if actions.IsActionDown(moveUpAction) {
		direction[1]--
	}
	if actions.IsActionDown(moveDownAction) {
		direction[1]++
	}
	if direction.Len() > 0 {
		direction = direction.Normalize()
	}

	for entry := range s.query.Iter(world) {
		ecomponent.Velocity.Get(entry).Value = direction.Mul(s.speed)
	}
}
