package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 速度组件，保存实体每秒在世界空间中的移动量
type VelocityComponent struct {
	// 每秒移动速度
	Value mgl32.Vec2
}

// 提供给实体创建、查询和组件读写使用的 Donburi 组件类型句柄
var Velocity = donburi.NewComponentType[VelocityComponent]().SetName("engine.Velocity")
