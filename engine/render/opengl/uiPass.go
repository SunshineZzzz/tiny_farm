package opengl

import (
	"errors"

	emath "tiny_farm/engine/utils/math"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理 UI 层最终绘制
//
// 当前阶段只提供独立批处理边界，UI 使用 logical 坐标并绘制到默认帧缓冲的 letterbox viewport
type uiPass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// UI 绘制着色器
	shader *shaderProgram
	// UI 侧独立批处理器
	batch *spriteBatch
	// 视图投影 uniform 位置
	viewProjLocation int32
	// UI 纹理采样器 uniform 位置
	textureLocation int32
	// 批处理是否采样纹理 uniform 位置
	useTextureLocation int32
}

// 创建 UI pass
func newUIPass(glCtx gl.Context, shader *shaderProgram) (*uiPass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}
	if shader == nil {
		return nil, errors.New("ui shader is nil")
	}

	pass := &uiPass{
		glCtx:  glCtx,
		shader: shader,
	}
	if err := pass.init(); err != nil {
		pass.clean()
		return nil, err
	}
	return pass, nil
}

// 将纯色矩形加入 UI 队列
func (p *uiPass) queueRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if p == nil || p.batch == nil {
		return errors.New("ui pass is nil")
	}
	return p.batch.queueRect(rect, color)
}

// 将贴图矩形加入 UI 队列
func (p *uiPass) queueTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	if p == nil || p.batch == nil {
		return errors.New("ui pass is nil")
	}
	return p.batch.queueTexture(texture, dstRect, uvRect, mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
}

// 将 UI 队列绘制到默认帧缓冲的 letterbox viewport
func (p *uiPass) render(viewport emath.Rect, logicalSize mgl32.Vec2) error {
	if p == nil || p.glCtx == nil || p.shader == nil || p.batch == nil {
		return nil
	}

	if viewport.Size.X() <= 0 || viewport.Size.Y() <= 0 {
		return errors.New("ui viewport size is invalid")
	}
	if logicalSize.X() <= 0 || logicalSize.Y() <= 0 {
		return errors.New("ui logical size is invalid")
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	p.glCtx.Viewport(
		int32(viewport.Position.X()),
		int32(viewport.Position.Y()),
		int32(viewport.Size.X()),
		int32(viewport.Size.Y()),
	)

	p.shader.use()
	viewProj := mgl32.Ortho(0, logicalSize.X(), logicalSize.Y(), 0, -1, 1)
	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	if err := p.batch.flush(p.textureLocation, p.useTextureLocation); err != nil {
		p.glCtx.UseProgram(0)
		return err
	}
	p.glCtx.UseProgram(0)
	return nil
}

// 释放 UI pass 的 OpenGL 资源
func (p *uiPass) clean() {
	if p == nil {
		return
	}

	if p.batch != nil {
		p.batch.clean()
		p.batch = nil
	}
	p.shader = nil
	p.viewProjLocation = -1
	p.textureLocation = -1
	p.useTextureLocation = -1
}

// 初始化 UI uniform 和批处理资源
func (p *uiPass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.textureLocation = p.shader.uniformLocation("uTexture")
	p.useTextureLocation = p.shader.uniformLocation("uUseTexture")

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	return nil
}
