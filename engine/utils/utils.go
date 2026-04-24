package utils

import (
	"image"
	"image/draw"
	"unsafe"
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
func ImageToNRGBA(img image.Image) *image.NRGBA {
	if rgba, ok := img.(*image.NRGBA); ok && rgba.Stride == rgba.Rect.Dx()*4 {
		return rgba
	}

	bounds := img.Bounds()
	rgba := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	return rgba
}
