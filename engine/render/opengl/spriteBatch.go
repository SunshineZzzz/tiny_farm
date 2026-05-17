package opengl

import (
	"errors"
	"fmt"
	"unsafe"

	"tiny_farm/engine/utils"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	// 每个 float32 占用字节数
	float32Size = int(unsafe.Sizeof(float32(0)))
	// 每个 uint32 占用字节数
	uint32Size = int(unsafe.Sizeof(uint32(0)))
	// 最小精灵批处理容量
	minSpriteBatchCapacity = 64
	// 每个精灵顶点 float 数量
	// 2 个 float：x, y
	// 2 个 float：u, v
	// 4 个 float：r, g, b, a
	spriteVertexFloatCount = 8
	// 每个精灵顶点字节数
	spriteVertexByteSize = spriteVertexFloatCount * float32Size
	// 每个精灵顶点数量
	spriteVertexCount = 4
	// 每个精灵索引数量
	spriteIndexCount = 6
)

// 精灵绘制命令
type spriteCommand struct {
	// 底层 OpenGL texture 句柄
	texture uint32
	// 这条命令在索引缓冲里的起始位置
	indexFrom uint32
	// 这条命令要画多少个 index
	indexCount uint32
	// 是否使用纹理
	useTexture bool
}

// CPU 端精灵批处理
//
// 当前阶段支持纯色矩形和基础贴图，按提交顺序合并连续使用同一纹理的命令
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
	// 纯色矩形使用的 1x1 白色纹理
	defaultTexture uint32
	// CPU 端顶点缓存，每个顶点为 x、y、u、v、r、g、b、a
	vertices []float32
	// CPU 端索引缓存
	indices []uint32
	// CPU 端精灵绘制命令缓存
	commands []spriteCommand
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
	// 设置 UV 描述信息
	// 1 - location = 1
	// 2 - 这个属性有 2 个 float：u, v
	// gl.FLOAT - 类型是 float
	// false - 不归一化
	// spriteVertexByteSize - 每个顶点间隔 32 字节
	// 2*4 - 从每个顶点的第 8 字节开始读
	b.glCtx.VertexAttribPointer(1, 2, gl.FLOAT, false, int32(spriteVertexByteSize), 2*float32Size)
	// 启用 location = 1，让 shader 可以收到这个顶点 UV 数据
	b.glCtx.EnableVertexAttribArray(1)
	// 设置颜色描述信息
	// 2 - location = 2
	// 4 - 这个属性有 4 个 float：r, g, b, a
	// gl.FLOAT - 类型是 float
	// false - 不归一化
	// spriteVertexByteSize - 每个顶点间隔 32 字节
	// 4*4 - 从每个顶点的第 16 字节开始读
	b.glCtx.VertexAttribPointer(2, 4, gl.FLOAT, false, int32(spriteVertexByteSize), 4*float32Size)
	// 启用 location = 2，让 shader 可以收到这个顶点颜色数据
	b.glCtx.EnableVertexAttribArray(2)
	// 解绑VAO
	b.glCtx.BindVertexArray(0)

	// 确保CPU端缓存容量足够和GPU端缓存容量足够
	if err := b.ensureCapacity(initialCapacity); err != nil {
		return err
	}

	// 清空CPU端缓存
	b.reset()

	// 创建默认纹理
	if err := b.createDefaultTexture(); err != nil {
		return err
	}

	return nil
}

// 释放 GPU 和 CPU 端批处理资源
func (b *spriteBatch) clean() {
	if b == nil || b.glCtx == nil {
		return
	}

	if b.defaultTexture != 0 {
		b.glCtx.DeleteTexture(b.defaultTexture)
		b.defaultTexture = 0
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
	b.commands = nil
}

// 将纯色矩形加入本帧批处理队列
func (b *spriteBatch) queueRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if b == nil || b.glCtx == nil {
		return errors.New("sprite batch is nil")
	}

	if rect.Z() <= 0 || rect.W() <= 0 {
		return nil
	}

	return b.queueSprite(0, false, rect, mgl32.Vec4{0.0, 0.0, 1.0, 1.0}, color)
}

// 将纯色四边形加入本帧批处理队列
func (b *spriteBatch) queueQuad(points [4]mgl32.Vec2, color mgl32.Vec4) error {
	if b == nil || b.glCtx == nil {
		return errors.New("sprite batch is nil")
	}

	spriteCount := len(b.vertices) / (spriteVertexCount * spriteVertexFloatCount)
	if err := b.ensureCapacity(spriteCount + 1); err != nil {
		return err
	}

	baseIndex := uint32(len(b.vertices) / spriteVertexFloatCount)
	uv := [4]mgl32.Vec2{
		{0.0, 0.0},
		{1.0, 0.0},
		{1.0, 1.0},
		{0.0, 1.0},
	}
	for i, point := range points {
		b.vertices = append(b.vertices,
			point.X(), point.Y(),
			uv[i].X(), uv[i].Y(),
			color.X(), color.Y(), color.Z(), color.W(),
		)
	}

	indexFrom := uint32(len(b.indices))
	b.indices = append(b.indices,
		baseIndex+0, baseIndex+1, baseIndex+2,
		baseIndex+2, baseIndex+3, baseIndex+0,
	)
	if len(b.commands) > 0 {
		last := &b.commands[len(b.commands)-1]
		if last.texture == 0 && !last.useTexture {
			last.indexCount += spriteIndexCount
			return nil
		}
	}
	b.commands = append(b.commands, spriteCommand{
		texture:    0,
		indexFrom:  indexFrom,
		indexCount: spriteIndexCount,
		useTexture: false,
	})

	return nil
}

