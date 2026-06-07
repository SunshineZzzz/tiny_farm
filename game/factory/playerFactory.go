package factory

import (
	"errors"

	"tiny_farm/engine/ecs/component"
	"tiny_farm/engine/resource"
	gamecomponent "tiny_farm/game/component"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 创建具备输入、移动和精灵渲染能力的玩家实体
func CreatePlayer(world donburi.World) (donburi.Entity, error) {
	if world == nil {
		return donburi.Null, errors.New("ecs world is nil")
	}

	entity := world.Create(
		gamecomponent.PlayerTag,
		component.Transform,
		component.Velocity,
		component.Sprite,
		component.Render,
	)
	// Entity 只保存实体标识，需要通过所属 World 获取 Entry 后才能读写组件
	entry := world.Entry(entity)

	component.Transform.SetValue(entry, component.TransformComponent{
		Position: mgl32.Vec2{32.0, 32.0},
		Scale:    mgl32.Vec2{1.0, 1.0},
	})
	component.Velocity.SetValue(entry, component.VelocityComponent{})
	component.Sprite.SetValue(entry, component.SpriteComponent{
		TextureKey:  resource.ResourceKey("Button Normal.png"),
		TexturePath: "assets/tests/Button Normal.png",
		SourceRect:  mgl32.Vec4{0.0, 0.0, 24.0, 16.0},
		Size:        mgl32.Vec2{24.0, 16.0},
		Pivot:       mgl32.Vec2{0.5, 0.5},
	})
	component.Render.SetValue(entry, component.RenderComponent{
		Layer: component.MainRenderLayer,
		Depth: 32.0,
		Color: mgl32.Vec4{0.9, 0.72, 0.32, 1.0},
	})

	return entity, nil
}
