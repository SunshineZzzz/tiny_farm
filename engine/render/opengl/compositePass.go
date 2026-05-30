package opengl

import (
	"errors"

	emath "tiny_farm/engine/utils/math"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理最终合成输出
//
// 当前阶段合成 scene、lighting、emissive 和 bloom，后续继续从这里扩展输出控制
type compositePass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 合成用着色器
	shader *shaderProgram
	// 全屏合成批处理器
	batch *spriteBatch
	// 光照输入缺省白纹理，表示全亮
	defaultLightTexture *Texture
	// 自发光输入缺省黑纹理，表示没有自发光贡献
	defaultEmissiveTexture *Texture
	// Bloom 输入缺省黑纹理，表示没有辉光贡献
	defaultBloomTexture *Texture
	// 环境光颜色，RGB 表示基础亮度
	ambientColor mgl32.Vec4
	// Bloom 合成强度
	bloomStrength float32
	// 视图投影 uniform 位置
	viewProjLocation int32
	// 场景纹理采样器 uniform 位置
	sceneTextureLocation int32
	// 光照纹理采样器 uniform 位置
	lightTextureLocation int32
	// 自发光纹理采样器 uniform 位置
	emissiveTextureLocation int32
	// Bloom 纹理采样器 uniform 位置
	bloomTextureLocation int32
	// 环境光 uniform 位置
	ambientLocation int32
	// Bloom 强度 uniform 位置
	bloomStrengthLocation int32
	// 上一帧统计
	stats PassStats
}

// 合成输入纹理集合
//
// sceneColor 是必需输入，lightColor 和 emissiveColor 有默认纹理兜底
type compositePassInput struct {
	// ScenePass 输出的颜色纹理
	sceneColor *Texture
	// LightingPass 输出的光照纹理，为 nil 时使用全亮白纹理
	lightColor *Texture
	// EmissivePass 输出的自发光纹理，为 nil 时使用全黑纹理
	emissiveColor *Texture
	// BloomPass 输出的辉光纹理，为 nil 时使用全黑纹理
	bloomColor *Texture
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
		glCtx:         glCtx,
		shader:        shader,
		ambientColor:  mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
		bloomStrength: 1.0,
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

	lightColor := input.lightColor
	if lightColor == nil {
		lightColor = p.defaultLightTexture
	}
	if lightColor == nil || lightColor.id == 0 {
		return errors.New("composite light texture is nil")
	}

	emissiveColor := input.emissiveColor
	if emissiveColor == nil {
		emissiveColor = p.defaultEmissiveTexture
	}
	if emissiveColor == nil || emissiveColor.id == 0 {
		return errors.New("composite emissive texture is nil")
	}

	bloomColor := input.bloomColor
	if bloomColor == nil {
		bloomColor = p.defaultBloomTexture
	}
	if bloomColor == nil || bloomColor.id == 0 {
		return errors.New("composite bloom texture is nil")
	}

	// 绑定的是 默认帧缓冲，也就是OpenGL当前上下文关联的窗口后缓冲区
	// 它的尺寸通常等于 窗口drawable size / framebuffer size，不是一定等于SDL的窗口逻辑大小
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	// 接下来NDC坐标[-1,1]最终映射到窗口的哪个像素区域
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

	p.stats = passStatsFromBatch(true, p.batch.stats())
	p.shader.use()
	viewProj := mgl32.Ortho(0, viewport.Size.X(), viewport.Size.Y(), 0, -1, 1)
	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	// 第1, 2, 3号纹理单元
	const lightTextureUnit = gl.TEXTURE0 + 1
	const emissiveTextureUnit = gl.TEXTURE0 + 2
	const bloomTextureUnit = gl.TEXTURE0 + 3
	// uLightColor使用纹理单元1, uEmissiveColor使用纹理单元2, uBloomColor使用纹理单元3
	p.glCtx.Uniform1i(p.lightTextureLocation, 1)
	p.glCtx.Uniform1i(p.emissiveTextureLocation, 2)
	p.glCtx.Uniform1i(p.bloomTextureLocation, 3)
	p.glCtx.Uniform3fv(p.ambientLocation, []float32{
		p.ambientColor.X(),
		p.ambientColor.Y(),
		p.ambientColor.Z(),
	})
	p.glCtx.Uniform1fv(p.bloomStrengthLocation, []float32{p.bloomStrength})
	// 激活纹理单元1
	p.glCtx.ActiveTexture(lightTextureUnit)
	// 绑定光照纹理到纹理单元1
	p.glCtx.BindTexture(gl.TEXTURE_2D, lightColor.id)
	// 激活纹理单元2
	p.glCtx.ActiveTexture(emissiveTextureUnit)
	// 绑定自发光纹理到纹理单元2
	p.glCtx.BindTexture(gl.TEXTURE_2D, emissiveColor.id)
	// 激活纹理单元3
	p.glCtx.ActiveTexture(bloomTextureUnit)
	// 绑定 Bloom 纹理到纹理单元3
	p.glCtx.BindTexture(gl.TEXTURE_2D, bloomColor.id)

	// 收尾清理
	defer func() {
		// 切回GL_TEXTURE1
		p.glCtx.ActiveTexture(lightTextureUnit)
		// 把GL_TEXTURE1上绑定的light texture解绑
		p.glCtx.BindTexture(gl.TEXTURE_2D, 0)
		// 切回GL_TEXTURE2
		p.glCtx.ActiveTexture(emissiveTextureUnit)
		// 把GL_TEXTURE2上绑定的emissive texture解绑
		p.glCtx.BindTexture(gl.TEXTURE_2D, 0)
		// 切回GL_TEXTURE3
		p.glCtx.ActiveTexture(bloomTextureUnit)
		// 把GL_TEXTURE3上绑定的bloom texture解绑
		p.glCtx.BindTexture(gl.TEXTURE_2D, 0)
		// 再把当前活动纹理单元恢复到GL_TEXTURE0
		p.glCtx.ActiveTexture(gl.TEXTURE0)
	}()

	if err := p.batch.flush(p.sceneTextureLocation, -1); err != nil {
		p.glCtx.UseProgram(0)
		return err
	}
	p.glCtx.UseProgram(0)
	return nil
}

