package spatial

import (
	"tiny_farm/engine/ecs/component"
	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 碰撞解析器，将目标位置裁剪为静态和动态障碍允许的实际位置
//
// 当前按 X、Y 两轴依次解析以支持沿墙滑动，动态碰撞只检查轴向候选位置
type CollisionResolver struct {
	// 当前场景持有的 ECS World
	world donburi.World
	// 静态和动态空间查询入口
	spatialIndex *SpatialIndexManager
}

// 创建使用指定 ECS World 和空间索引的碰撞解析器
func NewCollisionResolver(world donburi.World, spatialIndex *SpatialIndexManager) *CollisionResolver {
	return &CollisionResolver{
		world:        world,
		spatialIndex: spatialIndex,
	}
}

// 解析实体从当前位置到目标位置的可行移动
func (r *CollisionResolver) ResolveMovement(entity donburi.Entity, currentPos, targetPos mgl32.Vec2) mgl32.Vec2 {
	if r == nil || r.world == nil || r.spatialIndex == nil || !r.world.Valid(entity) {
		return currentPos
	}

	entry := r.world.Entry(entity)
	delta := targetPos.Sub(currentPos)
	// AABB 矩形碰撞器
	if entry.HasComponent(component.AABBCollider) {
		collider := *component.AABBCollider.Get(entry)
		return r.resolveMovementForShape(
			// 实体移动前的Transform.Position
			currentPos,
			// 实体本帧期望移动的位移量
			delta,
			// 用于在实体中心位置和矩形左上角之间换算
			collider.Size.Mul(0.5),
			// 矩形碰撞器的偏移量
			collider.Offset,
			// 输入任意 Transform.Position，返回碰撞器在该位置时的 AABB 世界矩形
			func(position mgl32.Vec2) emath.Rect {
				return AABBBoundsRectByTransformPosition(position, collider)
			},
			// 实体位于指定 Transform.Position 时，是否会与其他动态实体发生碰撞
			func(position mgl32.Vec2) bool {
				// 计算实体位于 position 时的矩形碰撞形状
				shape := ColliderShape{
					Type: ColliderShapeRect,
					Rect: AABBBoundsRectByTransformPosition(position, collider),
				}
				// 检查该形状是否与除自身外的动态碰撞器重叠
				return r.anyDynamicOverlap(entity, shape)
			},
		)
	}
	// Circle 碰撞器
	if entry.HasComponent(component.CircleCollider) {
		collider := *component.CircleCollider.Get(entry)
		halfExtent := mgl32.Vec2{collider.Radius, collider.Radius}
		return r.resolveMovementForShape(
			currentPos,
			delta,
			halfExtent,
			collider.Offset,
			func(position mgl32.Vec2) emath.Rect {
				return CircleBoundsRectByTransformPosition(position, collider)
			},
			func(position mgl32.Vec2) bool {
				shape := ColliderShape{
					Type:   ColliderShapeCircle,
					Center: CircleCenterByTransformPosition(position, collider),
					Radius: collider.Radius,
				}
				return r.anyDynamicOverlap(entity, shape)
			},
		)
	}
	return targetPos
}

// 判断实体是否可以直接放置到目标位置
func (r *CollisionResolver) CanMoveTo(entity donburi.Entity, targetPos mgl32.Vec2) bool {
	if r == nil || r.world == nil || r.spatialIndex == nil || !r.world.Valid(entity) {
		return false
	}
	shape, ok := BuildColliderShapeByTransformPosition(r.world.Entry(entity), targetPos)
	if !ok {
		return true
	}
	if r.spatialIndex.staticGrid.HasSolidInRect(shapeBounds(shape)) {
		return false
	}
	return !r.anyDynamicOverlap(entity, shape)
}

// 使用实体当前 Transform 将碰撞器同步到动态空间索引
func (r *CollisionResolver) SyncDynamicCollider(entity donburi.Entity) {
	if r == nil || r.spatialIndex == nil {
		return
	}
	r.spatialIndex.UpdateColliderEntity(entity)
}

// 按 X、Y 两轴依次解析同一碰撞形状的移动
// currentPos：移动前的 Transform.Position
// delta：本帧期望移动量，即 targetPos - currentPos
// halfExtent：碰撞形状外接矩形尺寸的一半
// offset：碰撞器中心相对 Transform.Position 的偏移
// boundsByTransformPosition：计算指定 Transform 位置处的碰撞外接矩形
// hasDynamicOverlap：检测指定 Transform 位置是否撞到动态实体
// 返回值：经过静态和动态碰撞限制后，实体实际可以到达的 Transform.Position
func (r *CollisionResolver) resolveMovementForShape(
	currentPos mgl32.Vec2,
	delta mgl32.Vec2,
	halfExtent mgl32.Vec2,
	offset mgl32.Vec2,
	boundsByTransformPosition func(mgl32.Vec2) emath.Rect,
	hasDynamicOverlap func(mgl32.Vec2) bool,
) mgl32.Vec2 {
	resolvedPos := currentPos
	// 单轴解析函数
	resolveAxis := func(axisDelta float32, xAxis bool) {
		if axisDelta == 0.0 {
			return
		}

		startRect := boundsByTransformPosition(resolvedPos)
		targetRect := startRect
		direction := SweepDirectionNorth
		if xAxis {
			targetRect.Position[0] += axisDelta
			if axisDelta > 0 {
				direction = SweepDirectionEast
			} else {
				direction = SweepDirectionWest
			}
		} else {
			targetRect.Position[1] += axisDelta
			if axisDelta > 0 {
				direction = SweepDirectionSouth
			}
		}

		// 检测碰撞矩形从 startRect 移动到 targetRect 的途中是否撞到静态瓦片
		// sweep.Rect.Position 是碰撞外接矩形的左上角，但函数最终需要返回 Transform.Position，所以需要换算
		// 两者关系：
		// 碰撞器中心 = Transform.Position + Offset
		// 矩形左上角 = 碰撞器中心 - HalfExtent
		// 因此：
		// 矩形左上角 = Transform.Position + Offset - HalfExtent
		sweep := r.spatialIndex.ResolveStaticSweep(startRect, targetRect, direction)
		candidatePos := resolvedPos
		if xAxis {
			candidatePos[0] = sweep.Rect.Position.X() + halfExtent.X() - offset.X()
		} else {
			candidatePos[1] = sweep.Rect.Position.Y() + halfExtent.Y() - offset.Y()
		}

		// 兜底
		// 实体起始位置已经在 SOLID 里面
		// 移动距离小于 gridEpsilon，扫掠被跳过
		if r.spatialIndex.staticGrid.HasSolidInRect(boundsByTransformPosition(candidatePos)) {
			return
		}
		// 检查碰撞矩形是否与动态碰撞器重叠
		if hasDynamicOverlap(candidatePos) {
			return
		}
		// 更新最终位置
		resolvedPos = candidatePos
	}

	// 先解析 X 轴
	resolveAxis(delta.X(), true)
	// 再解析 Y 轴
	resolveAxis(delta.Y(), false)
	return resolvedPos
}

// 判断查询形状是否与除自身外的任意动态碰撞器重叠
func (r *CollisionResolver) anyDynamicOverlap(self donburi.Entity, query ColliderShape) bool {
	if r == nil || r.world == nil || r.spatialIndex == nil {
		return false
	}

	// 粗略查找附近实体
	var candidates []donburi.Entity
	if query.Type == ColliderShapeRect {
		candidates = r.spatialIndex.QueryColliderCandidates(query.Rect)
	} else {
		candidates = r.spatialIndex.QueryCircleColliderCandidates(query.Center, query.Radius)
	}
	// 精确检查每个候选实体是否与查询形状重叠
	for _, candidate := range candidates {
		if candidate == self || !r.world.Valid(candidate) {
			continue
		}
		// 获取候选实体的真实碰撞形状
		shape, ok := BuildColliderShape(r.world.Entry(candidate))
		if !ok {
			continue
		}
		// 进行精确相交检测
		if query.Type == ColliderShapeRect && IntersectsRectQuery(query.Rect, shape) {
			return true
		}
		if query.Type == ColliderShapeCircle && IntersectsCircleQuery(query.Center, query.Radius, shape) {
			return true
		}
	}
	return false
}

// 返回碰撞形状用于静态瓦片查询的外接矩形
func shapeBounds(shape ColliderShape) emath.Rect {
	if shape.Type == ColliderShapeRect {
		return shape.Rect
	}
	radius := mgl32.Vec2{shape.Radius, shape.Radius}
	return emath.Rect{
		Position: shape.Center.Sub(radius),
		Size:     radius.Mul(2),
	}
}
