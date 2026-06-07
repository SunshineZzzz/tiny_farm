package component

import "github.com/yohamta/donburi"

var (
	// 玩家标签(组件)，标识实体由玩家方向输入控制
	PlayerTag = donburi.NewTag().SetName("game.Player")
)
