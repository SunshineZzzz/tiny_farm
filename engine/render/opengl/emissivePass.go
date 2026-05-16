package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理世界层自发光离屏渲染目标
//
// 当前阶段把自发光矩形和贴图绘制到独立的 logical size FBO，最终由 CompositePass 加到受光后的场景颜色上
type emissivePass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 自发光绘制着色器
	shader *shaderProgram
	// 自发光侧独立批处理器
	batch *spriteBatch
	// 离屏 framebuffer
	framebuffer uint32
	// 自发光颜色纹理
	colorTexture *Texture
	// 离屏缓冲尺寸，固定等于逻辑分辨率
	size mgl32.Vec2
	// 是否启用自发光合成
	enabled bool
	// 上一帧统计
	stats PassStats
	// 视图投影 uniform 位置
	viewProjLocation int32
	// 自发光纹理采样器 uniform 位置
	textureLocation int32
	// 批处理是否采样纹理 uniform 位置
	useTextureLocation int32
}

// 创建自发光离屏渲染目标
func newEmissivePass(glCtx gl.Context, logicalSize mgl32.Vec2, shader *shaderProgram) (*emissivePass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	if logicalSize.X() <= 0.0 || logicalSize.Y() <= 0.0 {
		return nil, errors.New("logical size is invalid")
	}

	if shader == nil {
		return nil, errors.New("emissive shader is nil")
	}

	pass := &emissivePass{
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

// 设置是否启用自发光 pass
func (p *emissivePass) setEnabled(enabled bool) {
	if p == nil {
		return
	}
	p.enabled = enabled
}

// 将纯色自发光矩形加入队列
func (p *emissivePass) queueRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if p == nil || p.batch == nil {
		return errors.New("emissive pass is nil")
	}
	return p.batch.queueRect(rect, color)
}

// 将贴图自发光矩形加入队列
func (p *emissivePass) queueTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if p == nil || p.batch == nil {
		return errors.New("emissive pass is nil")
	}
	return p.batch.queueTexture(texture, dstRect, uvRect, color)
}

// 清空自发光离屏缓冲
func (p *emissivePass) clear() {
	if p == nil || p.glCtx == nil || p.framebuffer == 0 {
		return
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, p.framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	p.glCtx.ClearColor(0.0, 0.0, 0.0, 1.0)
	p.glCtx.Clear(gl.COLOR_BUFFER_BIT)
}

// 将自发光队列绘制到 logical size FBO
func (p *emissivePass) render() error {
	if p == nil || p.glCtx == nil || p.framebuffer == 0 || p.shader == nil || p.batch == nil {
		return nil
	}

	if !p.enabled {
		p.stats = PassStats{Enabled: false}
		p.batch.reset()
		return nil
	}
	p.stats = passStatsFromBatch(true, p.batch.stats())

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, p.framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	p.glCtx.Enable(gl.BLEND)
	p.glCtx.BlendEquationSeparate(gl.FUNC_ADD, gl.FUNC_ADD)
	p.glCtx.BlendFuncSeparate(gl.ONE, gl.ONE, gl.ONE, gl.ONE)

	p.shader.use()
	viewProj := mgl32.Ortho(0, p.size.X(), p.size.Y(), 0, -1, 1)
	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	if err := p.batch.flush(p.textureLocation, p.useTextureLocation); err != nil {
		p.restoreState()
		return err
	}
	p.restoreState()
	return nil
}

// 返回自发光输出纹理
func (p *emissivePass) texture() *Texture {
	if p == nil || !p.enabled {
		return nil
	}
	return p.colorTexture
}

// 返回上一帧自发光 pass 统计
func (p *emissivePass) renderStats() PassStats {
	if p == nil {
		return PassStats{}
	}
	return p.stats
}

// 释放自发光 pass 的 OpenGL 资源
func (p *emissivePass) clean() {
	if p == nil {
		return
	}

	if p.colorTexture != nil {
		p.colorTexture.Close()
		p.colorTexture = nil
	}
	if p.batch != nil {
		p.batch.clean()
		p.batch = nil
	}
	p.shader = nil
	if p.glCtx != nil && p.framebuffer != 0 {
		p.glCtx.DeleteFramebuffer(p.framebuffer)
		p.framebuffer = 0
	}
	p.viewProjLocation = -1
	p.textureLocation = -1
	p.useTextureLocation = -1
	p.stats = PassStats{}
}

// 恢复后续 pass 使用的默认 OpenGL 状态
func (p *emissivePass) restoreState() {
	if p == nil || p.glCtx == nil {
		return
	}

	p.glCtx.UseProgram(0)
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	p.glCtx.BlendEquationSeparate(gl.FUNC_ADD, gl.FUNC_ADD)
	p.glCtx.BlendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
}

// 初始化 FBO、颜色纹理、uniform 和批处理资源
func (p *emissivePass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.textureLocation = p.shader.uniformLocation("uTexture")
	p.useTextureLocation = p.shader.uniformLocation("uUseTexture")

	if p.viewProjLocation < 0 || p.textureLocation < 0 || p.useTextureLocation < 0 {
		return errors.New("emissive pass uniform location is invalid")
	}

	framebuffer := p.glCtx.CreateFramebuffer()
	if framebuffer == 0 {
		return fmt.Errorf("create emissive framebuffer failed,%v", p.glCtx.GetError())
	}

	textureID := p.glCtx.CreateTexture()
	if textureID == 0 {
		p.glCtx.DeleteFramebuffer(framebuffer)
		return fmt.Errorf("create emissive color texture failed,%v", p.glCtx.GetError())
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
		return fmt.Errorf("emissive framebuffer is incomplete,%d", status)
	}

	p.framebuffer = framebuffer
	p.colorTexture = &Texture{
		glCtx:  p.glCtx,
		id:     textureID,
		width:  width,
		height: height,
		path:   "emissive-pass-color",
	}

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	return nil
}
