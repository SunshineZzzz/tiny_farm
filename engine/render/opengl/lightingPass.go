package opengl

import (
	"errors"
	"fmt"
	"math"

	"tiny_farm/engine/utils"
	emath "tiny_farm/engine/utils/math"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	// 点光
	lightTypePoint int32 = iota
	// 聚光灯
	lightTypeSpot
	// 方向光(平行光)
	lightTypeDirectional
)

const (
	// 一个光源 quad 有 4 个顶点：左上、右上、右下、左下
	lightQuadVertexCount = 4
	// 每个顶点写 4 个 float：x, y, u, v
	lightQuadFloatCountPerVertex = 4
	// UV 从每个顶点的第 3 个 float 开始，也就是跳过 x, y 两个 float
	lightQuadUVOffsetFloatCount = 2
	// 一个 quad 用两个三角形绘制，每个三角形 3 个索引，所以一共 6 个索引
	lightQuadIndexCount = 6
)

// 光源绘制命令
//
// 点光和聚光使用 logical 坐标矩形，方向光使用 screen-space 全屏矩形
type lightCommand struct {
	// 光源类型，决定本次绘制使用点光、聚光还是方向光分支
	lightType int32
	// 光源颜色，RGB 参与光照计算，Alpha 当前不参与计算
	color mgl32.Vec4
	// 光源强度，最终会与颜色和遮罩相乘
	intensity float32
	// 点光和聚光的中心位置，使用 logical 坐标
	position mgl32.Vec2
	// 点光和聚光的影响半径，单位为 logical 像素
	radius float32
	// 聚光灯朝向，使用局部 quad 的 UV 方向
	spotDir mgl32.Vec2
	// 聚光灯内锥角余弦值，夹角余弦大于该值时完全照亮
	spotInnerCos float32
	// 聚光灯外锥角余弦值，夹角余弦小于该值时完全不照
	spotOuterCos float32
	// 投影轴方向，沿该方向的投影值越大，遮罩越接近1，越亮
	// 注意：它不是物理平行光的“照射方向”，而是屏幕空间明暗渐变的方向
	dir2D mgl32.Vec2
	// 方向光明暗过渡中心位置，范围为 0 到 1
	dirOffset float32
	// 方向光明暗过渡柔和宽度，范围为 0 到 0.5
	dirSoftness float32
	// 正午混合强度，越接近 1 越接近全屏均匀照亮
	middayBlend float32
}

// 管理世界层光照缓冲
//
// 当前阶段生成 logical size 的 light color，动态光源以加法混合累积到黑底光照纹理
type lightingPass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 光照绘制 shader
	shader *shaderProgram
	// 光照 framebuffer
	framebuffer uint32
	// 光照颜色纹理
	colorTexture *Texture
	// 光照 quad 顶点数组对象
	vao uint32
	// 光照 quad 顶点缓冲
	vbo uint32
	// 光照 quad 索引缓冲
	ebo uint32
	// 光照缓冲尺寸，固定等于逻辑分辨率
	size mgl32.Vec2
	// 本帧待绘制光源命令
	commands []lightCommand
	// 是否启用光照合成
	enabled bool
	// 视图投影 uniform 位置
	viewProjLocation int32
	// 光源颜色 uniform 位置
	lightColorLocation int32
	// 光源强度 uniform 位置
	lightIntensityLocation int32
	// 光源类型 uniform 位置
	lightTypeLocation int32
	// 聚光方向 uniform 位置
	spotDirLocation int32
	// 聚光内角余弦 uniform 位置
	spotInnerCosLocation int32
	// 聚光外角余弦 uniform 位置
	spotOuterCosLocation int32
	// 方向光方向 uniform 位置
	dir2DLocation int32
	// 方向光偏移 uniform 位置
	dirOffsetLocation int32
	// 方向光柔和范围 uniform 位置
	dirSoftnessLocation int32
	// 正午混合强度 uniform 位置
	middayBlendLocation int32
}

// 创建光照 pass
func newLightingPass(glCtx gl.Context, logicalSize mgl32.Vec2, shader *shaderProgram) (*lightingPass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	if shader == nil {
		return nil, errors.New("lighting shader is nil")
	}

	if logicalSize.X() <= 0.0 || logicalSize.Y() <= 0.0 {
		return nil, errors.New("logical size is invalid")
	}

	pass := &lightingPass{
		glCtx:   glCtx,
		shader:  shader,
		size:    logicalSize,
		enabled: true,
	}

	if err := pass.init(); err != nil {
		pass.clean()
		return nil, err
	}

	return pass, nil
}

