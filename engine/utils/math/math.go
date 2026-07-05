package math

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// float32 color
type FColor struct {
	R, G, B, A float32
}

// 创建FColor
func NewFColor(r, g, b float32, a float32) FColor {
	return FColor{
		R: r,
		G: g,
		B: b,
		A: a,
	}
}

// 从十六进制颜色值创建FColor
func NewFColorByHex(hex uint32) FColor {
	return FColor{
		R: float32((hex>>24)&0xFF) / 255.0,
		G: float32((hex>>16)&0xFF) / 255.0,
		B: float32((hex>>8)&0xFF) / 255.0,
		A: float32((hex & 0xFF)) / 255.0,
	}
}

// -- 创建一些预设颜色，方便使用 ---
func Red() FColor    { return NewFColor(1.0, 0.0, 0.0, 1.0) }
func Green() FColor  { return NewFColor(0.0, 1.0, 0.0, 1.0) }
func Blue() FColor   { return NewFColor(0.0, 0.0, 1.0, 1.0) }
func White() FColor  { return NewFColor(1.0, 1.0, 1.0, 1.0) }
func Black() FColor  { return NewFColor(0.0, 0.0, 0.0, 1.0) }
func Purple() FColor { return NewFColor(1.0, 0.0, 1.0, 1.0) }
func Orange() FColor { return NewFColor(1.0, 0.65, 0.0, 1.0) }
func Grey() FColor   { return NewFColor(0.5, 0.5, 0.5, 1.0) }
func Yellow() FColor { return NewFColor(1.0, 1.0, 0.0, 1.0) }

// 数值类型
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// 2维向量的布尔值
type Vec2B [2]bool

// 获取向量的X分量
func (v Vec2B) X() bool {
	return v[0]
}

// 获取向量的Y分量
func (v Vec2B) Y() bool {
	return v[1]
}

// 矩形
type Rect struct {
	// 矩形左上角的世界坐标
	Position mgl32.Vec2
	// 矩形的大小
	Size mgl32.Vec2
}

// 将矩形转换为依次包含 x、y、宽度和高度的四维向量
func (r Rect) RectToVec4() mgl32.Vec4 {
	return mgl32.Vec4{
		r.Position.X(),
		r.Position.Y(),
		r.Size.X(),
		r.Size.Y(),
	}
}

// 2维向量的元素最大值
func Mgl32Vec2Max(a, b mgl32.Vec2) mgl32.Vec2 {
	return mgl32.Vec2{
		max(a.X(), b.X()),
		max(a.Y(), b.Y()),
	}
}

// 限制向量在min向量和max向量之间
func Mgl32Vec2Clamp(vec, min, max mgl32.Vec2) mgl32.Vec2 {
	return mgl32.Vec2{
		mgl32.Clamp(vec.X(), min.X(), max.X()),
		mgl32.Clamp(vec.Y(), min.Y(), max.Y()),
	}
}

// 向量分量乘法
func Mgl32Vec2MulElem(src, factor mgl32.Vec2) mgl32.Vec2 {
	return mgl32.Vec2{
		src.X() * factor.X(),
		src.Y() * factor.Y(),
	}
}

// 取模运算
func Mod[T Number](x, y T) T {
	res := float64(x) - float64(y)*math.Floor(float64(x)/float64(y))
	return T(res)
}

// 2维向量的绝对值
func Mgl32Vec2ABS(a, b mgl32.Vec2) mgl32.Vec2 {
	return mgl32.Vec2{
		mgl32.Abs(a.X() - b.X()),
		mgl32.Abs(a.Y() - b.Y()),
	}
}

// 2维向量的线性插值
func Mgl32Vec2Mix(current, target mgl32.Vec2, t float32) mgl32.Vec2 {
	// 确保 t 在 0.0 到 1.0 之间，防止过冲（可选，取决于你是否想要弹性效果）
	if t > 1.0 {
		t = 1.0
	}
	if t < 0.0 {
		t = 0.0
	}

	// 公式：A + (B - A) * t
	return current.Add(target.Sub(current).Mul(t))
}

// clamp
// 限制值在min和max之间
func Clamp[T Number](value, min, max T) T {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

/**
 * @brief 计算窗口与逻辑尺寸的信箱(letterbox)视口信息。
 */
type LetterboxMetrics struct {
	// 视口信息(glViewport)
	Viewport Rect
	// 均匀缩放因子，窗口尺寸 / 逻辑尺寸
	Scale float32
}

func ComputeLetterboxMetrics(windowSize, logicalSize mgl32.Vec2) LetterboxMetrics {
	result := LetterboxMetrics{}

	if logicalSize.X() <= 0.0 || logicalSize.Y() <= 0.0 ||
		windowSize.X() <= 0.0 || windowSize.Y() <= 0.0 {
		result.Viewport = Rect{mgl32.Vec2{0.0, 0.0}, mgl32.Vec2{1.0, 1.0}}
		result.Scale = 1.0
		return result
	}

	scaleX := windowSize.X() / logicalSize.X()
	scaleY := windowSize.Y() / logicalSize.Y()
	result.Scale = min(scaleX, scaleY)

	if math.IsNaN(float64(result.Scale)) || math.IsInf(float64(result.Scale), 0) || result.Scale <= 0.0 {
		result.Scale = 1.0
	}

	// 逻辑尺寸 × 均匀缩放比例 = 画面在窗口里的实际物理大小。
	// 它不包含周围的黑边。
	// 它是你最终告诉显卡（通过 glViewport）“我这块画面要画多大”的那个数值。
	viewportSize := logicalSize.Mul(result.Scale)
	viewportSize = Mgl32Vec2Clamp(viewportSize, mgl32.Vec2{1.0, 1.0}, windowSize)
	result.Viewport.Size = viewportSize
	// 画面居中
	result.Viewport.Position = windowSize.Sub(viewportSize).Mul(0.5)
	return result
}

// 安全归一化2维向量
func SafeNormalizeVec2(v mgl32.Vec2, fallback mgl32.Vec2) mgl32.Vec2 {
	const eps = 1e-5
	lenSq := v.X()*v.X() + v.Y()*v.Y()
	if lenSq < eps*eps {
		return fallback
	}
	invLen := float32(1.0 / math.Sqrt(float64(lenSq)))
	return v.Mul(invLen)
}

// 按可用长度等比压缩一对边距，避免两侧区域发生重叠
func ClampPair(first, second, available float32) (float32, float32) {
	total := first + second
	if total <= available || total <= 0.0 {
		return first, second
	}
	scale := available / total
	return first * scale, second * scale
}

// 将四个浮点分量转换为四维向量，长度不符时返回回退值
func Vec4FromFloat32s(values []float32, fallback mgl32.Vec4) mgl32.Vec4 {
	if len(values) != 4 {
		return fallback
	}
	return mgl32.Vec4{values[0], values[1], values[2], values[3]}
}

// 将两个浮点分量转换为二维向量，长度不符时返回回退值
func Vec2FromFloat32s(values []float32, fallback mgl32.Vec2) mgl32.Vec2 {
	if len(values) != 2 {
		return fallback
	}
	return mgl32.Vec2{values[0], values[1]}
}
