package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	// 流程大概是：
	// 原图亮部
	// ↓ 降采样
	// 1/2
	// ↓ 降采样
	// 1/4
	// ↓ 降采样
	// 1/8
	// ↓ 降采样
	// 1/16
	// 然后从最小层往回走：
	// 1/16
	// ↓ 上采样并叠加到 1/8
	// 1/8
	// ↓ 上采样并叠加到 1/4
	// 1/4
	// ↓ 上采样并叠加到 1/2
	// 1/2
	// ↓ 上采样并叠加到原图
	//
	// 模拟光散射
	// 真实光线传播是连续过程：
	// 强光产生核心光晕(Level1，锐利)
	// 光向外扩散形成中等辉光(Level2)
	// 继续扩散形成大范围环境光(Level3)
	// 逐级上采样叠加模拟了这个扩散过程：
	// 大范围光晕(Level3)会"渗透"到所有上层
	// 中等光晕(Level2)会"渗透"到上面两层
	// 小光晕(Level1)只影响最上层
	// 结果：每个像素最终的颜色包含了所有尺度的光晕贡献
	// Bloom 降采样层数，当前先固定为 4 层
	bloomLevelCount = 4
)

// Bloom 单层后处理资源
//
// 每层持有一组 ping/pong 目标，ping 用作横向模糊输出，pong 用作纵向模糊输出和向上累加目标
type bloomLevel struct {
	// 横向模糊 framebuffer
	pingFramebuffer uint32
	// 纵向模糊 framebuffer
	pongFramebuffer uint32
	// 横向模糊中间纹理
	pingTexture *Texture
	// 本层 Bloom 输出纹理
	pongTexture *Texture
	// 本层后处理缓冲尺寸
	size mgl32.Vec2
}

// 管理自发光输入的多级 Bloom 后处理
//
// 当前阶段对 emissive 输入做多级降采样模糊，再从最小层逐级向上加法累积到第 0 层
type bloomPass struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// Bloom 模糊和回贴着色器
	shader *shaderProgram
	// 全屏贴图批处理器
	batch *spriteBatch
	// 多级 Bloom ping-pong 资源
	levels []bloomLevel
	// 后处理基准尺寸，固定等于逻辑分辨率
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

	for _, level := range p.levels {
		p.clearFramebuffer(level.pingFramebuffer, level.size)
		p.clearFramebuffer(level.pongFramebuffer, level.size)
	}
}

// 对自发光纹理执行降采样, 在小纹理上做模糊, 逐级上采样并叠加
func (p *bloomPass) render(input *Texture) error {
	if p == nil || p.glCtx == nil || p.shader == nil || p.batch == nil {
		return nil
	}
	if !p.enabled || input == nil || input.id == 0 || len(p.levels) == 0 {
		return nil
	}

	p.setTextureFilter(input, gl.LINEAR)
	defer p.setTextureFilter(input, gl.NEAREST)

	source := input
	for i := range p.levels {
		// 第一级从全尺寸emissive画到1/2画布时, 目标画布宽高变成1/2, 目标像素数变成1/4
		// 画布变小后，目标小画布上的1像素偏移，对应到原始发光纹理上就是更大的距离
		// 比如原始emissive是800*600, 第一级Bloom画布是400*300, 在第一级画布里移动1个像素，对应原图大约移动
		// 800/400=2, 600/300=2, 也就是原图里的2*2像素范围, 因为这里把输入纹理设成了线性过滤p.setTextureFilter(input, gl.LINEAR)
		// 所以采样落在原图某个UV坐标时，硬件会对附近的2*2的texel做双线性插值。效果近似是
		// 小画布上的1个像素≈原图上附近几个像素的混合结果, 这一步会丢掉高频细节，所以看起来变软、变糊
		level := &p.levels[i]
		if err := p.renderBlurPass(source, level.pingFramebuffer, level.size, mgl32.Vec2{1.0, 0.0}); err != nil {
			return err
		}
		if err := p.renderBlurPass(level.pingTexture, level.pongFramebuffer, level.size, mgl32.Vec2{0.0, 1.0}); err != nil {
			return err
		}
		source = level.pongTexture
	}

	for i := len(p.levels) - 1; i > 0; i-- {
		// 大画布采样小画布，目标画布更大、源纹理更小时，多个目标fragment会采到源纹理上同一个texel附近
		// LINEAR过滤，每个目标像素的UV位置略有不同，会从源纹理相邻的2*2的texel插值，于是光晕就被“铺开”了
		src := p.levels[i].pongTexture
		dst := &p.levels[i-1]
		if err := p.additiveBlit(src, dst.pongFramebuffer, dst.size); err != nil {
			return err
		}
	}

	return nil
}

