package spatial

import (
	"tiny_farm/engine/ecs/component"
	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 碰撞形状类型
type ColliderShapeType int

const (
	// 矩形碰撞
	ColliderShapeRect ColliderShapeType = iota
	// 圆形碰撞
	ColliderShapeCircle
)

// 统一碰撞形状，用于 narrowphase 相交测试
type ColliderShape struct {
	// 碰撞形状类型
	Type ColliderShapeType
	// 矩形碰撞
	Rect emath.Rect
	// 圆形碰撞
	Center mgl32.Vec2
	// 圆形碰撞半径
	Radius float32
}

// 从 ECS 组件构造碰撞形状
func BuildColliderShape(entry *donburi.Entry) (ColliderShape, bool) {
	if entry == nil || !entry.Valid() || !entry.HasComponent(component.Transform) {
		return ColliderShape{}, false
	}
	transform := component.Transform.Get(entry)
	return BuildColliderShapeByTransformPosition(entry, transform.Position)
}

// 根据实体的碰撞器组件，构造实体位于指定 Transform 世界位置时的碰撞形状
func BuildColliderShapeByTransformPosition(entry *donburi.Entry, position mgl32.Vec2) (ColliderShape, bool) {
	if entry == nil || !entry.Valid() {
		return ColliderShape{}, false
	}
	if entry.HasComponent(component.AABBCollider) {
		collider := component.AABBCollider.Get(entry)
		return ColliderShape{
			Type: ColliderShapeRect,
			Rect: AABBBoundsRectByTransformPosition(position, *collider),
		}, true
	}
	if entry.HasComponent(component.CircleCollider) {
		collider := component.CircleCollider.Get(entry)
		return ColliderShape{
			Type:   ColliderShapeCircle,
			Center: CircleCenterByTransformPosition(position, *collider),
			Radius: collider.Radius,
		}, true
	}
	return ColliderShape{}, false
}

// 计算 AABB 碰撞器位于指定 Transform 世界位置时的世界矩形
func AABBBoundsRectByTransformPosition(position mgl32.Vec2, collider component.AABBColliderComponent) emath.Rect {
	center := position.Add(collider.Offset)
	return emath.Rect{
		Position: center.Sub(collider.Size.Mul(0.5)),
		Size:     collider.Size,
	}
}

// 计算圆形碰撞器位于指定 Transform 世界位置时的世界圆心
func CircleCenterByTransformPosition(position mgl32.Vec2, collider component.CircleColliderComponent) mgl32.Vec2 {
	return position.Add(collider.Offset)
}

// 计算圆形碰撞器位于指定 Transform 世界位置时的外接矩形
func CircleBoundsRectByTransformPosition(position mgl32.Vec2, collider component.CircleColliderComponent) emath.Rect {
	radius := mgl32.Vec2{collider.Radius, collider.Radius}
	return emath.Rect{
		Position: CircleCenterByTransformPosition(position, collider).Sub(radius),
		Size:     radius.Mul(2.0),
	}
}

// 判断查询矩形与碰撞形状是否相交
func IntersectsRectQuery(rect emath.Rect, shape ColliderShape) bool {
	if shape.Type == ColliderShapeRect {
		return RectRectOverlap(rect, shape.Rect)
	}
	return RectCircleOverlap(rect, shape.Center, shape.Radius)
}

// 判断查询圆与碰撞形状是否相交
func IntersectsCircleQuery(center mgl32.Vec2, radius float32, shape ColliderShape) bool {
	if shape.Type == ColliderShapeRect {
		return RectCircleOverlap(shape.Rect, center, radius)
	}
	return CircleCircleOverlap(center, radius, shape.Center, shape.Radius)
}

// 判断查询点与碰撞形状是否相交
func IntersectsPointQuery(point mgl32.Vec2, shape ColliderShape) bool {
	if shape.Type == ColliderShapeRect {
		return PointRectOverlap(shape.Rect, point)
	}
	return PointCircleOverlap(shape.Center, shape.Radius, point)
}

// 判断两个矩形是否重叠，仅边界接触不算重叠
func RectRectOverlap(lhs, rhs emath.Rect) bool {
	lhsMax := lhs.Position.Add(lhs.Size)
	rhsMax := rhs.Position.Add(rhs.Size)
	separated := lhsMax.X() <= rhs.Position.X() ||
		rhsMax.X() <= lhs.Position.X() ||
		lhsMax.Y() <= rhs.Position.Y() ||
		rhsMax.Y() <= lhs.Position.Y()
	return !separated
}

// 判断两个圆是否重叠，边界接触也算重叠
func CircleCircleOverlap(lhsCenter mgl32.Vec2, lhsRadius float32, rhsCenter mgl32.Vec2, rhsRadius float32) bool {
	delta := lhsCenter.Sub(rhsCenter)
	radiusSum := lhsRadius + rhsRadius
	return delta.Dot(delta) <= radiusSum*radiusSum
}

// 矩形与圆相交，边界接触也算相交
func RectCircleOverlap(rect emath.Rect, center mgl32.Vec2, radius float32) bool {
	rectMax := rect.Position.Add(rect.Size)
	// 找到矩形上距离圆心最近的点，再判断该点是否落在圆内。
	closest := mgl32.Vec2{
		mgl32.Clamp(center.X(), rect.Position.X(), rectMax.X()),
		mgl32.Clamp(center.Y(), rect.Position.Y(), rectMax.Y()),
	}
	delta := center.Sub(closest)
	return delta.Dot(delta) <= radius*radius
}

// 判断一个点是否位于矩形内部，使用的是半开区间
// [minX, maxX)
// [minY, maxY)
func PointRectOverlap(rect emath.Rect, point mgl32.Vec2) bool {
	rectMax := rect.Position.Add(rect.Size)
	return point.X() >= rect.Position.X() &&
		point.Y() >= rect.Position.Y() &&
		point.X() < rectMax.X() &&
		point.Y() < rectMax.Y()
}

// 点与圆相交，边界接触也算相交
func PointCircleOverlap(center mgl32.Vec2, radius float32, point mgl32.Vec2) bool {
	delta := point.Sub(center)
	return delta.Dot(delta) <= radius*radius
}
