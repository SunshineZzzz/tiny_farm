package component

import (
	"github.com/yohamta/donburi"
)

var (
	// 实体删除标签(组件)，标识实体应在当前更新阶段结束时集中删除
	NeedRemoveTag = donburi.NewTag().SetName("engine.NeedRemove")
)
