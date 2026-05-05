package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理自发光输入的最小 Bloom 后处理
//
// 当前阶段只使用一组 ping-pong FBO 做横向和纵向两次模糊，后续再扩展为多级降采样链路
type bloomPass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// Bloom 模糊着色器
	shader *shaderProgram
	// 全屏贴图批处理器
	batch *spriteBatch
	// 横向模糊 framebuffer
	horizontalFramebuffer uint32
	// 纵向模糊 framebuffer
	verticalFramebuffer uint32
	// 横向模糊中间纹理
	horizontalTexture *Texture
	// 最终 Bloom 输出纹理
	verticalTexture *Texture
	// 后处理缓冲尺寸，固定等于逻辑分辨率
	size mgl32.Vec2
	// 是否启用 Bloom 后处理
	enabled bool
	// 视图投影 uniform 位置
	viewProjLocation int32
	// 输入纹理采样器 uniform 位置
	textureLocation int32
	// 输入纹理像素尺寸倒数 uniform 位置
	texelSizeLocation int32
	// 模糊方向 uniform 位置
	directionLocation int32
}

// 创建 Bloom 后处理 pass
func newBloomPass(glCtx gl.Context, logicalSize mgl32.Vec2, shader *shaderProgram) (*bloomPass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	if logicalSize.X() <= 0.0 || logicalSize.Y() <= 0.0 {
		return nil, errors.New("logical size is invalid")
	}

	if shader == nil {
		return nil, errors.New("bloom shader is nil")
	}

	pass := &bloomPass{
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

// 设置是否启用 Bloom 后处理
func (p *bloomPass) setEnabled(enabled bool) {
	if p == nil {
		return
	}
	p.enabled = enabled
}

// 清空 Bloom 后处理缓冲
func (p *bloomPass) clear() {
	if p == nil || p.glCtx == nil {
		return
	}

	p.clearFramebuffer(p.horizontalFramebuffer)
	p.clearFramebuffer(p.verticalFramebuffer)
}

// 对自发光纹理执行横纵两次模糊
func (p *bloomPass) render(input *Texture) error {
	if p == nil || p.glCtx == nil || p.shader == nil || p.batch == nil {
		return nil
	}
	if !p.enabled || input == nil || input.id == 0 {
		return nil
	}

	p.setTextureFilter(input, gl.LINEAR)
	defer p.setTextureFilter(input, gl.NEAREST)

	if err := p.renderBlurPass(input, p.horizontalFramebuffer, mgl32.Vec2{1.0, 0.0}); err != nil {
		return err
	}
	return p.renderBlurPass(p.horizontalTexture, p.verticalFramebuffer, mgl32.Vec2{0.0, 1.0})
}

// 返回 Bloom 输出纹理
func (p *bloomPass) texture() *Texture {
	if p == nil || !p.enabled {
		return nil
	}
	return p.verticalTexture
}

// 释放 Bloom pass 的 OpenGL 资源
func (p *bloomPass) clean() {
	if p == nil {
		return
	}

	if p.horizontalTexture != nil {
		p.horizontalTexture.Close()
		p.horizontalTexture = nil
	}
	if p.verticalTexture != nil {
		p.verticalTexture.Close()
		p.verticalTexture = nil
	}
	if p.batch != nil {
		p.batch.clean()
		p.batch = nil
	}
	p.shader = nil
	if p.glCtx != nil && p.horizontalFramebuffer != 0 {
		p.glCtx.DeleteFramebuffer(p.horizontalFramebuffer)
		p.horizontalFramebuffer = 0
	}
	if p.glCtx != nil && p.verticalFramebuffer != 0 {
		p.glCtx.DeleteFramebuffer(p.verticalFramebuffer)
		p.verticalFramebuffer = 0
	}
	p.viewProjLocation = -1
	p.textureLocation = -1
	p.texelSizeLocation = -1
	p.directionLocation = -1
}

// 初始化 FBO、纹理、uniform 和全屏批处理器
func (p *bloomPass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.textureLocation = p.shader.uniformLocation("uTexture")
	p.texelSizeLocation = p.shader.uniformLocation("uTexelSize")
	p.directionLocation = p.shader.uniformLocation("uDirection")

	if p.viewProjLocation < 0 || p.textureLocation < 0 ||
		p.texelSizeLocation < 0 || p.directionLocation < 0 {
		return errors.New("bloom pass uniform location is invalid")
	}

	horizontalFramebuffer, horizontalTexture, err := p.createRenderTarget("bloom-horizontal")
	if err != nil {
		return err
	}
	p.horizontalFramebuffer = horizontalFramebuffer
	p.horizontalTexture = horizontalTexture

	verticalFramebuffer, verticalTexture, err := p.createRenderTarget("bloom-vertical")
	if err != nil {
		return err
	}
	p.verticalFramebuffer = verticalFramebuffer
	p.verticalTexture = verticalTexture

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	return nil
}

func (p *bloomPass) createRenderTarget(path string) (uint32, *Texture, error) {
	framebuffer := p.glCtx.CreateFramebuffer()
	if framebuffer == 0 {
		return 0, nil, fmt.Errorf("create bloom framebuffer failed,%v", p.glCtx.GetError())
	}

	textureID := p.glCtx.CreateTexture()
	if textureID == 0 {
		p.glCtx.DeleteFramebuffer(framebuffer)
		return 0, nil, fmt.Errorf("create bloom texture failed,%v", p.glCtx.GetError())
	}

	width := int32(p.size.X())
	height := int32(p.size.Y())
	p.glCtx.BindTexture(gl.TEXTURE_2D, textureID)
	// Bloom 的 5-tap shader 依赖线性采样合并相邻采样点
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
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
		return 0, nil, fmt.Errorf("bloom framebuffer is incomplete,%d", status)
	}

	return framebuffer, &Texture{
		glCtx:  p.glCtx,
		id:     textureID,
		width:  width,
		height: height,
		path:   path,
	}, nil
}

func (p *bloomPass) renderBlurPass(input *Texture, framebuffer uint32, direction mgl32.Vec2) error {
	if input == nil || input.id == 0 || framebuffer == 0 {
		return nil
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	// 覆盖写入输出纹理，而不是和输出纹理里原来的内容混合
	p.glCtx.Disable(gl.BLEND)

	if err := p.batch.queueTexture(
		input,
		mgl32.Vec4{0.0, 0.0, p.size.X(), p.size.Y()},
		mgl32.Vec4{0.0, 0.0, 1.0, 1.0},
		mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
	); err != nil {
		p.restoreState()
		return err
	}

	p.shader.use()
	viewProj := mgl32.Ortho(0, p.size.X(), p.size.Y(), 0, -1, 1)
	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	p.glCtx.Uniform2fv(p.texelSizeLocation, []float32{1.0 / p.size.X(), 1.0 / p.size.Y()})
	p.glCtx.Uniform2fv(p.directionLocation, []float32{direction.X(), direction.Y()})
	if err := p.batch.flush(p.textureLocation, -1); err != nil {
		p.restoreState()
		return err
	}
	p.restoreState()
	return nil
}

func (p *bloomPass) setTextureFilter(texture *Texture, filter uint32) {
	if texture == nil || texture.id == 0 {
		return
	}

	p.glCtx.BindTexture(gl.TEXTURE_2D, texture.id)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, int32(filter))
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, int32(filter))
	p.glCtx.BindTexture(gl.TEXTURE_2D, 0)
}

func (p *bloomPass) clearFramebuffer(framebuffer uint32) {
	if framebuffer == 0 {
		return
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	p.glCtx.ClearColor(0.0, 0.0, 0.0, 1.0)
	p.glCtx.Clear(gl.COLOR_BUFFER_BIT)
}

func (p *bloomPass) restoreState() {
	if p == nil || p.glCtx == nil {
		return
	}

	p.glCtx.UseProgram(0)
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	p.glCtx.Enable(gl.BLEND)
	p.glCtx.BlendEquationSeparate(gl.FUNC_ADD, gl.FUNC_ADD)
	p.glCtx.BlendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
}
