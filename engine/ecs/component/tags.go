package component

import (
	"github.com/yohamta/donburi"
)

var (
	// 实体删除标签(组件)，标识实体应在当前更新阶段结束时集中删除
	NeedRemoveTag = donburi.NewTag().SetName("engine.NeedRemove")
	// 动态空间索引标签，标识实体需要注册到动态实体网格
	SpatialIndexTag = donburi.NewTag().SetName("engine.SpatialIndex")
	// 变换脏标签，标识实体位置变化后需要刷新空间索引
	TransformDirtyTag = donburi.NewTag().SetName("engine.TransformDirty")
)
