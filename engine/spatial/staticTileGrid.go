package spatial

import (
	"math"

	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 单个瓦片单元的数据，保存瓦片属性和按图层分桶的实体
type TileCellData struct {
	// 瓦片属性位掩码，可以组合多个
	tType TileType
	// 该格子上挂载的实体，按图层保存，同一个图层只能保存一个实体
	entities map[LayerID]donburi.Entity
	// 记录图层的插入顺序，用于遍历时保持稳定
	layers []LayerID
}

// 添加或替换指定图层上的实体
func (c *TileCellData) AddEntity(entity donburi.Entity, layer LayerID) {
	if c.entities == nil {
		c.entities = make(map[LayerID]donburi.Entity)
	}
	if _, ok := c.entities[layer]; !ok {
		c.layers = append(c.layers, layer)
	}
	c.entities[layer] = entity
}

// 删除指定实体
func (c *TileCellData) RemoveEntity(entity donburi.Entity) {
	for layer, current := range c.entities {
		if current == entity {
			delete(c.entities, layer)
			c.removeLayer(layer)
			return
		}
	}
}

// 获取指定图层上的实体
func (c *TileCellData) Entity(layer LayerID) (donburi.Entity, bool) {
	if c.entities == nil {
		return donburi.Null, false
	}
	entity, ok := c.entities[layer]
	return entity, ok
}

// 从稳定遍历顺序中删除指定图层
func (c *TileCellData) removeLayer(layer LayerID) {
	for i, current := range c.layers {
		if current == layer {
			c.layers = append(c.layers[:i], c.layers[i+1:]...)
			return
		}
	}
}

// 静态瓦片网格，用于查询地图瓦片属性和瓦片层实体
type StaticTileGrid struct {
	// 地图有多少列、多少行瓦片，不是像素大小
	mapSize Coord
	// 瓦片大小，单位为像素
	tileSize Coord
	// 保存地图上所有瓦片的数据，虽然地图是二维的，但这里用一维切片存储：
	// 二维坐标 (x,y) => 一维索引 = y * 地图宽度 + x
	cells []TileCellData
}

// 静态扫掠命中的瓦片边界信息，用于调试和结果分析
type SweepHitInfo struct {
	// 是否命中SOLID瓦片
	HitSolid bool
	// 表示命中了哪一条水平边界
	// 行 0     [ ][ ][ ]
	// 边界行 1 ─────────
	// 行 1     [ ][ ][ ]
	BoundaryRow int
	// 表示命中了哪一条垂直边界
	// 列 0 │ 列 1
	// [ ]  │ [ ]
	BoundaryCol int
	// 角色横向移动时，记录角色高度覆盖了哪些行
	RowStart int
	RowEnd   int
	// 角色纵向移动时，记录角色宽度覆盖了哪些列
	ColStart int
	ColEnd   int
}

// 创建尚未命中任何瓦片边界的扫掠信息
func newSweepHitInfo() SweepHitInfo {
	return SweepHitInfo{
		BoundaryRow: -1,
		BoundaryCol: -1,
		RowStart:    -1,
		RowEnd:      -1,
		ColStart:    -1,
		ColEnd:      -1,
	}
}

// 瓦片边界两侧的综合阻挡信息
// 它与 SweepHitInfo 的区别是：
// edgeBlockInfo：底层检查一条边界时使用的临时结果
// SweepHitInfo：sweep 最终返回给上层的命中和调试信息
type edgeBlockInfo struct {
	// 表示这条边界能不能通过，无论是薄墙还是完整 SOLID，都可能使它变成 true
	blocked bool
	// 表示造成阻挡的瓦片是不是完整 SOLID
	hasSolid bool
}

// 初始化静态网格
func (g *StaticTileGrid) Initialize(mapSize, tileSize Coord) {
	g.mapSize = Coord{X: max(mapSize.X, 0.0), Y: max(mapSize.Y, 0.0)}
	g.tileSize = Coord{X: max(tileSize.X, 1.0), Y: max(tileSize.Y, 1.0)}
	g.cells = make([]TileCellData, g.mapSize.X*g.mapSize.Y)
}

// 重置为空状态
func (g *StaticTileGrid) Reset() {
	g.mapSize = Coord{}
	g.tileSize = Coord{X: 1.0, Y: 1.0}
	g.cells = nil
}

// 判断网格是否已经完成初始化
func (g *StaticTileGrid) IsInitialized() bool {
	return g.mapSize.X > 0 && g.mapSize.Y > 0 && g.tileSize.X > 0 && g.tileSize.Y > 0 && len(g.cells) == g.mapSize.X*g.mapSize.Y
}

// 设置瓦片属性标志
func (g *StaticTileGrid) SetTileType(coord Coord, tileType TileType) {
	if !g.validCoord(coord) {
		return
	}
	g.cells[g.index(coord)].tType |= tileType
}

// 清除瓦片属性标志
func (g *StaticTileGrid) ClearTileType(coord Coord, mask TileType) {
	if !g.validCoord(coord) {
		return
	}
	// Go 中 &^ 叫“位清除”运算：
	// 左边有、mask 也有 → 清除
	// 左边有、mask 没有 → 保留
	g.cells[g.index(coord)].tType &^= mask
}

// 获取瓦片属性
func (g *StaticTileGrid) TileType(coord Coord) TileType {
	if !g.validCoord(coord) {
		return TileNormal
	}
	return g.cells[g.index(coord)].tType
}

// 添加瓦片层实体
func (g *StaticTileGrid) AddEntity(coord Coord, entity donburi.Entity, layer LayerID) {
	if !g.validCoord(coord) {
		return
	}
	g.cells[g.index(coord)].AddEntity(entity, layer)
}

// 添加世界坐标处的瓦片层实体
func (g *StaticTileGrid) AddEntityAtWorldPos(worldPos mgl32.Vec2, entity donburi.Entity, layer LayerID) {
	g.AddEntity(g.WorldToTileCoord(worldPos), entity, layer)
}

// 删除瓦片层实体
func (g *StaticTileGrid) RemoveEntity(coord Coord, entity donburi.Entity) {
	if !g.validCoord(coord) {
		return
	}
	g.cells[g.index(coord)].RemoveEntity(entity)
}

// 获取指定图层上的瓦片实体
func (g *StaticTileGrid) Entity(coord Coord, layer LayerID) (donburi.Entity, bool) {
	if !g.validCoord(coord) {
		return donburi.Null, false
	}
	return g.cells[g.index(coord)].Entity(layer)
}

// 获取世界坐标处指定图层上的瓦片实体
func (g *StaticTileGrid) EntityAtWorldPos(worldPos mgl32.Vec2, layer LayerID) (donburi.Entity, bool) {
	return g.Entity(g.WorldToTileCoord(worldPos), layer)
}

// 判断瓦片是否为完整阻挡
func (g *StaticTileGrid) IsSolid(coord Coord) bool {
	return HasAllTileFlags(g.TileType(coord), TileSolidFlag)
}

// 判断世界坐标处瓦片是否为完整阻挡
func (g *StaticTileGrid) IsSolidAtWorldPos(worldPos mgl32.Vec2) bool {
	return g.IsSolid(g.WorldToTileCoord(worldPos))
}

// 判断矩形范围内是否包含完整阻挡瓦片
func (g *StaticTileGrid) HasSolidInRect(rect emath.Rect) bool {
	minCoord, maxCoord, ok := g.TileRangeForRect(rect)
	if !ok {
		return false
	}
	for y := minCoord.Y; y <= maxCoord.Y; y++ {
		for x := minCoord.X; x <= maxCoord.X; x++ {
			if g.IsSolid(Coord{X: x, Y: y}) {
				return true
			}
		}
	}
	return false
}

// 计算矩形覆盖的瓦片范围，按半开区间处理右下边界
func (g *StaticTileGrid) TileRangeForRect(rect emath.Rect) (Coord, Coord, bool) {
	if !g.IsInitialized() {
		return Coord{}, Coord{}, false
	}
	minCoord := g.WorldToTileCoord(rect.Position)
	maxPos := rect.Position.Add(rect.Size).Sub(mgl32.Vec2{gridEpsilon, gridEpsilon})
	maxPos = emath.Mgl32Vec2Max(maxPos, rect.Position)
	maxCoord := g.WorldToTileCoord(maxPos)
	minCoord = clampCoord(minCoord, Coord{}, Coord{X: g.mapSize.X - 1.0, Y: g.mapSize.Y - 1.0})
	maxCoord = clampCoord(maxCoord, Coord{}, Coord{X: g.mapSize.X - 1.0, Y: g.mapSize.Y - 1.0})
	return minCoord, maxCoord, true
}

// 将世界坐标转换为瓦片坐标
func (g *StaticTileGrid) WorldToTileCoord(worldPos mgl32.Vec2) Coord {
	return Coord{
		X: int(math.Floor(float64(worldPos.X() / float32(g.tileSize.X)))),
		Y: int(math.Floor(float64(worldPos.Y() / float32(g.tileSize.Y)))),
	}
}

// 获取世界坐标所在瓦片对应的世界矩形
func (g *StaticTileGrid) RectAtWorldPos(worldPos mgl32.Vec2) emath.Rect {
	coord := g.WorldToTileCoord(worldPos)
	return emath.Rect{
		Position: mgl32.Vec2{float32(coord.X * g.tileSize.X), float32(coord.Y * g.tileSize.Y)},
		Size:     mgl32.Vec2{float32(g.tileSize.X), float32(g.tileSize.Y)},
	}
}

// 沿 Y 轴扫掠矩形并返回首个阻挡边界裁剪后的位置
// 是否撞到阻挡，裁剪后矩形左上角的 Y，命中的边界信息
func (g *StaticTileGrid) SweepVertical(startRect, targetRect emath.Rect, movingSouth bool) (bool, float32, SweepHitInfo) {
	hitInfo := newSweepHitInfo()
	if !g.IsInitialized() {
		return false, targetRect.Position.Y(), hitInfo
	}

	startEdge := startRect.Position.Y()
	targetEdge := targetRect.Position.Y()
	if movingSouth {
		startEdge += startRect.Size.Y()
		targetEdge += targetRect.Size.Y()
	}
	// 如果前沿几乎没有移动，就直接返回。
	if math.Abs(float64(targetEdge-startEdge)) < float64(gridEpsilon) {
		return false, targetRect.Position.Y(), hitInfo
	}

	// 计算矩形覆盖的列
	minX := min(startRect.Position.X(), targetRect.Position.X())
	maxX := max(
		startRect.Position.X()+startRect.Size.X(),
		targetRect.Position.X()+targetRect.Size.X(),
	) - gridEpsilon
	colStart := int(math.Floor(float64(minX / float32(g.tileSize.X))))
	colEnd := int(math.Floor(float64(maxX / float32(g.tileSize.X))))
	hitInfo.ColStart = colStart
	hitInfo.ColEnd = colEnd

	// 向下扫描
	tileHeight := float32(g.tileSize.Y)
	if movingSouth {
		firstBoundary := int(math.Ceil(float64(startEdge / tileHeight)))
		lastBoundary := int(math.Floor(float64(targetEdge / tileHeight)))
		// 假设 tile 高度为 32，矩形底边从 y=20 移动到 y=100，途中会跨过：y=32 → y=64 → y=96
		// 循环从上到下依次检查：boundaryRow = 1、2、3
		for boundaryRow := firstBoundary; boundaryRow <= lastBoundary; boundaryRow++ {
			// 检查水平边界两侧是否存在方向阻挡
			info := g.horizontalEdgeBlockInfo(boundaryRow, colStart, colEnd)
			if !info.blocked {
				continue
			}
			hitInfo.BoundaryRow = boundaryRow
			hitInfo.HitSolid = info.hasSolid
			resolvedBottom := float32(boundaryRow)*tileHeight - gridEpsilon
			return true, resolvedBottom - startRect.Size.Y(), hitInfo
		}
		return false, targetRect.Position.Y(), hitInfo
	}

	// 向上扫描
	firstBoundary := int(math.Floor(float64(startEdge / tileHeight)))
	lastBoundary := int(math.Ceil(float64(targetEdge / tileHeight)))
	for boundaryRow := firstBoundary; boundaryRow >= lastBoundary; boundaryRow-- {
		info := g.horizontalEdgeBlockInfo(boundaryRow, colStart, colEnd)
		if !info.blocked {
			continue
		}
		hitInfo.BoundaryRow = boundaryRow
		hitInfo.HitSolid = info.hasSolid
		return true, float32(boundaryRow)*tileHeight + gridEpsilon, hitInfo
	}
	return false, targetRect.Position.Y(), hitInfo
}

// 沿 X 轴扫掠矩形并返回首个阻挡边界裁剪后的位置
func (g *StaticTileGrid) SweepHorizontal(startRect, targetRect emath.Rect, movingEast bool) (bool, float32, SweepHitInfo) {
	hitInfo := newSweepHitInfo()
	if !g.IsInitialized() {
		return false, targetRect.Position.X(), hitInfo
	}

	startEdge := startRect.Position.X()
	targetEdge := targetRect.Position.X()
	if movingEast {
		startEdge += startRect.Size.X()
		targetEdge += targetRect.Size.X()
	}
	if math.Abs(float64(targetEdge-startEdge)) < float64(gridEpsilon) {
		return false, targetRect.Position.X(), hitInfo
	}

	minY := min(startRect.Position.Y(), targetRect.Position.Y())
	maxY := max(
		startRect.Position.Y()+startRect.Size.Y(),
		targetRect.Position.Y()+targetRect.Size.Y(),
	) - gridEpsilon
	rowStart := int(math.Floor(float64(minY / float32(g.tileSize.Y))))
	rowEnd := int(math.Floor(float64(maxY / float32(g.tileSize.Y))))
	hitInfo.RowStart = rowStart
	hitInfo.RowEnd = rowEnd

	tileWidth := float32(g.tileSize.X)
	if movingEast {
		firstBoundary := int(math.Ceil(float64(startEdge / tileWidth)))
		lastBoundary := int(math.Floor(float64(targetEdge / tileWidth)))
		for boundaryCol := firstBoundary; boundaryCol <= lastBoundary; boundaryCol++ {
			info := g.verticalEdgeBlockInfo(boundaryCol, rowStart, rowEnd)
			if !info.blocked {
				continue
			}
			hitInfo.BoundaryCol = boundaryCol
			hitInfo.HitSolid = info.hasSolid
			resolvedRight := float32(boundaryCol)*tileWidth - gridEpsilon
			return true, resolvedRight - startRect.Size.X(), hitInfo
		}
		return false, targetRect.Position.X(), hitInfo
	}

	firstBoundary := int(math.Floor(float64(startEdge / tileWidth)))
	lastBoundary := int(math.Ceil(float64(targetEdge / tileWidth)))
	for boundaryCol := firstBoundary; boundaryCol >= lastBoundary; boundaryCol-- {
		info := g.verticalEdgeBlockInfo(boundaryCol, rowStart, rowEnd)
		if !info.blocked {
			continue
		}
		hitInfo.BoundaryCol = boundaryCol
		hitInfo.HitSolid = info.hasSolid
		return true, float32(boundaryCol)*tileWidth + gridEpsilon, hitInfo
	}
	return false, targetRect.Position.X(), hitInfo
}

// 检查竖直瓦片边界两侧是否存在方向阻挡
func (g *StaticTileGrid) verticalEdgeBlockInfo(boundaryCol, rowStart, rowEnd int) edgeBlockInfo {
	// 同一条竖直边界可以由两侧任意一个 tile 声明阻挡
	leftCol := boundaryCol - 1
	rightCol := boundaryCol
	info := edgeBlockInfo{}
	for row := rowStart; row <= rowEnd; row++ {
		if row < 0 || row >= g.mapSize.Y {
			continue
		}
		if leftCol >= 0 && leftCol < g.mapSize.X {
			tileType := g.TileType(Coord{X: leftCol, Y: row})
			info.blocked = info.blocked || HasTileFlag(tileType, TileBlockE)
			info.hasSolid = info.hasSolid || HasAllTileFlags(tileType, TileSolidFlag)
		}
		if rightCol >= 0 && rightCol < g.mapSize.X {
			tileType := g.TileType(Coord{X: rightCol, Y: row})
			info.blocked = info.blocked || HasTileFlag(tileType, TileBlockW)
			info.hasSolid = info.hasSolid || HasAllTileFlags(tileType, TileSolidFlag)
		}
		if info.blocked {
			return info
		}
	}
	return info
}

// 检查水平瓦片边界两侧是否存在方向阻挡
func (g *StaticTileGrid) horizontalEdgeBlockInfo(boundaryRow, colStart, colEnd int) edgeBlockInfo {
	// 同一条水平边界可以由两侧任意一个 tile 声明阻挡
	aboveRow := boundaryRow - 1
	belowRow := boundaryRow
	info := edgeBlockInfo{}
	for col := colStart; col <= colEnd; col++ {
		if col < 0 || col >= g.mapSize.X {
			continue
		}
		if belowRow >= 0 && belowRow < g.mapSize.Y {
			tileType := g.TileType(Coord{X: col, Y: belowRow})
			info.blocked = info.blocked || HasTileFlag(tileType, TileBlockN)
			info.hasSolid = info.hasSolid || HasAllTileFlags(tileType, TileSolidFlag)
		}
		if aboveRow >= 0 && aboveRow < g.mapSize.Y {
			tileType := g.TileType(Coord{X: col, Y: aboveRow})
			info.blocked = info.blocked || HasTileFlag(tileType, TileBlockS)
			info.hasSolid = info.hasSolid || HasAllTileFlags(tileType, TileSolidFlag)
		}
		if info.blocked {
			return info
		}
	}
	return info
}

// 判断瓦片坐标是否位于地图范围内
func (g *StaticTileGrid) validCoord(coord Coord) bool {
	return coord.X >= 0 && coord.X < g.mapSize.X && coord.Y >= 0 && coord.Y < g.mapSize.Y
}

// 将二维瓦片坐标转换为一维单元索引
func (g *StaticTileGrid) index(coord Coord) int {
	return coord.Y*g.mapSize.X + coord.X
}
