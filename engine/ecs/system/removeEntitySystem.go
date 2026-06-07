package system

import (
	"tiny_farm/engine/ecs/component"

	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

// 实体删除系统，集中删除已经标记的实体
//
// 先收集实体标识再执行删除，避免查询遍历期间修改 ECS 结构
type RemoveEntitySystem struct {
	// 查询句柄，用于查询包含删除标签的实体
	query *donburi.Query
}

// 创建待删除实体清理系统
func NewRemoveEntitySystem() *RemoveEntitySystem {
	return &RemoveEntitySystem{
		query: donburi.NewQuery(filter.Contains(component.NeedRemoveTag)),
	}
}

// 删除当前 World 中全部待删除实体
func (s *RemoveEntitySystem) Update(world donburi.World) {
	if s == nil || s.query == nil || world == nil {
		return
	}

	entities := make([]donburi.Entity, 0)
	for entry := range s.query.Iter(world) {
		entities = append(entities, entry.Entity())
	}
	for _, entity := range entities {
		world.Remove(entity)
	}
}
