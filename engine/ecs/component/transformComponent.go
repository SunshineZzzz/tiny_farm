package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 变换组件，保存实体在世界空间中的位置、缩放和旋转
type TransformComponent struct {
	// 世界空间位置
	Position mgl32.Vec2
	// 实体在两个坐标轴上的缩放
	Scale mgl32.Vec2
	// 旋转弧度
	Rotation float32
}

// 提供给实体创建、查询和组件读写使用的 Donburi 组件类型句柄
var Transform = donburi.NewComponentType[TransformComponent]().SetName("engine.Transform")
