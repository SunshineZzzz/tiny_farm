package world

import (
	"errors"

	"tiny_farm/engine/ecs/component"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 创建当前 ECS 接入阶段用于验证组件读写的演示实体
func CreateDemoEntity(world donburi.World) (donburi.Entity, error) {
	if world == nil {
		return donburi.Null, errors.New("ecs world is nil")
	}

	entity := world.Create(
		component.Transform,
		component.Velocity,
		component.ShapeRender,
	)
	// Entity 只保存实体标识，需要通过所属 World 获取 Entry 后才能读写组件
	entry := world.Entry(entity)

	component.Transform.SetValue(entry, component.TransformComponent{
		Position: mgl32.Vec2{32.0, 32.0},
		Scale:    mgl32.Vec2{1.0, 1.0},
	})
	component.Velocity.SetValue(entry, component.VelocityComponent{
		Value: mgl32.Vec2{24.0, 12.0},
	})
	component.ShapeRender.SetValue(entry, component.ShapeRenderComponent{
		Size:  mgl32.Vec2{24.0, 16.0},
		Color: mgl32.Vec4{0.9, 0.72, 0.32, 1.0},
	})

	return entity, nil
}
