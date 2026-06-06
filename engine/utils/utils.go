package utils

import (
	"fmt"
	"image"
	"image/draw"
	"strconv"
	"strings"
	"unsafe"

	"github.com/go-gl/mathgl/mgl32"
)

// 把 float32 切片作为字节切片
func Float32Bytes(values []float32) []byte {
	if len(values) == 0 {
		return nil
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}

// 把 uint32 切片作为字节切片
func Uint32Bytes(values []uint32) []byte {
	if len(values) == 0 {
		return nil
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}

// 将任意 image.Image 转成紧凑 RGBA 像素数组
// flipY 是否垂直翻转，true 表示翻转
// 图片像素往往按照左上方为(0，0)坐标点组织数据，OpenGL按照左下方为(0，0)，必须反转y轴
func ImageToNRGBA(img image.Image, flipY bool) *image.NRGBA {
	// 1. 如果本身就是紧凑的 NRGBA 且不需要翻转，直接返回
	if rgba, ok := img.(*image.NRGBA); ok && !flipY && rgba.Stride == rgba.Rect.Dx()*4 {
		return rgba
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	rowSize := w * 4

	// 2. 预先分配好目标空间
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))

	// 3. 将输入图像转为标准的 NRGBA 缓冲区 (src)
	// 这里必须先有一个标准化的过程，因为输入 img 可能是 Paletted, Gray, YCbCr 等
	var src *image.NRGBA
	if s, ok := img.(*image.NRGBA); ok && s.Stride == rowSize {
		src = s
	} else {
		src = image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(src, src.Bounds(), img, bounds.Min, draw.Src)
	}

	// 4. 进行翻转逻辑
	if flipY {
		// 以行为单位，从两头向中间拷贝
		for y := range h {
			srcOffset := y * rowSize
			dstOffset := (h - 1 - y) * rowSize
			// 核心：copy 指令在 Go 中是汇编优化的，极快
			copy(dst.Pix[dstOffset:dstOffset+rowSize], src.Pix[srcOffset:srcOffset+rowSize])
		}
	} else if src != dst {
		// 如果不需要翻转但 src 还没拷贝到 dst (比如转换了格式)，直接全量拷贝
		copy(dst.Pix, src.Pix)
	}

	return dst
}

// 解析 #RRGGBB 或 #RRGGBBAA 颜色
func ParseHexColor(value string) (mgl32.Vec4, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(trimmed) != 6 && len(trimmed) != 8 {
		return mgl32.Vec4{}, fmt.Errorf("color %q is invalid", value)
	}
	if len(trimmed) == 6 {
		trimmed += "FF"
	}
	parsed, err := strconv.ParseUint(trimmed, 16, 32)
	if err != nil {
		return mgl32.Vec4{}, fmt.Errorf("color %q is invalid: %w", value, err)
	}
	return mgl32.Vec4{
		float32((parsed>>24)&0xFF) / 255.0,
		float32((parsed>>16)&0xFF) / 255.0,
		float32((parsed>>8)&0xFF) / 255.0,
		float32(parsed&0xFF) / 255.0,
	}, nil
}