// 初始化 FBO、颜色纹理、quad 缓冲和 uniform 位置
func (p *lightingPass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.lightColorLocation = p.shader.uniformLocation("uLightColor")
	p.lightIntensityLocation = p.shader.uniformLocation("uLightIntensity")
	p.lightTypeLocation = p.shader.uniformLocation("uLightType")
	p.spotDirLocation = p.shader.uniformLocation("uSpotDir")
	p.spotInnerCosLocation = p.shader.uniformLocation("uSpotInnerCos")
	p.spotOuterCosLocation = p.shader.uniformLocation("uSpotOuterCos")
	p.dir2DLocation = p.shader.uniformLocation("uDir2D")
	p.dirOffsetLocation = p.shader.uniformLocation("uDirOffset")
	p.dirSoftnessLocation = p.shader.uniformLocation("uDirSoftness")
	p.middayBlendLocation = p.shader.uniformLocation("uMiddayBlend")

	if p.viewProjLocation < 0 || p.lightColorLocation < 0 ||
		p.lightIntensityLocation < 0 || p.lightTypeLocation < 0 ||
		p.spotDirLocation < 0 || p.spotInnerCosLocation < 0 ||
		p.spotOuterCosLocation < 0 || p.dir2DLocation < 0 ||
		p.dirOffsetLocation < 0 || p.dirSoftnessLocation < 0 ||
		p.middayBlendLocation < 0 {
		return errors.New("lighting pass uniform location is invalid")
	}

	if err := p.createBuffers(); err != nil {
		return err
	}

	framebuffer := p.glCtx.CreateFramebuffer()
	if framebuffer == 0 {
		return fmt.Errorf("create lighting framebuffer failed,%v", p.glCtx.GetError())
	}

	textureID := p.glCtx.CreateTexture()
	if textureID == 0 {
		p.glCtx.DeleteFramebuffer(framebuffer)
		return fmt.Errorf("create lighting color texture failed,%v", p.glCtx.GetError())
	}

	width := int32(p.size.X())
	height := int32(p.size.Y())
	p.glCtx.BindTexture(gl.TEXTURE_2D, textureID)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	p.glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	p.glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(gl.RGBA), width, height, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	p.glCtx.BindTexture(gl.TEXTURE_2D, 0)

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, framebuffer)
	p.glCtx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, textureID, 0)
	status := p.glCtx.CheckFramebufferStatus(gl.FRAMEBUFFER)
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	if status != gl.FRAMEBUFFER_COMPLETE {
		p.glCtx.DeleteTexture(textureID)
		p.glCtx.DeleteFramebuffer(framebuffer)
		return fmt.Errorf("lighting framebuffer is incomplete,%d", status)
	}

	p.framebuffer = framebuffer
	p.colorTexture = &Texture{
		glCtx:  p.glCtx,
		id:     textureID,
		width:  width,
		height: height,
		path:   "lighting-pass-color",
	}
	return nil
}

// 设置是否启用光照合成
func (p *lightingPass) setEnabled(enabled bool) {
	if p == nil {
		return
	}
	p.enabled = enabled
}

// 提交点光源
func (p *lightingPass) queuePointLight(position mgl32.Vec2, radius float32, color mgl32.Vec4, intensity float32) error {
	if p == nil {
		return errors.New("lighting pass is nil")
	}

	if radius <= 0.0 || intensity <= 0.0 {
		return nil
	}

	p.commands = append(p.commands, lightCommand{
		lightType: lightTypePoint,
		position:  position,
		radius:    radius,
		color:     color,
		intensity: intensity,
	})
	return nil
}

// 提交聚光灯
//
// direction 使用 logical 坐标方向，传入 shader 前会转换成局部 UV 方向
func (p *lightingPass) queueSpotLight(position mgl32.Vec2, radius float32, direction mgl32.Vec2, color mgl32.Vec4, intensity float32, innerAngleDeg float32, outerAngleDeg float32) error {
	if p == nil {
		return errors.New("lighting pass is nil")
	}

	if radius <= 0.0 || intensity <= 0.0 {
		return nil
	}

	innerCos := float32(math.Cos(float64(innerAngleDeg) * math.Pi / 180.0))
	outerCos := float32(math.Cos(float64(outerAngleDeg) * math.Pi / 180.0))
	if innerCos < outerCos {
		innerCos, outerCos = outerCos, innerCos
	}

	spotDir := emath.SafeNormalizeVec2(mgl32.Vec2{direction.X(), -direction.Y()}, mgl32.Vec2{0.0, -1.0})
	p.commands = append(p.commands, lightCommand{
		lightType:    lightTypeSpot,
		position:     position,
		radius:       radius,
		color:        color,
		intensity:    intensity,
		spotDir:      spotDir,
		spotInnerCos: innerCos,
		spotOuterCos: outerCos,
	})
	return nil
}