// 返回 Bloom 输出纹理
func (p *bloomPass) texture() *Texture {
	if p == nil || !p.enabled || len(p.levels) == 0 {
		return nil
	}
	return p.levels[0].pongTexture
}

// 释放 Bloom pass 的 OpenGL 资源
func (p *bloomPass) clean() {
	if p == nil {
		return
	}

	for i := range p.levels {
		level := &p.levels[i]
		if level.pingTexture != nil {
			level.pingTexture.Close()
			level.pingTexture = nil
		}
		if level.pongTexture != nil {
			level.pongTexture.Close()
			level.pongTexture = nil
		}
		if p.glCtx != nil && level.pingFramebuffer != 0 {
			p.glCtx.DeleteFramebuffer(level.pingFramebuffer)
			level.pingFramebuffer = 0
		}
		if p.glCtx != nil && level.pongFramebuffer != 0 {
			p.glCtx.DeleteFramebuffer(level.pongFramebuffer)
			level.pongFramebuffer = 0
		}
	}
	p.levels = nil

	if p.batch != nil {
		p.batch.clean()
		p.batch = nil
	}
	p.shader = nil
	p.viewProjLocation = -1
	p.textureLocation = -1
	p.texelSizeLocation = -1
	p.directionLocation = -1
}

// 初始化多级 FBO、纹理、uniform 和全屏批处理器
func (p *bloomPass) init() error {
	p.viewProjLocation = p.shader.uniformLocation("uViewProj")
	p.textureLocation = p.shader.uniformLocation("uTexture")
	p.texelSizeLocation = p.shader.uniformLocation("uTexelSize")
	p.directionLocation = p.shader.uniformLocation("uDirection")

	if p.viewProjLocation < 0 || p.textureLocation < 0 ||
		p.texelSizeLocation < 0 || p.directionLocation < 0 {
		return errors.New("bloom pass uniform location is invalid")
	}

	if err := p.createLevels(); err != nil {
		return err
	}

	batch, err := newSpriteBatch(p.glCtx, minSpriteBatchCapacity)
	if err != nil {
		return err
	}
	p.batch = batch

	return nil
}

// 创建固定层数的 Bloom 降采样目标
//
// 当前每层宽高按上一层一半计算，最小限制为 1 像素，避免极小逻辑分辨率下生成非法纹理
func (p *bloomPass) createLevels() error {
	p.levels = make([]bloomLevel, 0, bloomLevelCount)
	divisor := float32(2.0)
	for i := range bloomLevelCount {
		size := mgl32.Vec2{
			max(1.0, p.size.X()/divisor),
			max(1.0, p.size.Y()/divisor),
		}

		pingFramebuffer, pingTexture, err := p.createRenderTarget(size, fmt.Sprintf("bloom-level-%d-ping", i))
		if err != nil {
			return err
		}
		pongFramebuffer, pongTexture, err := p.createRenderTarget(size, fmt.Sprintf("bloom-level-%d-pong", i))
		if err != nil {
			pingTexture.Close()
			p.glCtx.DeleteFramebuffer(pingFramebuffer)
			return err
		}

		p.levels = append(p.levels, bloomLevel{
			pingFramebuffer: pingFramebuffer,
			pongFramebuffer: pongFramebuffer,
			pingTexture:     pingTexture,
			pongTexture:     pongTexture,
			size:            size,
		})
		divisor *= 2.0
	}
	return nil
}

