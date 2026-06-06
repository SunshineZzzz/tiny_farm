package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理场景离屏渲染目标
//
// 持有场景 FBO、场景批处理和场景 shader，用于把世界内容先绘制到 logical size 的离屏纹理
type scenePass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 场景精灵着色器
	shader *shaderProgram
	// 场景精灵批处理器
	batch *spriteBatch
	// 离屏 framebuffer
	framebuffer uint32
	// 离屏颜色纹理
	colorTexture *Texture
	// 离屏缓冲尺寸，固定等于逻辑分辨率
	size mgl32.Vec2
	// 视图投影 uniform 位置
	viewProjLocation int32
	// 场景纹理采样器 uniform 位置
	textureLocation int32
	// 批处理是否采样纹理 uniform 位置
	useTextureLocation int32
	// 上一帧统计
	stats PassStats
}

// 创建场景离屏渲染目标
func newScenePass(glCtx gl.Context, logicalSize mgl32.Vec2, shader *shaderProgram) (*scenePass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}
	if logicalSize.X() <= 0.0 || logicalSize.Y() <= 0.0 {
		return nil, errors.New("logical size is invalid")
	}
	if shader == nil {
		return nil, errors.New("scene shader is nil")
	}

	pass := &scenePass{
		glCtx:  glCtx,
		shader: shader,
		size:   logicalSize,
	}
	if err := pass.init(); err != nil {
		pass.clean()
		return nil, err
	}
	return pass, nil
}

// 将纯色矩形加入场景队列
func (p *scenePass) queueRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if p == nil || p.batch == nil {
		return errors.New("scene pass is nil")
	}
	return p.batch.queueRect(rect, color)
}

// 将纯色四边形加入场景队列
func (p *scenePass) queueQuad(points [4]mgl32.Vec2, color mgl32.Vec4) error {
	if p == nil || p.batch == nil {
		return errors.New("scene pass is nil")
	}
	return p.batch.queueQuad(points, color)
}

// 将贴图矩形加入场景队列
func (p *scenePass) queueTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	return p.queueTextureColor(texture, dstRect, uvRect, mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
}

// 将单色调制的贴图矩形加入场景队列
func (p *scenePass) queueTextureColor(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if p == nil || p.batch == nil {
		return errors.New("scene pass is nil")
	}
	return p.batch.queueTexture(texture, dstRect, uvRect, color)
}

// 提交带颜色参数的贴图绘制
func (p *scenePass) queueTextureColorOptions(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color ColorOptions) error {
	if p == nil || p.batch == nil {
		return errors.New("scene pass is nil")
	}
	return p.batch.queueTextureColorOptions(texture, dstRect, uvRect, color)
}

// 清空场景离屏缓冲
func (p *scenePass) clear(color mgl32.Vec4) {
	if p == nil || p.glCtx == nil || p.framebuffer == 0 {
		return
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, p.framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	p.glCtx.ClearColor(color.X(), color.Y(), color.Z(), color.W())
	p.glCtx.Clear(gl.COLOR_BUFFER_BIT)
}

// 将场景队列绘制到 logical size FBO
func (p *scenePass) render() error {
	if p == nil || p.glCtx == nil || p.framebuffer == 0 || p.shader == nil || p.batch == nil {
		return nil
	}

	p.stats = passStatsFromBatch(true, p.batch.stats())
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, p.framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))

	p.shader.use()
	// 设置视图投影矩阵
	/*
	* |  2/w    0      0     -1 |
	* |   0   -2/h     0      1 |
	* |   0     0     -1      0 |
	* |   0     0      0      1 |
	*
	* 对一个点，p = (x, y, z, 1)，乘完以后，
	* clipX =  2*x/w - 1
	* clipY = -2*y/h + 1， y越大，clipY越小，从而实现了Y轴向下增长的坐标系
	* clipZ = -z
	* clipW = 1
	 */
	viewProj := mgl32.Ortho(0, p.size.X(), p.size.Y(), 0, -1, 1)
	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	if err := p.batch.flush(p.textureLocation, p.useTextureLocation); err != nil {
		p.glCtx.UseProgram(0)
		return err
	}
	p.glCtx.UseProgram(0)
	return nil
}

// 返回上一帧场景 pass 统计
func (p *scenePass) renderStats() PassStats {
	if p == nil {
		return PassStats{}
	}
	return p.stats
}

// 返回场景输出纹理
func (p *scenePass) texture() *Texture {
	if p == nil {
		return nil
	}
	return p.colorTexture
}

// 释放场景离屏缓冲资源
func (p *scenePass) clean() {
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

// 初始化 FBO、颜色纹理、uniform 和批处理资源
func (p *scenePass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.textureLocation = p.shader.uniformLocation("uTexture")
	p.useTextureLocation = p.shader.uniformLocation("uUseTexture")

	if p.viewProjLocation < 0 || p.textureLocation < 0 || p.useTextureLocation < 0 {
		return errors.New("scene pass uniform location is invalid")
	}

	// 创建FBO
	framebuffer := p.glCtx.CreateFramebuffer()
	if framebuffer == 0 {
		return fmt.Errorf("create scene framebuffer failed,%v", p.glCtx.GetError())
	}

	// 创建颜色纹理
	textureID := p.glCtx.CreateTexture()
	if textureID == 0 {
		p.glCtx.DeleteFramebuffer(framebuffer)
		return fmt.Errorf("create scene color texture failed,%v", p.glCtx.GetError())
	}

	width := int32(p.size.X())
	height := int32(p.size.Y())
	// 绑定纹理对象，后续参数设置和像素上传都落到这个对象上
	p.glCtx.BindTexture(gl.TEXTURE_2D, textureID)
	// GL_NEAREST 直接取最近纹素，避免像素风资源出现模糊边缘
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	// GL_CLAMP_TO_EDGE 避免 UV 越界时采到相邻图块颜色
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	p.glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	// 只分配纹理内存，不上传初始像素内容，因为这张纹理后面会被 framebuffer 写入
	p.glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(gl.RGBA), width, height, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	p.glCtx.BindTexture(gl.TEXTURE_2D, 0)

	// 绑定FBO
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, framebuffer)
	// 将texture设置为FBO的颜色附件
	p.glCtx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, textureID, 0)
	status := p.glCtx.CheckFramebufferStatus(gl.FRAMEBUFFER)
	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	if status != gl.FRAMEBUFFER_COMPLETE {
		p.glCtx.DeleteTexture(textureID)
		p.glCtx.DeleteFramebuffer(framebuffer)
		return fmt.Errorf("scene framebuffer is incomplete,%d", status)
	}

	p.framebuffer = framebuffer
	p.colorTexture = &Texture{
		glCtx:  p.glCtx,
		id:     textureID,
		width:  width,
		height: height,
		path:   "scene-pass-color",
	}

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	return nil
}
