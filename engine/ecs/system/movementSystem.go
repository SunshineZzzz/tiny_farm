package system

import (
	"tiny_farm/engine/ecs/component"

	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

// 根据实体速度推进世界空间位置
//
// 当前只处理最小移动规则，不包含碰撞、固定时间步、插值和空间索引
type MovementSystem struct {
	// 复用 Transform 和 Velocity 的组合查询，避免每帧重复创建
	query *donburi.Query
}

// 创建只处理 Transform 和 Velocity 组件的移动系统
func NewMovementSystem() *MovementSystem {
	return &MovementSystem{
		query: donburi.NewQuery(filter.Contains(
			component.Transform,
			component.Velocity,
		)),
	}
}

// 使用传入的帧间隔推进所有匹配实体的位置
//
// deltaTime 使用秒为单位，由主循环的相对帧率方案统一提供
func (s *MovementSystem) Update(world donburi.World, deltaTime float64) {
	if s == nil || s.query == nil || world == nil {
		return
	}

	scaledDeltaTime := float32(deltaTime)
	for entry := range s.query.Iter(world) {
		transform := component.Transform.Get(entry)
		velocity := component.Velocity.Get(entry)
		transform.Position = transform.Position.Add(velocity.Value.Mul(scaledDeltaTime))
	}
}
