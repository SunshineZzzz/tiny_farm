package system

import (
	"tiny_farm/engine/ecs/component"
	"tiny_farm/engine/spatial"

	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

// 移动系统配置
type MovementSystemOptions struct {
	// 是否在每个实体移动前后立即刷新动态空间索引
	SyncDynamicGridDuringMovement bool
}

// 移动系统，根据实体速度和可选碰撞解析器推进世界空间位置
//
// 当前使用相对帧率提供的 deltaTime，不包含固定时间步和渲染插值
type MovementSystem struct {
	// 查询句柄，用于查询同时包含 Transform 和 Velocity 的实体
	query *donburi.Query
	// 可选碰撞解析器，为空时直接应用目标位置
	collisionResolver *spatial.CollisionResolver
	// 移动和空间索引同步配置
	options MovementSystemOptions
}

// 创建移动系统，可选碰撞解析器默认启用移动前后即时索引同步
func NewMovementSystem(resolvers ...*spatial.CollisionResolver) *MovementSystem {
	var resolver *spatial.CollisionResolver
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	return NewMovementSystemWithOptions(resolver, MovementSystemOptions{
		SyncDynamicGridDuringMovement: true,
	})
}

// 使用指定碰撞解析器和配置创建移动系统
func NewMovementSystemWithOptions(resolver *spatial.CollisionResolver, options MovementSystemOptions) *MovementSystem {
	return &MovementSystem{
		query: donburi.NewQuery(filter.Contains(
			component.Transform,
			component.Velocity,
		)),
		collisionResolver: resolver,
		options:           options,
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
	movedEntities := make([]donburi.Entity, 0)
	for entry := range s.query.Iter(world) {
		transform := component.Transform.Get(entry)
		velocity := component.Velocity.Get(entry)
		currentPos := transform.Position
		targetPos := currentPos.Add(velocity.Value.Mul(scaledDeltaTime))

		hasCollider := entry.HasComponent(component.AABBCollider) || entry.HasComponent(component.CircleCollider)
		if s.collisionResolver != nil && hasCollider {
			syncDuringMovement := s.options.SyncDynamicGridDuringMovement &&
				entry.HasComponent(component.SpatialIndexTag)
			// 移动前同步旧位置
			if syncDuringMovement {
				s.collisionResolver.SyncDynamicCollider(entry.Entity())
			}
			// 解决碰撞并更新位置
			transform.Position = s.collisionResolver.ResolveMovement(entry.Entity(), currentPos, targetPos)
			// 移动后同步新位置
			if syncDuringMovement {
				s.collisionResolver.SyncDynamicCollider(entry.Entity())
			}
		} else {
			transform.Position = targetPos
		}
		if transform.Position != currentPos {
			movedEntities = append(movedEntities, entry.Entity())
		}
	}

	for _, entity := range movedEntities {
		if !world.Valid(entity) {
			continue
		}
		entry := world.Entry(entity)
		if !entry.HasComponent(component.TransformDirtyTag) {
			entry.AddComponent(component.TransformDirtyTag)
		}
	}
}
