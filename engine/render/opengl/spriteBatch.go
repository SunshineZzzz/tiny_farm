package opengl

import (
	"errors"
	"unsafe"

	"tiny_farm/engine/utils"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	// 每个float32占用字节数
	float32Size = int(unsafe.Sizeof(float32(0)))
	// 每个uint32占用字节数
	uint32Size = int(unsafe.Sizeof(uint32(0)))
	// 最小精灵批处理容量
	minSpriteBatchCapacity = 64
	// 每个精灵顶点 float 数量
	// 2 个 float：x, y
	// 4 个 float：r, g, b, a
	spriteVertexFloatCount = 6
	// 每个精灵顶点字节数
	spriteVertexByteSize = spriteVertexFloatCount * float32Size
	// 每个精灵顶点数量
	spriteVertexCount = 4
	// 每个精灵索引数量
	spriteIndexCount = 6
)

// CPU 端精灵批处理
//
// 当前阶段只支持纯色矩形，先复用参考实现的“入队后统一 flush”结构
type spriteBatch struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 顶点数组对象
	vao uint32
	// 顶点缓冲对象
	vbo uint32
	// 索引缓冲对象
	ebo uint32
	// 当前 GPU 缓冲可容纳的精灵数量
	capacity int
	// CPU 端顶点缓存，每个顶点为 x、y、r、g、b、a
	vertices []float32
	// CPU 端索引缓存
	indices []uint32
}

// 创建纯色矩形批处理器
func newSpriteBatch(glCtx gl.Context, initialCapacity int) (*spriteBatch, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	if initialCapacity < minSpriteBatchCapacity {
		initialCapacity = minSpriteBatchCapacity
	}

	batch := &spriteBatch{glCtx: glCtx}
	if err := batch.init(initialCapacity); err != nil {
		batch.clean()
		return nil, err
	}

	return batch, nil
}

// 初始化 VAO、VBO、EBO 和顶点格式
func (b *spriteBatch) init(initialCapacity int) error {
	// 创建VAO，VBO，EBO
	b.vao = b.glCtx.CreateVertexArray()
	b.vbo = b.glCtx.CreateBuffer()
	b.ebo = b.glCtx.CreateBuffer()

	if b.vao == 0 || b.vbo == 0 || b.ebo == 0 {
		return errors.New("create sprite batch buffers failed")
	}

	// 绑定VAO，接下来设置的顶点格式，都记录到这个 VAO 里
	b.glCtx.BindVertexArray(b.vao)
	// 绑定VBO
	b.glCtx.BindBuffer(gl.ARRAY_BUFFER, b.vbo)
	// 绑定EBO
	b.glCtx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, b.ebo)
	// 设置位置描述信息
	// 0 - location = 0
	// 2 - 这个属性有 2 个 float：x, y
	// gl.FLOAT - 类型是 float
	// false - 不归一化
	// spriteVertexByteSize - 每个顶点间隔 24 字节
	// 0 - 从每个顶点的第 0 字节开始读
	b.glCtx.VertexAttribPointer(0, 2, gl.FLOAT, false, int32(spriteVertexByteSize), 0)
	// 启用 location = 0，让 shader 可以收到这个顶点位置数据
	b.glCtx.EnableVertexAttribArray(0)
	// 设置颜色描述信息
	// 1 - location = 1
	// 4 - 这个属性有 4 个 float：r, g, b, a
	// gl.FLOAT - 类型是 float
	// false - 不归一化
	// spriteVertexByteSize - 每个顶点间隔 24 字节
	// 2*4 - 从每个顶点的第 8 字节开始读
	b.glCtx.VertexAttribPointer(1, 4, gl.FLOAT, false, int32(spriteVertexByteSize), 2*4)
	// 启用 location = 1，让 shader 可以收到这个顶点颜色数据
	b.glCtx.EnableVertexAttribArray(1)
	// 解绑VAO
	b.glCtx.BindVertexArray(0)

	// 确保CPU端缓存容量足够和GPU端缓存容量足够
	if err := b.ensureCapacity(initialCapacity); err != nil {
		return err
	}

	// 清空CPU端缓存
	b.reset()

	return nil
}

// 释放 GPU 和 CPU 端批处理资源
func (b *spriteBatch) clean() {
	if b == nil || b.glCtx == nil {
		return
	}

	if b.ebo != 0 {
		b.glCtx.DeleteBuffer(b.ebo)
		b.ebo = 0
	}

	if b.vbo != 0 {
		b.glCtx.DeleteBuffer(b.vbo)
		b.vbo = 0
	}

	if b.vao != 0 {
		b.glCtx.DeleteVertexArray(b.vao)
		b.vao = 0
	}

	b.capacity = 0
	b.vertices = nil
	b.indices = nil
}

