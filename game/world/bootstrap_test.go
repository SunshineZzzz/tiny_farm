package world

import (
	"testing"

	"tiny_farm/engine/ecs/component"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

// 验证演示实体包含当前阶段约定的组件和值
func TestCreateDemoEntity(t *testing.T) {
	world := donburi.NewWorld()

	entity, err := CreateDemoEntity(world)
	if err != nil {
		t.Fatalf("create demo entity: %v", err)
	}

	entry := world.Entry(entity)
	transform := component.Transform.Get(entry)
	velocity := component.Velocity.Get(entry)
	shapeRender := component.ShapeRender.Get(entry)

	if transform.Position != (mgl32.Vec2{32.0, 32.0}) {
		t.Fatalf("unexpected position: %v", transform.Position)
	}
	if transform.Scale != (mgl32.Vec2{1.0, 1.0}) {
		t.Fatalf("unexpected scale: %v", transform.Scale)
	}
	if velocity.Value != (mgl32.Vec2{}) {
		t.Fatalf("unexpected velocity: %v", velocity.Value)
	}
	if shapeRender.Size != (mgl32.Vec2{24.0, 16.0}) {
		t.Fatalf("unexpected shape size: %v", shapeRender.Size)
	}
	if shapeRender.Color != (mgl32.Vec4{0.9, 0.72, 0.32, 1.0}) {
		t.Fatalf("unexpected shape color: %v", shapeRender.Color)
	}
}

// 验证组合查询只返回同时包含三个最小组件的实体
func TestDemoEntityComponentQuery(t *testing.T) {
	world := donburi.NewWorld()
	if _, err := CreateDemoEntity(world); err != nil {
		t.Fatalf("create demo entity: %v", err)
	}
	world.Create(component.Transform)

	query := donburi.NewQuery(filter.Contains(
		component.Transform,
		component.Velocity,
		component.ShapeRender,
	))

	count := 0
	for range query.Iter(world) {
		count++
	}

	if count != 1 {
		t.Fatalf("unexpected matched entity count: %d", count)
	}
}

// 验证缺少 World 时返回明确错误
func TestCreateDemoEntityRejectsNilWorld(t *testing.T) {
	if _, err := CreateDemoEntity(nil); err == nil {
		t.Fatal("expected nil world error")
	}
}
