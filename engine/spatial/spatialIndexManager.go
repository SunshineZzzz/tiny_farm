package spatial

import (
	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 碰撞查询结果，合并静态瓦片和动态碰撞体信息
type CollisionResult struct {
	// 是否命中静态完整阻挡瓦片
	HasStaticCollision bool
	// 动态碰撞体命中列表
	DynamicColliders []donburi.Entity
}

// 静态扫掠方向
type SweepDirection int

const (
	// 向北移动
	SweepDirectionNorth SweepDirection = iota
	// 向南移动
	SweepDirectionSouth
	// 向东移动
	SweepDirectionEast
	// 向西移动
	SweepDirectionWest
)

// 静态瓦片扫掠结果
type SweepResult struct {
	// 裁剪后的碰撞矩形
	Rect emath.Rect
	// 是否命中方向阻挡边界
	HitThinWall bool
	// 是否命中完整阻挡瓦片边界
	HitSolid bool
	// 首个命中边界和扫掠范围
	HitInfo SweepHitInfo
}

// 空间索引管理器，统一管理静态瓦片网格和动态实体网格
type SpatialIndexManager struct {
	// 静态瓦片网格
	staticGrid StaticTileGrid
	// 动态实体网格
	dynamicGrid DynamicEntityGrid
	// 当前场景持有的 ECS World 实例
	world donburi.World
}

// 初始化空间索引
func (m *SpatialIndexManager) Initialize(world donburi.World, mapSize, tileSize Coord, worldBoundsMin, worldBoundsMax, dynamicCellSize mgl32.Vec2) {
	m.world = world
	m.staticGrid.Initialize(mapSize, tileSize)
	m.dynamicGrid.Initialize(worldBoundsMin, worldBoundsMax, dynamicCellSize)
}

// 重置为空状态
func (m *SpatialIndexManager) Reset() {
	m.world = nil
	m.staticGrid.Reset()
	m.dynamicGrid.Reset()
}

// 判断是否已经完成初始化
func (m *SpatialIndexManager) IsInitialized() bool {
	return m.world != nil && m.staticGrid.IsInitialized() && m.dynamicGrid.IsInitialized()
}

// 静态网格访问
func (m *SpatialIndexManager) StaticGrid() *StaticTileGrid {
	return &m.staticGrid
}

// 动态网格访问
func (m *SpatialIndexManager) DynamicGrid() *DynamicEntityGrid {
	return &m.dynamicGrid
}

// 设置瓦片属性标志
func (m *SpatialIndexManager) SetTileType(coord Coord, tileType TileType) {
	m.staticGrid.SetTileType(coord, tileType)
}

// 添加瓦片层实体
func (m *SpatialIndexManager) AddTileEntity(coord Coord, entity donburi.Entity, layer LayerID) {
	m.staticGrid.AddEntity(coord, entity, layer)
}

// 添加世界坐标处的瓦片层实体
func (m *SpatialIndexManager) AddTileEntityAtWorldPos(worldPos mgl32.Vec2, entity donburi.Entity, layer LayerID) {
	m.staticGrid.AddEntityAtWorldPos(worldPos, entity, layer)
}

// 查询世界坐标处是否为完整阻挡
func (m *SpatialIndexManager) IsSolidAt(worldPos mgl32.Vec2) bool {
	return m.staticGrid.IsSolidAtWorldPos(worldPos)
}

// 查询世界坐标对应的瓦片坐标
func (m *SpatialIndexManager) TileCoordAtWorldPos(worldPos mgl32.Vec2) Coord {
	return m.staticGrid.WorldToTileCoord(worldPos)
}

// 查询世界坐标所在瓦片的世界矩形
func (m *SpatialIndexManager) RectAtWorldPos(worldPos mgl32.Vec2) emath.Rect {
	return m.staticGrid.RectAtWorldPos(worldPos)
}

// 查询世界坐标处指定图层上的瓦片实体
func (m *SpatialIndexManager) TileEntityAtWorldPos(worldPos mgl32.Vec2, layer LayerID) (donburi.Entity, bool) {
	return m.staticGrid.EntityAtWorldPos(worldPos, layer)
}

// 添加或刷新动态碰撞实体
func (m *SpatialIndexManager) UpdateColliderEntity(entity donburi.Entity) {
	if m.world == nil || !m.world.Valid(entity) {
		return
	}
	entry := m.world.Entry(entity)
	shape, ok := BuildColliderShape(entry)
	if !ok {
		m.dynamicGrid.RemoveEntity(entity)
		return
	}
	if shape.Type == ColliderShapeRect {
		m.dynamicGrid.AddEntity(entity, shape.Rect)
		return
	}
	m.dynamicGrid.AddCircleEntity(entity, shape.Center, shape.Radius)
}

// 删除动态碰撞实体
func (m *SpatialIndexManager) RemoveColliderEntity(entity donburi.Entity) {
	m.dynamicGrid.RemoveEntity(entity)
}

// 查询矩形覆盖单元内的候选实体，只执行 broadphase
func (m *SpatialIndexManager) QueryColliderCandidates(rect emath.Rect) []donburi.Entity {
	return m.dynamicGrid.QueryEntities(rect)
}

// 查询圆形覆盖单元内的候选实体，只执行 broadphase
func (m *SpatialIndexManager) QueryCircleColliderCandidates(center mgl32.Vec2, radius float32) []donburi.Entity {
	return m.dynamicGrid.QueryCircleEntities(center, radius)
}

// 查询与矩形真实相交的动态碰撞体
func (m *SpatialIndexManager) QueryColliders(rect emath.Rect) []donburi.Entity {
	candidates := m.dynamicGrid.QueryEntities(rect)
	return m.filterOverlaps(candidates, func(shape ColliderShape) bool {
		return IntersectsRectQuery(rect, shape)
	})
}

// 查询与圆形真实相交的动态碰撞体
func (m *SpatialIndexManager) QueryCircleColliders(center mgl32.Vec2, radius float32) []donburi.Entity {
	candidates := m.dynamicGrid.QueryCircleEntities(center, radius)
	return m.filterOverlaps(candidates, func(shape ColliderShape) bool {
		return IntersectsCircleQuery(center, radius, shape)
	})
}

// 查询与点真实相交的动态碰撞体
func (m *SpatialIndexManager) QueryCollidersAt(worldPos mgl32.Vec2) []donburi.Entity {
	candidates := m.dynamicGrid.QueryEntitiesAt(worldPos)
	return m.filterOverlaps(candidates, func(shape ColliderShape) bool {
		return IntersectsPointQuery(worldPos, shape)
	})
}

// 同时查询静态阻挡和动态碰撞体
func (m *SpatialIndexManager) CheckCollision(rect emath.Rect) CollisionResult {
	return CollisionResult{
		HasStaticCollision: m.staticGrid.HasSolidInRect(rect),
		DynamicColliders:   m.QueryColliders(rect),
	}
}

// 沿指定方向扫掠静态瓦片边界并裁剪目标矩形
func (m *SpatialIndexManager) ResolveStaticSweep(startRect, targetRect emath.Rect, direction SweepDirection) SweepResult {
	result := SweepResult{
		Rect:    targetRect,
		HitInfo: newSweepHitInfo(),
	}
	if !m.staticGrid.IsInitialized() {
		return result
	}

	vertical := direction == SweepDirectionNorth || direction == SweepDirectionSouth
	positive := direction == SweepDirectionSouth || direction == SweepDirectionEast
	var hit bool
	var resolvedPosition float32
	if vertical {
		hit, resolvedPosition, result.HitInfo = m.staticGrid.SweepVertical(startRect, targetRect, positive)
	} else {
		hit, resolvedPosition, result.HitInfo = m.staticGrid.SweepHorizontal(startRect, targetRect, positive)
	}
	if !hit {
		return result
	}

	result.HitSolid = result.HitInfo.HitSolid
	result.HitThinWall = !result.HitInfo.HitSolid
	if vertical {
		result.Rect.Position[1] = resolvedPosition
	} else {
		result.Rect.Position[0] = resolvedPosition
	}
	return result
}

// 对 broadphase 返回的候选实体做精确碰撞检测，也就是 narrowphase
func (m *SpatialIndexManager) filterOverlaps(candidates []donburi.Entity, intersects func(ColliderShape) bool) []donburi.Entity {
	if m.world == nil {
		return nil
	}
	overlaps := make([]donburi.Entity, 0, len(candidates))
	for _, entity := range candidates {
		if !m.world.Valid(entity) {
			continue
		}
		shape, ok := BuildColliderShape(m.world.Entry(entity))
		if ok && intersects(shape) {
			overlaps = append(overlaps, entity)
		}
	}
	return overlaps
}
