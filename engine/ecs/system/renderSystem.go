package system

import (
	"cmp"
	"errors"
	"slices"

	"tiny_farm/engine/ecs/component"
	"tiny_farm/engine/render"
	"tiny_farm/engine/resource"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

// 保存单个精灵经过组件解析后的待排序绘制数据
type spriteRenderItem struct {
	// 跨类别绘制层，数值越小越先绘制
	layer int
	// 同层绘制深度，数值越小越先绘制
	depth float32
	// 本次绘制使用的已加载纹理
	texture *render.Texture
	// 世界空间中的目标矩形
	dstRect mgl32.Vec4
	// 归一化纹理坐标区域
	uvRect mgl32.Vec4
	// 与纹理颜色相乘的颜色调制值
	color mgl32.Vec4
}

// 渲染系统，负责收集、排序并提交正式精灵绘制命令
type RenderSystem struct {
	// 查询句柄，用于查询同时包含 Transform、Sprite 和 Render 组件的实体
	query *donburi.Query
}

// 创建需要 Transform、Sprite 和 Render 组件的渲染系统
func NewRenderSystem() *RenderSystem {
	return &RenderSystem{
		query: donburi.NewQuery(filter.Contains(
			component.Transform,
			component.Sprite,
			component.Render,
		)),
	}
}

// 按 layer 和 depth 排序后提交当前 World 的精灵
func (s *RenderSystem) Render(world donburi.World, resource *resource.ResourceManager, render *render.Renderer) error {
	if s == nil || s.query == nil || world == nil {
		return nil
	}
	if resource == nil || render == nil {
		return errors.New("sprite render dependencies are nil")
	}

	// 创建空切片，用于收集本帧所有精灵绘制项
	items := make([]spriteRenderItem, 0)
	for entry := range s.query.Iter(world) {
		transformCom := component.Transform.Get(entry)
		spriteCom := component.Sprite.Get(entry)
		renderCom := component.Render.Get(entry)
		texture, err := resource.Texture(spriteCom.TextureKey, spriteCom.TexturePath)
		if err != nil {
			return err
		}
		textureSize := texture.Size()
		if textureSize[0] <= 0 || textureSize[1] <= 0 {
			continue
		}

		// 最终尺寸 = 精灵基础尺寸 x 实体缩放
		size := mgl32.Vec2{
			spriteCom.Size[0] * transformCom.Scale[0],
			spriteCom.Size[1] * transformCom.Scale[1],
		}
		// 计算目标矩形左上角
		// Transform.Position 不是必然表示左上角，而是表示精灵 Pivot 所在的世界位置
		// 左上角 = Transform.Position - Pivot x 最终尺寸
		position := transformCom.Position.Sub(mgl32.Vec2{
			spriteCom.Pivot[0] * size[0],
			spriteCom.Pivot[1] * size[1],
		})
		// 创建待绘制项，并添加到本帧列表
		items = append(items, spriteRenderItem{
			layer:   renderCom.Layer,
			depth:   renderCom.Depth,
			texture: texture,
			dstRect: mgl32.Vec4{position[0], position[1], size[0], size[1]},
			uvRect: mgl32.Vec4{
				spriteCom.SourceRect[0] / textureSize[0],
				spriteCom.SourceRect[1] / textureSize[1],
				spriteCom.SourceRect[2] / textureSize[0],
				spriteCom.SourceRect[3] / textureSize[1],
			},
			color: renderCom.Color,
		})
	}

	// 对所有精灵绘制项进行稳定排序，如果两个元素比较结果相等，会保留它们原来的相对顺序
	// 先按 layer 从小到大
	// 再按 depth 从小到大
	// 完全相同则保持原顺序
	slices.SortStableFunc(items, func(left, right spriteRenderItem) int {
		if result := cmp.Compare(left.layer, right.layer); result != 0 {
			return result
		}
		return cmp.Compare(left.depth, right.depth)
	})

	// 提交所有排序后的精灵绘制命令
	for _, item := range items {
		if err := render.DrawWorldTextureColor(item.texture, item.dstRect, item.uvRect, item.color); err != nil {
			return err
		}
	}
	return nil
}
