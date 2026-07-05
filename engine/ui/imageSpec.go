package ui

import (
	"errors"

	"tiny_farm/engine/utils/defs"
	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
)

// 描述源图片四边需要保持固定的像素宽度：
//  left           right
// ┌────┬────────┬────┐
// │左上│   上   │右上 │ top
// ├────┼────────┼────┤
// │ 左 │  中间  │ 右  │
// ├────┼────────┼────┤
// │左下│   下   │右下 │ bottom
// └────┴────────┴────┘
//
// 四个角通常保持原尺寸，边缘只沿一个方向拉伸，中间区域可以双向拉伸。

// 表示九宫格四边在源图片中的像素宽度
type NineSliceMargins struct {
	// 左侧固定区域宽度，单位为源图片像素
	Left float32 `json:"left"`
	// 顶部固定区域高度，单位为源图片像素
	Top float32 `json:"top"`
	// 右侧固定区域宽度，单位为源图片像素
	Right float32 `json:"right"`
	// 底部固定区域高度，单位为源图片像素
	Bottom float32 `json:"bottom"`
}

// 判断当前配置是否启用图片切片
// 负数边距视为无效，四边全为零时按普通图片绘制
func (m *NineSliceMargins) ShouldSlice() bool {
	return m.Left >= 0.0 && m.Top >= 0.0 && m.Right >= 0.0 && m.Bottom >= 0.0 &&
		(m.Left+m.Right > 0.0 || m.Top+m.Bottom > 0.0)
}

// 描述 UI 贴图绘制数据
type ImageSpec struct {
	// 纹理在资源系统中的映射键
	TextureKey defs.ResourceKey
	// 未指定纹理键时使用的资源路径
	Path string
	// 源图片裁剪区域，依次为 x、y、宽度和高度
	SourceRect mgl32.Vec4
	// 与纹理颜色相乘的 RGBA 调制值
	Color mgl32.Vec4
	// 是否沿水平方向翻转纹理
	Flipped bool
	// 可选的九宫格边距配置
	NineSlice *NineSliceMargins
}

// 返回解析后的纹理 key
func (s ImageSpec) ResolvedTextureKey() defs.ResourceKey {
	if s.TextureKey != "" {
		return s.TextureKey
	}
	return defs.ResourceKey(s.Path)
}

