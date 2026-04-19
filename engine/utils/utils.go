package utils

import (
	"unsafe"
)

// 把 float32 切片作为字节切片传给 OpenGL 缓冲上传接口
func Float32Bytes(values []float32) []byte {
	if len(values) == 0 {
		return nil
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}

// 把 uint32 切片作为字节切片传给 OpenGL 缓冲上传接口
func Uint32Bytes(values []uint32) []byte {
	if len(values) == 0 {
		return nil
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}
