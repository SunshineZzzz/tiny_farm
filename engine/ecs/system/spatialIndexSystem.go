package system

import (
	"tiny_farm/engine/ecs/component"
	"tiny_farm/engine/spatial"

	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

// 空间索引系统，按 dirty 标记刷新动态实体网格
type SpatialIndexSystem struct {
	// 空间索引统一入口
	spatialIndex *spatial.SpatialIndexManager
	// 查询句柄，用于查询需要同步的动态实体
	query *donburi.Query
}

// 创建空间索引同步系统
func NewSpatialIndexSystem(spatialIndex *spatial.SpatialIndexManager) *SpatialIndexSystem {
	return &SpatialIndexSystem{
		spatialIndex: spatialIndex,
		query: donburi.NewQuery(filter.Contains(
			component.SpatialIndexTag,
			component.TransformDirtyTag,
			component.Transform,
		)),
	}
}

// 刷新所有位置发生变化的动态碰撞实体
func (s *SpatialIndexSystem) Update(world donburi.World) {
	if s == nil || s.query == nil || s.spatialIndex == nil || world == nil {
		return
	}
	for entry := range s.query.Iter(world) {
		s.spatialIndex.UpdateColliderEntity(entry.Entity())
		entry.RemoveComponent(component.TransformDirtyTag)
	}
}
