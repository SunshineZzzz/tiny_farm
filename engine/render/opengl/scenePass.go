package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理场景离屏渲染目标
//
// 当前阶段只负责固定 logical size 的颜色缓冲，用于把场景先画到 FBO，
// 再在 Present 阶段输出到默认 framebuffer
type scenePass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 离屏 framebuffer
	framebuffer uint32
	// 离屏颜色纹理
	colorTexture *Texture
	// 离屏缓冲尺寸，固定等于逻辑分辨率
	size mgl32.Vec2
}

// 创建场景离屏渲染目标
func newScenePass(glCtx gl.Context, logicalSize mgl32.Vec2) (*scenePass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}
	if logicalSize.X() <= 0 || logicalSize.Y() <= 0 {
		return nil, errors.New("logical size is invalid")
	}

	pass := &scenePass{
		glCtx: glCtx,
		size:  logicalSize,
	}
	if err := pass.init(); err != nil {
		pass.clean()
		return nil, err
	}
	return pass, nil
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
	if p.glCtx != nil && p.framebuffer != 0 {
		p.glCtx.DeleteFramebuffer(p.framebuffer)
		p.framebuffer = 0
	}
}

// 初始化 FBO 和颜色纹理
func (p *scenePass) init() error {
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
	// 上传颜色纹理像素数据
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
	return nil
}
