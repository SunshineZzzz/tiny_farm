package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理世界层光照缓冲
//
// 当前阶段只生成 logical size 的 light color，并用环境光颜色清空整张光照纹理
type lightingPass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 光照 framebuffer
	framebuffer uint32
	// 光照颜色纹理
	colorTexture *Texture
	// 光照缓冲尺寸，固定等于逻辑分辨率
	size mgl32.Vec2
	// 环境光颜色，RGB 表示亮度，Alpha 固定按 1 使用
	ambientColor mgl32.Vec4
	// 是否启用光照合成
	enabled bool
}

// 创建光照 pass
func newLightingPass(glCtx gl.Context, logicalSize mgl32.Vec2) (*lightingPass, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	if logicalSize.X() <= 0.0 || logicalSize.Y() <= 0.0 {
		return nil, errors.New("logical size is invalid")
	}

	pass := &lightingPass{
		glCtx:        glCtx,
		size:         logicalSize,
		ambientColor: mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
		enabled:      true,
	}

	if err := pass.init(); err != nil {
		pass.clean()
		return nil, err
	}

	return pass, nil
}

// 初始化 FBO 和颜色纹理
func (p *lightingPass) init() error {
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

// 设置环境光颜色
func (p *lightingPass) setAmbientColor(color mgl32.Vec4) {
	if p == nil {
		return
	}
	p.ambientColor = mgl32.Vec4{color.X(), color.Y(), color.Z(), 1.0}
}

// 设置是否启用光照合成
func (p *lightingPass) setEnabled(enabled bool) {
	if p == nil {
		return
	}
	p.enabled = enabled
}

// 将环境光写入 light color
func (p *lightingPass) render() error {
	if p == nil || p.glCtx == nil || p.framebuffer == 0 {
		return nil
	}

	if !p.enabled {
		return nil
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, p.framebuffer)
	p.glCtx.Viewport(0, 0, int32(p.size.X()), int32(p.size.Y()))
	// 把整张光照纹理清成环境光颜色
	p.glCtx.ClearColor(p.ambientColor.X(), p.ambientColor.Y(), p.ambientColor.Z(), 1.0)
	p.glCtx.Clear(gl.COLOR_BUFFER_BIT)
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
	if p.glCtx != nil && p.framebuffer != 0 {
		p.glCtx.DeleteFramebuffer(p.framebuffer)
		p.framebuffer = 0
	}
}