// 返回上一帧合成 pass 统计
func (p *compositePass) renderStats() PassStats {
	if p == nil {
		return PassStats{}
	}
	return p.stats
}

// 设置环境光颜色
func (p *compositePass) setAmbientColor(color mgl32.Vec4) {
	if p == nil {
		return
	}
	p.ambientColor = mgl32.Vec4{color.X(), color.Y(), color.Z(), 1.0}
}

// 设置 Bloom 合成强度
func (p *compositePass) setBloomStrength(strength float32) {
	if p == nil {
		return
	}
	p.bloomStrength = max(strength, 0.0)
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
	if p.defaultLightTexture != nil {
		p.defaultLightTexture.Close()
		p.defaultLightTexture = nil
	}
	if p.defaultEmissiveTexture != nil {
		p.defaultEmissiveTexture.Close()
		p.defaultEmissiveTexture = nil
	}
	if p.defaultBloomTexture != nil {
		p.defaultBloomTexture.Close()
		p.defaultBloomTexture = nil
	}
	p.shader = nil
	p.viewProjLocation = -1
	p.sceneTextureLocation = -1
	p.lightTextureLocation = -1
	p.emissiveTextureLocation = -1
	p.bloomTextureLocation = -1
	p.ambientLocation = -1
	p.bloomStrengthLocation = -1
	p.stats = PassStats{}
}

// 初始化合成 uniform 和批处理资源
func (p *compositePass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.sceneTextureLocation = p.shader.uniformLocation("uSceneColor")
	p.lightTextureLocation = p.shader.uniformLocation("uLightColor")
	p.ambientLocation = p.shader.uniformLocation("uAmbient")
	p.emissiveTextureLocation = p.shader.uniformLocation("uEmissiveColor")
	p.bloomTextureLocation = p.shader.uniformLocation("uBloomColor")
	p.bloomStrengthLocation = p.shader.uniformLocation("uBloomStrength")

	if p.viewProjLocation < 0 || p.sceneTextureLocation < 0 ||
		p.lightTextureLocation < 0 || p.ambientLocation < 0 ||
		p.emissiveTextureLocation < 0 || p.bloomTextureLocation < 0 ||
		p.bloomStrengthLocation < 0 {
		return errors.New("composite pass uniform location is invalid")
	}

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	defaultLightTexture, err := newSolidTexture(p.glCtx, [4]byte{255, 255, 255, 255}, "composite-default-light")
	if err != nil {
		return err
	}
	p.defaultLightTexture = defaultLightTexture

	defaultEmissiveTexture, err := newSolidTexture(p.glCtx, [4]byte{0, 0, 0, 255}, "composite-default-emissive")
	if err != nil {
		return err
	}
	p.defaultEmissiveTexture = defaultEmissiveTexture

	defaultBloomTexture, err := newSolidTexture(p.glCtx, [4]byte{0, 0, 0, 255}, "composite-default-bloom")
	if err != nil {
		return err
	}
	p.defaultBloomTexture = defaultBloomTexture

	return nil
}

// 创建单像素纯色纹理，用于 pass 输入缺省值
func newSolidTexture(glCtx gl.Context, pixel [4]byte, path string) (*Texture, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	textureID := glCtx.CreateTexture()
	if textureID == 0 {
		return nil, errors.New("create solid texture failed")
	}

	glCtx.BindTexture(gl.TEXTURE_2D, textureID)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(gl.RGBA), 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, pixel[:])
	glCtx.BindTexture(gl.TEXTURE_2D, 0)

	return &Texture{
		glCtx:  glCtx,
		id:     textureID,
		width:  1,
		height: 1,
		path:   path,
	}, nil
}
