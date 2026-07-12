package spatial

import (
	emath "tiny_farm/engine/utils/math"
)

// 瓦片类型位掩码，用于静态地图属性和方向阻挡
type TileType uint32

const (
	// 普通地块
	TileNormal TileType = 0
	// 地块北侧(上边)阻挡地块
	TileBlockN TileType = 1 << iota
	// 地块南侧(下边)阻挡地块
	TileBlockS
	// 地块西侧(左边)阻挡地块
	TileBlockW
	// 地块东侧(右边)阻挡地块
	TileBlockE
)

const (
	// 引擎内置标志占用低 8 位
	TileBuiltinMask TileType = 0xFF
	// 游戏业务从第 9 位开始扩展
	TileCustomStart TileType = 1 << 8
	// TileSolid 表示四个方向都阻挡的完整墙体
	TileSolidFlag = TileBlockN | TileBlockS | TileBlockW | TileBlockE
	// 配合矩形的半开区间 [Position, Position+Size-epsilon)，避免右边缘或下边缘刚好落在 cell 边界时，多计算一格。
	gridEpsilon = float32(0.001)
)

// 判断是否包含任意指定标志
func HasTileFlag(value, flag TileType) bool {
	return value&flag != 0
}

// 判断是否完整包含指定标志组合
func HasAllTileFlags(value, mask TileType) bool {
	return value&mask == mask
}

// 网格坐标，X 为列，Y 为行
type Coord struct {
	X int
	Y int
}

// 把坐标限制在指定范围内，避免坐标越界
func clampCoord(value, minValue, maxValue Coord) Coord {
	return Coord{
		X: emath.Clamp(value.X, minValue.X, maxValue.X),
		Y: emath.Clamp(value.Y, minValue.Y, maxValue.Y),
	}
}

// 字符串图层标识，用于同一瓦片内按语义分桶实体
type LayerID string