// 创建 Bloom 单层使用的 framebuffer 和颜色纹理
//
// 当前只需要颜色附件，不创建深度或模板附件，因为 Bloom pass 只做全屏纹理后处理
func (p *bloomPass) createRenderTarget(size mgl32.Vec2, path string) (uint32, *Texture, error) {
	framebuffer := p.glCtx.CreateFramebuffer()
	if framebuffer == 0 {
		return 0, nil, fmt.Errorf("create bloom framebuffer failed,%v", p.glCtx.GetError())
	}

	textureID := p.glCtx.CreateTexture()
	if textureID == 0 {
		p.glCtx.DeleteFramebuffer(framebuffer)
		return 0, nil, fmt.Errorf("create bloom texture failed,%v", p.glCtx.GetError())
	}

	width := int32(size.X())
	height := int32(size.Y())
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

// 将输入纹理模糊写入指定 framebuffer
//
// direction 控制横向或纵向模糊，当前覆盖写入目标纹理，避免旧内容污染本轮 Bloom 结果
func (p *bloomPass) renderBlurPass(input *Texture, framebuffer uint32, size mgl32.Vec2, direction mgl32.Vec2) error {
	if input == nil || input.id == 0 || framebuffer == 0 {
		return nil
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, framebuffer)
	p.glCtx.Viewport(0, 0, int32(size.X()), int32(size.Y()))
	// 覆盖写入输出纹理，而不是和输出纹理里原来的内容混合
	p.glCtx.Disable(gl.BLEND)

	return p.drawTexture(input, size, direction)
}

// 将低分辨率 Bloom 结果加法叠加到目标层
//
// 当前用于从最小层逐级向上累积，让大范围光晕贡献写入更高分辨率层
func (p *bloomPass) additiveBlit(input *Texture, framebuffer uint32, size mgl32.Vec2) error {
	if input == nil || input.id == 0 || framebuffer == 0 {
		return nil
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, framebuffer)
	p.glCtx.Viewport(0, 0, int32(size.X()), int32(size.Y()))
	// RGB 和 Alpha 都用加法公式
	// 最终颜色 = 源颜色 * 源系数 + 目标颜色 * 目标系数
	p.glCtx.Enable(gl.BLEND)
	p.glCtx.BlendEquationSeparate(gl.FUNC_ADD, gl.FUNC_ADD)
	p.glCtx.BlendFuncSeparate(gl.ONE, gl.ONE, gl.ONE, gl.ONE)

	return p.drawTexture(input, size, mgl32.Vec2{0.0, 0.0})
}

// 把输入纹理按目标尺寸画满当前 framebuffer
//
// 当前复用 spriteBatch 做全屏四边形绘制，direction 为零时 shader 只做普通采样回贴
func (p *bloomPass) drawTexture(input *Texture, size mgl32.Vec2, direction mgl32.Vec2) error {
	if err := p.batch.queueTexture(
		input,
		mgl32.Vec4{0.0, 0.0, size.X(), size.Y()},
		mgl32.Vec4{0.0, 0.0, 1.0, 1.0},
		mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
	); err != nil {
		p.restoreState()
		return err
	}

	p.shader.use()
	viewProj := mgl32.Ortho(0, size.X(), size.Y(), 0, -1, 1)
	p.glCtx.UniformMatrix4fv(p.viewProjLocation, viewProj[:])
	p.glCtx.Uniform2fv(p.texelSizeLocation, []float32{1.0 / size.X(), 1.0 / size.Y()})
	p.glCtx.Uniform2fv(p.directionLocation, []float32{direction.X(), direction.Y()})
	if err := p.batch.flush(p.textureLocation, -1); err != nil {
		p.restoreState()
		return err
	}
	p.restoreState()
	return nil
}

// 临时切换纹理过滤方式
//
// 当前 Bloom 降采样和上采样依赖线性过滤，处理结束后调用方会恢复原有最近邻采样
func (p *bloomPass) setTextureFilter(texture *Texture, filter uint32) {
	if texture == nil || texture.id == 0 {
		return
	}

	p.glCtx.BindTexture(gl.TEXTURE_2D, texture.id)
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, int32(filter))
	p.glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, int32(filter))
	p.glCtx.BindTexture(gl.TEXTURE_2D, 0)
}

// 清空指定 Bloom framebuffer
//
// 当前固定清成透明黑色，表示该层没有任何辉光贡献
func (p *bloomPass) clearFramebuffer(framebuffer uint32, size mgl32.Vec2) {
	if framebuffer == 0 {
		return
	}

	p.glCtx.BindFramebuffer(gl.FRAMEBUFFER, framebuffer)
	p.glCtx.Viewport(0, 0, int32(size.X()), int32(size.Y()))
	p.glCtx.ClearColor(0.0, 0.0, 0.0, 1.0)
	p.glCtx.Clear(gl.COLOR_BUFFER_BIT)
}

// 恢复后续 pass 使用的默认 OpenGL 状态
//
// 当前 Bloom 会切换 framebuffer、shader 和混合模式，绘制结束后需要回到普通 alpha 混合
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