// 将纯色矩形加入本帧批处理队列
func (b *spriteBatch) queueRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if b == nil || b.glCtx == nil {
		return errors.New("sprite batch is nil")
	}

	if rect.Z() <= 0 || rect.W() <= 0 {
		return nil
	}

	spriteCount := len(b.vertices) / (spriteVertexCount * spriteVertexFloatCount)
	if err := b.ensureCapacity(spriteCount + 1); err != nil {
		return err
	}

	baseIndex := uint32(len(b.vertices) / spriteVertexFloatCount)
	b.vertices = append(b.vertices,
		rect.X(), rect.Y(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X()+rect.Z(), rect.Y(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X()+rect.Z(), rect.Y()+rect.W(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X(), rect.Y()+rect.W(), color.X(), color.Y(), color.Z(), color.W(),
	)
	b.indices = append(b.indices,
		baseIndex+0, baseIndex+1, baseIndex+2,
		baseIndex+2, baseIndex+3, baseIndex+0,
	)

	return nil
}

// 提交本帧所有已入队矩形
func (b *spriteBatch) flush() error {
	if b == nil || b.glCtx == nil {
		return errors.New("sprite batch is nil")
	}

	if len(b.indices) == 0 {
		b.reset()
		return nil
	}

	// 绑定之前准备好的VAO，恢复这套顶点解释规则
	b.glCtx.BindVertexArray(b.vao)
	// 绑定VBO，上传顶点数据
	b.glCtx.BindBuffer(gl.ARRAY_BUFFER, b.vbo)
	b.glCtx.BufferSubData(gl.ARRAY_BUFFER, 0, utils.Float32Bytes(b.vertices))
	// 绑定EBO，上传索引数据
	b.glCtx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, b.ebo)
	b.glCtx.BufferSubData(gl.ELEMENT_ARRAY_BUFFER, 0, utils.Uint32Bytes(b.indices))
	// 回执
	b.glCtx.DrawElements(gl.TRIANGLES, int32(len(b.indices)), gl.UNSIGNED_INT, 0)
	// 解绑VAO
	b.glCtx.BindVertexArray(0)

	// 清空CPU端缓存
	b.reset()

	return nil
}

// 清空 CPU 端队列，保留底层容量用于下一帧复用
func (b *spriteBatch) reset() {
	b.vertices = b.vertices[:0]
	b.indices = b.indices[:0]
}

// 确保 GPU 缓冲 和 CPU 缓冲至少能容纳 requiredSprites 个精灵
func (b *spriteBatch) ensureCapacity(requiredSprites int) error {
	if requiredSprites <= b.capacity {
		return nil
	}

	nextCapacity := b.capacity * 2
	nextCapacity = max(nextCapacity, minSpriteBatchCapacity)
	for nextCapacity < requiredSprites {
		nextCapacity *= 2
	}
	b.capacity = nextCapacity

	// 确保CPU端缓存容量足够
	// 计算下一个容量所需顶点字节数
	vertexBytes := nextCapacity * spriteVertexCount * spriteVertexByteSize
	// 计算下一个容量所需索引字节数
	indexBytes := nextCapacity * spriteIndexCount * uint32Size
	// 绑定VAO，接下来设置的顶点格式，都记录到这个 VAO 里
	b.glCtx.BindVertexArray(b.vao)
	// 绑定VBO，并且分配显存空间
	b.glCtx.BindBuffer(gl.ARRAY_BUFFER, b.vbo)
	b.glCtx.BufferInit(gl.ARRAY_BUFFER, vertexBytes, gl.DYNAMIC_DRAW)
	// 绑定EBO，并且分配显存空间
	b.glCtx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, b.ebo)
	b.glCtx.BufferInit(gl.ELEMENT_ARRAY_BUFFER, indexBytes, gl.DYNAMIC_DRAW)
	// 解绑VAO
	b.glCtx.BindVertexArray(0)

	// 确保CPU端缓存容量足够
	if cap(b.vertices) < nextCapacity*spriteVertexCount*spriteVertexFloatCount {
		newVertices := make([]float32, len(b.vertices), nextCapacity*spriteVertexCount*spriteVertexFloatCount)
		copy(newVertices, b.vertices)
		b.vertices = newVertices
	}
	if cap(b.indices) < nextCapacity*spriteIndexCount {
		newIndices := make([]uint32, len(b.indices), nextCapacity*spriteIndexCount)
		copy(newIndices, b.indices)
		b.indices = newIndices
	}

	return nil
}