// 提交屏幕空间方向光
//
// direction 使用 logical 坐标方向，传入 shader 前会转换成局部 UV 方向
func (p *lightingPass) queueDirectionalLight(direction mgl32.Vec2, color mgl32.Vec4, intensity float32, offset float32, softness float32, middayBlend float32) error {
	if p == nil {
		return errors.New("lighting pass is nil")
	}

	if intensity <= 0.0 {
		return nil
	}

	dir2D := mgl32.Vec2{direction.X(), -direction.Y()}
	p.commands = append(p.commands, lightCommand{
		lightType:   lightTypeDirectional,
		color:       color,
		intensity:   intensity,
		dir2D:       dir2D,
		dirOffset:   mgl32.Clamp(offset, 0.0, 1.0),
		dirSoftness: mgl32.Clamp(softness, 0.0001, 0.49),
		middayBlend: mgl32.Clamp(middayBlend, 0.0, 1.0),
	})
	return nil
}

// 清空光照离屏缓冲
func (p *lightingPass) clear() {
	if p == nil || p.glCtx == nil || p.framebuffer == 0 {
		return
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, p.framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	// 没有任何额外光照
	p.glCtx.ClearColor(0.0, 0.0, 0.0, 1.0)
	p.glCtx.Clear(gl.COLOR_BUFFER_BIT)
}

// 将本帧光源累积到 light color
func (p *lightingPass) render() error {
	if p == nil || p.glCtx == nil || p.framebuffer == 0 {
		return nil
	}

	if !p.enabled {
		p.commands = p.commands[:0]
		return nil
	}

	if len(p.commands) == 0 {
		p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
		return nil
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, p.framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	p.shader.use()
	p.glCtx.BindVertexArray(p.vao)
	p.glCtx.BindBuffer(gl.ARRAY_BUFFER, p.vbo)
	p.glCtx.Enable(gl.BLEND)
	// RGB 和 Alpha 都用加法公式
	// 最终颜色 = 源颜色 * 源系数 + 目标颜色 * 目标系数
	p.glCtx.BlendEquationSeparate(gl.FUNC_ADD, gl.FUNC_ADD)
	// 设置混合系数,  RGB源系数1, RGB目标系数1, Alpha源系数1, Alpha目标系数1
	p.glCtx.BlendFuncSeparate(gl.ONE, gl.ONE, gl.ONE, gl.ONE)

	for _, command := range p.commands {
		p.applyCommandUniforms(command)
		if err := p.drawCommand(command); err != nil {
			p.restoreState()
			return err
		}
	}

	p.commands = p.commands[:0]
	p.restoreState()
	return nil
}

// 返回光照输出纹理
func (p *lightingPass) texture() *Texture {
	if p == nil || !p.enabled {
		return nil
	}
	return p.colorTexture
}

// 释放光照缓冲资源
func (p *lightingPass) clean() {
	if p == nil {
		return
	}

	if p.colorTexture != nil {
		p.colorTexture.Close()
		p.colorTexture = nil
	}
	if p.glCtx != nil && p.ebo != 0 {
		p.glCtx.DeleteBuffer(p.ebo)
		p.ebo = 0
	}
	if p.glCtx != nil && p.vbo != 0 {
		p.glCtx.DeleteBuffer(p.vbo)
		p.vbo = 0
	}
	if p.glCtx != nil && p.vao != 0 {
		p.glCtx.DeleteVertexArray(p.vao)
		p.vao = 0
	}
	if p.glCtx != nil && p.framebuffer != 0 {
		p.glCtx.DeleteFramebuffer(p.framebuffer)
		p.framebuffer = 0
	}
	p.shader = nil
	p.commands = nil
}

// 创建光源 quad 使用的缓冲
func (p *lightingPass) createBuffers() error {
	p.vao = p.glCtx.CreateVertexArray()
	p.vbo = p.glCtx.CreateBuffer()
	p.ebo = p.glCtx.CreateBuffer()
	if p.vao == 0 || p.vbo == 0 || p.ebo == 0 {
		return errors.New("create lighting pass buffers failed")
	}

	p.glCtx.BindVertexArray(p.vao)
	p.glCtx.BindBuffer(gl.ARRAY_BUFFER, p.vbo)
	p.glCtx.BufferInit(gl.ARRAY_BUFFER, lightQuadVertexCount*lightQuadFloatCountPerVertex*float32Size, gl.DYNAMIC_DRAW)
	p.glCtx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, p.ebo)
	p.glCtx.BufferInit(gl.ELEMENT_ARRAY_BUFFER, lightQuadIndexCount*uint32Size, gl.DYNAMIC_DRAW)
	p.glCtx.BufferSubData(gl.ELEMENT_ARRAY_BUFFER, 0, utils.Uint32Bytes([]uint32{0, 1, 2, 2, 3, 0}))
	p.glCtx.VertexAttribPointer(0, 2, gl.FLOAT, false, int32(lightQuadFloatCountPerVertex*float32Size), 0)
	p.glCtx.EnableVertexAttribArray(0)
	p.glCtx.VertexAttribPointer(1, 2, gl.FLOAT, false, int32(lightQuadFloatCountPerVertex*float32Size), lightQuadUVOffsetFloatCount*float32Size)
	p.glCtx.EnableVertexAttribArray(1)
	p.glCtx.BindVertexArray(0)
	return nil
}

// 设置当前光源命令的 shader 参数
func (p *lightingPass) applyCommandUniforms(command lightCommand) {
	p.glCtx.Uniform3fv(p.lightColorLocation, []float32{command.color.X(), command.color.Y(), command.color.Z()})
	p.glCtx.Uniform1fv(p.lightIntensityLocation, []float32{command.intensity})
	p.glCtx.Uniform1i(p.lightTypeLocation, command.lightType)
	p.glCtx.Uniform2fv(p.spotDirLocation, []float32{command.spotDir.X(), command.spotDir.Y()})
	p.glCtx.Uniform1fv(p.spotInnerCosLocation, []float32{command.spotInnerCos})
	p.glCtx.Uniform1fv(p.spotOuterCosLocation, []float32{command.spotOuterCos})
	p.glCtx.Uniform2fv(p.dir2DLocation, []float32{command.dir2D.X(), command.dir2D.Y()})
	p.glCtx.Uniform1fv(p.dirOffsetLocation, []float32{command.dirOffset})
	p.glCtx.Uniform1fv(p.dirSoftnessLocation, []float32{command.dirSoftness})
	p.glCtx.Uniform1fv(p.middayBlendLocation, []float32{command.middayBlend})
}

// 绘制一条光源命令
func (p *lightingPass) drawCommand(command lightCommand) error {
	var rect mgl32.Vec4
	viewProj := mgl32.Ortho(0, p.size.X(), p.size.Y(), 0, -1, 1)
	if command.lightType == lightTypeDirectional {
		// 如果是方向光, 就画满整个光照缓冲
		rect = mgl32.Vec4{0.0, 0.0, p.size.X(), p.size.Y()}
		// 这段代码是为了消除屏幕宽高比对方向的扭曲影响，保证渐变方向/亮度增加方向在任何分辨率下角度都正确、不变形
		// 处理屏幕不是正方形的问题, 确保方向光方向正确
		fixedDir := mgl32.Vec2{
			command.dir2D.X() * p.size.Y(),
			command.dir2D.Y() * p.size.X(),
		}
		normalizedDir := emath.SafeNormalizeVec2(fixedDir, mgl32.Vec2{0.0, 1.0})
		p.glCtx.Uniform2fv(p.dir2DLocation, []float32{
			normalizedDir.X(),
			normalizedDir.Y(),
		})
	} else {
		// 如果是点光或聚光, 矩形中心是光源位置, 边长是光源半径的2倍
		rect = mgl32.Vec4{
			command.position.X() - command.radius,
			command.position.Y() - command.radius,
			command.radius * 2.0,
			command.radius * 2.0,
		}
	}

	if rect.Z() <= 0.0 || rect.W() <= 0.0 {
		return nil
	}

	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	vertices := []float32{
		// 左上
		rect.X(), rect.Y(), 0.0, 1.0,
		// 右上
		rect.X() + rect.Z(), rect.Y(), 1.0, 1.0,
		// 右下
		rect.X() + rect.Z(), rect.Y() + rect.W(), 1.0, 0.0,
		// 左下
		rect.X(), rect.Y() + rect.W(), 0.0, 0.0,
	}
	p.glCtx.BufferSubData(gl.ARRAY_BUFFER, 0, utils.Float32Bytes(vertices))
	p.glCtx.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, 0)
	return nil
}

// 恢复后续 pass 使用的默认 OpenGL 状态
func (p *lightingPass) restoreState() {
	p.glCtx.BindVertexArray(0)
	p.glCtx.UseProgram(0)
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	//  RGB 和 Alpha 都用加法公式
	// 结果 = 源 * 源系数 + 目标 * 目标系数
	p.glCtx.BlendEquationSeparate(gl.FUNC_ADD, gl.FUNC_ADD)
	// out.rgb = src.rgb * src.a + dst.rgb * (1 - src.a)
	// out.a = src.a * 1 + dst.a * (1 - src.a)
	p.glCtx.BlendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
}
