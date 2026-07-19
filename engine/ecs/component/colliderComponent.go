package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// AABB 碰撞器，适合箱子、树木、石头等矩形阻挡体
type AABBColliderComponent struct {
	// 碰撞盒尺寸，单位为世界坐标
	Size mgl32.Vec2
	// 从 Transform.Position 到碰撞器中心的偏移量
	Offset mgl32.Vec2
}

// 圆形碰撞器，适合玩家脚底或圆形触发范围
type CircleColliderComponent struct {
	// 碰撞圆半径，单位为世界坐标
	Radius float32
	// 从 Transform.Position 到碰撞器中心的偏移量
	Offset mgl32.Vec2
}

var (
	// AABB 碰撞器组件句柄
	AABBCollider = donburi.NewComponentType[AABBColliderComponent]().SetName("engine.AABBCollider")
	// 圆形碰撞器组件句柄
	CircleCollider = donburi.NewComponentType[CircleColliderComponent]().SetName("engine.CircleCollider")
)
