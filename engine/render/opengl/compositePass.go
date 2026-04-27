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
func newCompositePass(glCtx gl.Context) (*compositePass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	pass := &compositePass{
		glCtx: glCtx,
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
	if p.shader != nil {
		p.shader.clean()
		p.shader = nil
	}
	p.viewProjLocation = -1
	p.sceneTextureLocation = -1
	p.useTextureLocation = -1
}

// 初始化合成着色器和批处理资源
func (p *compositePass) init() error {
	const vertexShaderSource = `
	#version 330 core
	layout(location = 0) in vec2 aPos;
	layout(location = 1) in vec2 aUV;
	layout(location = 2) in vec4 aColor;

	uniform mat4 uViewProj;

	out vec2 vUV;
	out vec4 vColor;

	void main() {
		vUV = aUV;
		vColor = aColor;
		gl_Position = uViewProj * vec4(aPos, 0.0, 1.0);
	}
	`

	const fragmentShaderSource = `
	#version 330 core
	in vec2 vUV;
	in vec4 vColor;

	uniform sampler2D uSceneColor;
	uniform bool uUseTexture;

	out vec4 FragColor;

	void main() {
		if (uUseTexture) {
			FragColor = texture(uSceneColor, vUV) * vColor;
		} else {
			FragColor = vColor;
		}
	}
	`

	shader, err := newShaderProgram(p.glCtx, vertexShaderSource, fragmentShaderSource)
	if err != nil {
		return err
	}
	p.shader = shader
	p.viewProjLocation = shader.uniformLocation("uViewProj")
	p.sceneTextureLocation = shader.uniformLocation("uSceneColor")
	p.useTextureLocation = shader.uniformLocation("uUseTexture")

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	return nil
}
