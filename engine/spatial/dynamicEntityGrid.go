package spatial

import (
	"math"
	"slices"
	"sort"

	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 动态实体网格，将运行时可变化实体的碰撞范围登记到对应空间单元
// 可移动、生成、移除或破坏的实体，需要在网格中进行动态更新
type DynamicEntityGrid struct {
	// 网格的列数和行数
	gridSize Coord
	// 单个空间单元的尺寸，单位为世界坐标，每个格子多大
	cellSize mgl32.Vec2
	// 网格覆盖区域的世界坐标最小值，网格从哪里开始
	worldBoundsMin mgl32.Vec2
	// 网格覆盖区域的世界坐标最大值，网格从哪里结束
	worldBoundsMax mgl32.Vec2
	// cells:
	//   (2, 3) → [玩家、树木]
	//   (3, 3) → [树木]
	// entityToCells:
	//   玩家 → {(2, 3)}
	//   树木 → {(2, 3), (3, 3)}
	// 正向索引，记录每个空间单元中的实体
	cells map[Coord][]donburi.Entity
	// 反向索引，记录每个实体当前覆盖的所有空间单元
	entityToCells map[donburi.Entity]map[Coord]struct{}
}

// 初始化动态网格
func (g *DynamicEntityGrid) Initialize(worldBoundsMin, worldBoundsMax, cellSize mgl32.Vec2) {
	g.worldBoundsMin = worldBoundsMin
	g.worldBoundsMax = worldBoundsMax
	g.cellSize = mgl32.Vec2{max(cellSize.X(), 1.0), max(cellSize.Y(), 1.0)}
	worldSize := worldBoundsMax.Sub(worldBoundsMin)
	g.gridSize = Coord{
		X: max(int(math.Ceil(float64(max(worldSize.X(), 0.0)/g.cellSize.X()))), 0.0),
		Y: max(int(math.Ceil(float64(max(worldSize.Y(), 0.0)/g.cellSize.Y()))), 0.0),
	}
	g.cells = make(map[Coord][]donburi.Entity)
	g.entityToCells = make(map[donburi.Entity]map[Coord]struct{})
}

// 重置为空状态
func (g *DynamicEntityGrid) Reset() {
	g.gridSize = Coord{}
	g.cellSize = mgl32.Vec2{1.0, 1.0}
	g.worldBoundsMin = mgl32.Vec2{}
	g.worldBoundsMax = mgl32.Vec2{}
	g.cells = nil
	g.entityToCells = nil
}

// 判断网格是否已经完成初始化
func (g *DynamicEntityGrid) IsInitialized() bool {
	return g.gridSize.X > 0 && g.gridSize.Y > 0 && g.cellSize.X() > 0 && g.cellSize.Y() > 0
}

// 添加矩形碰撞实体
func (g *DynamicEntityGrid) AddEntity(entity donburi.Entity, bounds emath.Rect) {
	g.RemoveEntity(entity)
	cells := g.CellsForRect(bounds)
	g.AddEntityToCells(entity, cells)
}

// 添加圆形碰撞实体
func (g *DynamicEntityGrid) AddCircleEntity(entity donburi.Entity, center mgl32.Vec2, radius float32) {
	g.AddEntity(entity, emath.Rect{
		Position: center.Sub(mgl32.Vec2{radius, radius}),
		Size:     mgl32.Vec2{radius * 2, radius * 2},
	})
}

// 删除实体
func (g *DynamicEntityGrid) RemoveEntity(entity donburi.Entity) {
	oldCells, ok := g.entityToCells[entity]
	if !ok {
		return
	}
	for coord := range oldCells {
		entities := g.cells[coord]
		for i, current := range entities {
			if current == entity {
				entities = append(entities[:i], entities[i+1:]...)
				break
			}
		}
		if len(entities) == 0 {
			delete(g.cells, coord)
		} else {
			g.cells[coord] = entities
		}
	}
	delete(g.entityToCells, entity)
}

// 查询矩形覆盖单元内的候选实体
func (g *DynamicEntityGrid) QueryEntities(rect emath.Rect) []donburi.Entity {
	return g.QueryCells(g.CellsForRect(rect))
}

// 查询圆形覆盖单元内的候选实体
func (g *DynamicEntityGrid) QueryCircleEntities(center mgl32.Vec2, radius float32) []donburi.Entity {
	return g.QueryEntities(emath.Rect{
		Position: center.Sub(mgl32.Vec2{radius, radius}),
		Size:     mgl32.Vec2{radius * 2, radius * 2},
	})
}

// 查询点所在单元内的候选实体
func (g *DynamicEntityGrid) QueryEntitiesAt(worldPos mgl32.Vec2) []donburi.Entity {
	cell := g.WorldToCell(worldPos)
	if !g.ValidCell(cell) {
		return nil
	}
	return append([]donburi.Entity(nil), g.cells[cell]...)
}

// 统计已使用的单元数量
func (g *DynamicEntityGrid) UsedCellCount() int {
	return len(g.cells)
}

// 统计已注册的实体数量
func (g *DynamicEntityGrid) EntityCount() int {
	return len(g.entityToCells)
}

// 将实体写入单元到实体的正向索引，并记录实体到单元的反向索引
func (g *DynamicEntityGrid) AddEntityToCells(entity donburi.Entity, cells map[Coord]struct{}) {
	if len(cells) == 0 {
		return
	}
	for coord := range cells {
		if !slices.Contains(g.cells[coord], entity) {
			// Go 允许对 nil 切片执行 append
			g.cells[coord] = append(g.cells[coord], entity)
		}
	}
	g.entityToCells[entity] = cells
}

// 汇总指定单元中的实体并按实体 ID 排序
func (g *DynamicEntityGrid) QueryCells(cells map[Coord]struct{}) []donburi.Entity {
	// 同一个实体作为 map 的键，只会保留一次
	seen := make(map[donburi.Entity]struct{})
	for coord := range cells {
		for _, entity := range g.cells[coord] {
			seen[entity] = struct{}{}
		}
	}
	results := make([]donburi.Entity, 0, len(seen))
	for entity := range seen {
		results = append(results, entity)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Id() < results[j].Id()
	})
	return results
}

