package component

import (
	"tiny_farm/engine/utils/defs"

	"github.com/yohamta/donburi"
)

// 保存实体可响应的音效触发表
type AudioComponent struct {
	// 触发 key 到真实音效资源 key 的映射
	Sounds map[defs.ResourceKey]defs.ResourceKey
}

// 提供给实体创建、查询和组件读写使用的 Donburi 组件类型句柄
var Audio = donburi.NewComponentType[AudioComponent]().SetName("engine.Audio")