// 返回图片实际使用的颜色调制
func (s ImageSpec) ResolvedColor() mgl32.Vec4 {
	if s.Color == (mgl32.Vec4{}) {
		return mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	return s.Color
}

// 保存单个九宫格切片的目标区域和源区域
type imagePatch struct {
	// 目标绘制区域，依次为 x、y、宽度和高度
	dst mgl32.Vec4
	// 源图片裁剪区域，依次为 x、y、宽度和高度
	src mgl32.Vec4
}

// 按九宫格的 3×3 布局拆分源区域和目标区域
//
// 宽高为零的区域不会生成切片，因此返回结果可能少于九个
func buildNineSlicePatches(src, dst mgl32.Vec4, margins NineSliceMargins) ([]imagePatch, error) {
	// 源区域或目标区域没有有效宽高时无法切片
	if src.Z() <= 0.0 || src.W() <= 0.0 || dst.Z() <= 0.0 || dst.W() <= 0.0 {
		return nil, errors.New("nine-slice source or destination size is invalid")
	}

	// 避免边距超过源图片尺寸
	left := min(margins.Left, src.Z())
	right := min(margins.Right, max(src.Z()-left, 0.0))
	top := min(margins.Top, src.W())
	bottom := min(margins.Bottom, max(src.W()-top, 0.0))

	// 目标区域太小，无法容纳两侧边框时，按目标区域尺寸调整，缩放
	dstLeft, dstRight := emath.ClampPair(left, right, dst.Z())
	dstTop, dstBottom := emath.ClampPair(top, bottom, dst.W())

	// 源区域[左边宽度, 中间宽度, 右边宽度]
	srcWidths := [3]float32{left, max(src.Z()-left-right, 0.0), right}
	// 源区域[上边高度, 中间高度, 下边高度]
	srcHeights := [3]float32{top, max(src.W()-top-bottom, 0.0), bottom}
	// 目标区域[左边宽度, 中间宽度, 右边宽度]
	dstWidths := [3]float32{dstLeft, max(dst.Z()-dstLeft-dstRight, 0.0), dstRight}
	// 目标区域[上边高度, 中间高度, 下边高度]
	dstHeights := [3]float32{dstTop, max(dst.W()-dstTop-dstBottom, 0.0), dstBottom}

	patches := make([]imagePatch, 0, 9)
	srcY, dstY := src.Y(), dst.Y()
	// 按九宫格模型计算切片，但不保证输出九个 patch
	// 按行、按列遍历 3×3 网格
	// 左上 →上中 →右上
	// 左中 →中间 →右中
	// 左下 →下中 →右下
	for row := range 3 {
		srcX, dstX := src.X(), dst.X()
		for column := range 3 {
			if srcWidths[column] > 0.0 && srcHeights[row] > 0.0 && dstWidths[column] > 0.0 && dstHeights[row] > 0.0 {
				patches = append(patches, imagePatch{
					src: mgl32.Vec4{srcX, srcY, srcWidths[column], srcHeights[row]},
					dst: mgl32.Vec4{dstX, dstY, dstWidths[column], dstHeights[row]},
				})
			}
			srcX += srcWidths[column]
			dstX += dstWidths[column]
		}
		srcY += srcHeights[row]
		dstY += dstHeights[row]
	}
	return patches, nil
}

// 按图片描述将源区域绘制到目标区域，需要时拆分为多个切片绘制
func drawImageSpec(uiCtx *uiContext, spec ImageSpec, dstRect mgl32.Vec4) error {
	srcRect := spec.SourceRect
	if srcRect.Z() <= 0.0 || srcRect.W() <= 0.0 {
		return nil
	}
	if uiCtx.renderer == nil {
		return errors.New("renderer is nil")
	}
	if uiCtx.resManager == nil {
		return errors.New("texture manager is nil")
	}
	key := spec.ResolvedTextureKey()
	if key == "" {
		return errors.New("image texture key is empty")
	}
	texture, err := uiCtx.resManager.LoadTexture(key, spec.Path)
	if err != nil {
		return err
	}
	// 判断是否使用切片绘制
	if spec.NineSlice != nil && spec.NineSlice.ShouldSlice() {
		margins := *spec.NineSlice
		// 需要水平翻转时交换左右边距
		if spec.Flipped {
			margins.Left, margins.Right = margins.Right, margins.Left
		}
		// 这里会把源图和目标区域拆成多个矩形。可能是 2 个、3 个、9 个，不一定真的是完整九宫格
		patches, err := buildNineSlicePatches(srcRect, dstRect, margins)
		if err != nil {
			return err
		}
		for _, patch := range patches {
			// 如果渲染一个完整矩形，dst和src不需要改动，渲染层会反转UV，渲染接口已经提供翻转功能。
			// 但是nine-slice已经把一张图拆成多个矩形分别画了，所以光靠渲染接口提供的反转只能解决每个小块内部UV反转，
			// 但不能解决，左块应该改取右侧源图，右块应该改取左侧源图。所以需要手动调整源区域的 x 坐标。
			if spec.Flipped {
				// source patch 的左右换位
				// ┌──────┬────────┬──────┐
				// │ A    │  B     │ C    │
				// └──────┴────────┴──────┘
				//
				// ┌──────┬────────┬──────┐
				// │ C    │  B     │ A    │
				// └──────┴────────┴──────┘
				patch.src[0] = srcRect.X() + srcRect.Z() - (patch.src.X() - srcRect.X()) - patch.src.Z()
			}
			if err := uiCtx.renderer.DrawUITextureSourceRectColor(texture, patch.dst, patch.src, spec.ResolvedColor(), spec.Flipped); err != nil {
				return err
			}
		}
		return nil
	}
	// 不需要切片，最后就是普通图片绘制
	return uiCtx.renderer.DrawUITextureSourceRectColor(texture, dstRect, srcRect, spec.ResolvedColor(), spec.Flipped)
}