// 计算矩形覆盖的所有有效空间单元
func (g *DynamicEntityGrid) CellsForRect(rect emath.Rect) map[Coord]struct{} {
	cells := make(map[Coord]struct{})
	if !g.IsInitialized() {
		return cells
	}
	// 计算矩形左上角所在 cell
	minCell := g.WorldToCell(rect.Position)
	// 矩形右下角世界坐标
	// Rect 覆盖范围，按半开区间 [pos, pos+size) 解释。计算覆盖的 tile/cell 范围时，对 max 端做 epsilon 收缩（pos + size - epsilon），
	// 避免"矩形尺寸恰好等于 tile 尺寸时多算一格"的 off-by-one 错误。
	maxPos := rect.Position.Add(rect.Size).Sub(mgl32.Vec2{gridEpsilon, gridEpsilon})
	// 防止最大位置小于起始位置
	maxPos = emath.Mgl32Vec2Max(maxPos, rect.Position)
	// 计算右下角所在 cell
	maxCell := g.WorldToCell(maxPos)
	// 限制在网格范围内
	minCell = clampCoord(minCell, Coord{}, Coord{X: g.gridSize.X - 1.0, Y: g.gridSize.Y - 1.0})
	maxCell = clampCoord(maxCell, Coord{}, Coord{X: g.gridSize.X - 1.0, Y: g.gridSize.Y - 1.0})
	for y := minCell.Y; y <= maxCell.Y; y++ {
		for x := minCell.X; x <= maxCell.X; x++ {
			cells[Coord{X: x, Y: y}] = struct{}{}
		}
	}
	return cells
}

// 将世界坐标转换为相对于网格原点的单元坐标
func (g *DynamicEntityGrid) WorldToCell(worldPos mgl32.Vec2) Coord {
	relative := worldPos.Sub(g.worldBoundsMin)
	// 用 floor((世界坐标-网格起点)/格子尺寸) 计算格子编号。点刚好位于格子分界线上时，归到编号更大的右侧或下侧格子。
	return Coord{
		X: int(math.Floor(float64(relative.X() / g.cellSize.X()))),
		Y: int(math.Floor(float64(relative.Y() / g.cellSize.Y()))),
	}
}

// 判断单元坐标是否位于网格范围内
func (g *DynamicEntityGrid) ValidCell(coord Coord) bool {
	return coord.X >= 0 && coord.X < g.gridSize.X && coord.Y >= 0 && coord.Y < g.gridSize.Y
}
