package opengl

import (
	"errors"

	emath "tiny_farm/engine/utils/math"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理最终合成输出
//
// 当前阶段只把 ScenePass 输出的 scene color 贴回默认帧缓冲
// 后续 Lighting、Emissive、Bloom 都从这里扩展输入纹理
type compositePass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 合成用着色器
	shader *shaderProgram
	// 全屏合成批处理器
	batch *spriteBatch
	// 视图投影 uniform 位置
	viewProjLocation int32
	// 场景纹理采样器 uniform 位置
	sceneTextureLocation int32
	// 批处理是否采样纹理 uniform 位置
	useTextureLocation int32
}

// 合成输入纹理集合
//
// 当前只要求 sceneColor，其他 pass 输出会在后续阶段接入
type compositePassInput struct {
	// ScenePass 输出的颜色纹理
	sceneColor *Texture
}

// 创建最终合成 pass
func newCompositePass(glCtx gl.Context, shader *shaderProgram) (*compositePass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}
	if shader == nil {
		return nil, errors.New("composite shader is nil")
	}

	pass := &compositePass{
		glCtx:  glCtx,
		shader: shader,
	}
	if err := pass.init(); err != nil {
		pass.clean()
		return nil, err
	}
	return pass, nil
}

// 将输入纹理合成到默认帧缓冲的 letterbox viewport
func (p *compositePass) render(viewport emath.Rect, input compositePassInput) error {
	if p == nil || p.glCtx == nil || p.shader == nil || p.batch == nil {
		return nil
	}

	if input.sceneColor == nil {
		return errors.New("composite scene color texture is nil")
	}

	if viewport.Size.X() <= 0 || viewport.Size.Y() <= 0 {
		return errors.New("composite viewport size is invalid")
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	p.glCtx.Viewport(
		int32(viewport.Position.X()),
		int32(viewport.Position.Y()),
		int32(viewport.Size.X()),
		int32(viewport.Size.Y()),
	)

	if err := p.batch.queueTexture(
		input.sceneColor,
		mgl32.Vec4{0.0, 0.0, viewport.Size.X(), viewport.Size.Y()},
		mgl32.Vec4{0.0, 0.0, 1.0, 1.0},
		mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
	); err != nil {
		return err
	}

	p.shader.use()
	viewProj := mgl32.Ortho(0, viewport.Size.X(), viewport.Size.Y(), 0, -1, 1)
	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	if err := p.batch.flush(p.sceneTextureLocation, p.useTextureLocation); err != nil {
		p.glCtx.UseProgram(0)
		return err
	}
	p.glCtx.UseProgram(0)
	return nil
}

// 释放合成 pass 的 OpenGL 资源
func (p *compositePass) clean() {
	if p == nil {
		return
	}

	if p.batch != nil {
		p.batch.clean()
		p.batch = nil
	}
	p.shader = nil
	p.viewProjLocation = -1
	p.sceneTextureLocation = -1
	p.useTextureLocation = -1
}

// 初始化合成 uniform 和批处理资源
func (p *compositePass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.sceneTextureLocation = p.shader.uniformLocation("uSceneColor")
	p.useTextureLocation = p.shader.uniformLocation("uUseTexture")

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	return nil
}