// 将贴图矩形加入本帧批处理队列
func (b *spriteBatch) queueTexture(texture *Texture, rect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if texture == nil || texture.id == 0 {
		return errors.New("texture is nil")
	}

	// 对外 UV 语义保持左上为原点，这里统一转换成 OpenGL 采样使用的 v 方向
	glUVRect := mgl32.Vec4{
		uvRect.X(),
		1.0 - uvRect.Y(),
		uvRect.Z(),
		1.0 - uvRect.W(),
	}

	return b.queueSprite(texture.id, true, rect, glUVRect, color)
}

// 将一个精灵加入本帧队列，纹理命令只合并相邻且纹理一致的段
func (b *spriteBatch) queueSprite(texture uint32, useTexture bool, rect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
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
	// rect.X() // 左上角 x
	// rect.Y() // 左上角 y
	// rect.Z() // 宽度 width
	// rect.W() // 高度 height
	//
	// uvRect.X() // 左 u
	// uvRect.Y() // 上 v
	// uvRect.Z() // 右 u
	// uvRect.W() // 下 v
	//
	// 左上，右上，右下，左下
	b.vertices = append(b.vertices,
		rect.X(), rect.Y(), uvRect.X(), uvRect.Y(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X()+rect.Z(), rect.Y(), uvRect.Z(), uvRect.Y(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X()+rect.Z(), rect.Y()+rect.W(), uvRect.Z(), uvRect.W(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X(), rect.Y()+rect.W(), uvRect.X(), uvRect.W(), color.X(), color.Y(), color.Z(), color.W(),
	)
	indexFrom := uint32(len(b.indices))
	b.indices = append(b.indices,
		baseIndex+0, baseIndex+1, baseIndex+2,
		baseIndex+2, baseIndex+3, baseIndex+0,
	)
	if len(b.commands) > 0 {
		last := &b.commands[len(b.commands)-1]
		if last.texture == texture && last.useTexture == useTexture {
			last.indexCount += spriteIndexCount
			return nil
		}
	}
	b.commands = append(b.commands, spriteCommand{
		texture:    texture,
		indexFrom:  indexFrom,
		indexCount: spriteIndexCount,
		useTexture: useTexture,
	})

	return nil
}

// 提交本帧所有已入队矩形
// textureLocation - 纹理 sampler 位置
// useTextureLocation - 是否使用纹理 sampler 位置
func (b *spriteBatch) flush(textureLocation int32, useTextureLocation int32) error {
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
	// 设置 uTexture 使用纹理单元 0
	if textureLocation >= 0 {
		b.glCtx.Uniform1i(textureLocation, 0)
	}
	// 激活纹理单元 0
	b.glCtx.ActiveTexture(gl.TEXTURE0)
	// 按 command 分批绘制精灵
	for _, cmd := range b.commands {
		texture := cmd.texture
		if texture == 0 {
			texture = b.defaultTexture
		}
		// 绑定到 TEXTURE0 上
		b.glCtx.BindTexture(gl.TEXTURE_2D, texture)
		if useTextureLocation >= 0 {
			if cmd.useTexture {
				b.glCtx.Uniform1i(useTextureLocation, 1)
			} else {
				b.glCtx.Uniform1i(useTextureLocation, 0)
			}
		}
		// 用三角形模式绘制，画 cmd.indexCount 个索引，索引类型是 uint32，从 EBO 的 cmd.indexFrom 位置开始读
		// 最后一个参数要注意：int(cmd.indexFrom) * uint32Size，这里传的是 字节偏移量，不是 index 数量
		b.glCtx.DrawElements(gl.TRIANGLES, int32(cmd.indexCount), gl.UNSIGNED_INT, int(cmd.indexFrom)*uint32Size)
	}
	// 解绑 TEXTURE0
	b.glCtx.BindTexture(gl.TEXTURE_2D, 0)
	// 解绑VAO
	b.glCtx.BindVertexArray(0)

	// 清空CPU端缓存
	b.reset()

	return nil
}

// 返回当前队列的提交规模
//
// 调用方应在 flush 前读取，flush 会清空 CPU 端队列
func (b *spriteBatch) stats() spriteBatchStats {
	if b == nil {
		return spriteBatchStats{}
	}

	sprites := len(b.vertices) / (spriteVertexCount * spriteVertexFloatCount)
	return spriteBatchStats{
		drawCalls: len(b.commands),
		sprites:   sprites,
		vertices:  sprites * spriteVertexCount,
		indices:   len(b.indices),
	}
}

// 清空 CPU 端队列，保留底层容量用于下一帧复用
func (b *spriteBatch) reset() {
	b.vertices = b.vertices[:0]
	b.indices = b.indices[:0]
	b.commands = b.commands[:0]
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
	if cap(b.commands) < nextCapacity {
		newCommands := make([]spriteCommand, len(b.commands), nextCapacity)
		copy(newCommands, b.commands)
		b.commands = newCommands
	}

	return nil
}

// 创建纯色矩形使用的默认白纹理
func (b *spriteBatch) createDefaultTexture() error {
	if b.defaultTexture != 0 {
		return nil
	}

	texture := b.glCtx.CreateTexture()
	if texture == 0 {
		return fmt.Errorf("create default texture failed,%v", b.glCtx.GetError())
	}

	pixels := []byte{255, 255, 255, 255}
	b.glCtx.BindTexture(gl.TEXTURE_2D, texture)
	b.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	b.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	b.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	b.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	b.glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	b.glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(gl.RGBA), 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, pixels)
	b.glCtx.BindTexture(gl.TEXTURE_2D, 0)

	b.defaultTexture = texture
	return nil
}
