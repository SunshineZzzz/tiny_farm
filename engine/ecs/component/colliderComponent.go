package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// AABB 碰撞器，适合箱子、树木、石头等矩形阻挡体
type AABBColliderComponent struct {
	// 碰撞盒尺寸，单位为世界坐标
	Size mgl32.Vec2
	// 相对 Transform 位置的中心点偏移
	Offset mgl32.Vec2
}

// 圆形碰撞器，适合玩家脚底或圆形触发范围
type CircleColliderComponent struct {
	// 碰撞圆半径，单位为世界坐标
	Radius float32
	// 相对 Transform 位置的圆心偏移
	Offset mgl32.Vec2
}

var (
	// AABB 碰撞器组件句柄
	AABBCollider = donburi.NewComponentType[AABBColliderComponent]().SetName("engine.AABBCollider")
	// 圆形碰撞器组件句柄
	CircleCollider = donburi.NewComponentType[CircleColliderComponent]().SetName("engine.CircleCollider")
)
