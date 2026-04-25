package opengl

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"tiny_farm/engine/utils"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// OpenGL 纹理资源
//
// 当前阶段只保存最小绘制所需的句柄和像素尺寸，资源生命周期由创建它的 GLRenderer 管理
type Texture struct {
	// OpenGL 上下文
	glCtx gl.Context
	// 底层 OpenGL texture 句柄
	id uint32
	// 纹理像素尺寸
	width  int32
	height int32
	// 纹理文件路径
	path string
}

// 返回底层 OpenGL texture 句柄
func (t *Texture) ID() uint32 {
	if t == nil {
		return 0
	}
	return t.id
}

// 返回纹理像素尺寸
func (t *Texture) Size() mgl32.Vec2 {
	if t == nil {
		return mgl32.Vec2{}
	}
	return mgl32.Vec2{float32(t.width), float32(t.height)}
}

// 释放纹理资源
func (t *Texture) Close() {
	if t == nil || t.glCtx == nil || t.id == 0 {
		return
	}

	t.glCtx.DeleteTexture(t.id)
	t.id = 0
	t.width = 0
	t.height = 0
}

// 从图像文件创建 OpenGL texture
//
// 当前沿用参考实现的 NEAREST 和 CLAMP_TO_EDGE，优先保证像素风资源不糊边
func newTexture(glCtx gl.Context, path string) (*Texture, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	rgba := utils.ImageToNRGBA(img, true)
	bounds := rgba.Bounds()
	width := int32(bounds.Dx())
	height := int32(bounds.Dy())
	if width <= 0 || height <= 0 {
		return nil, errors.New("texture size is invalid")
	}

	// 创建纹理对象
	textureID := glCtx.CreateTexture()
	if textureID == 0 {
		return nil, fmt.Errorf("create texture failed,%v", glCtx.GetError())
	}

	// 绑定纹理对象，后续参数设置和像素上传都落到这个对象上
	glCtx.BindTexture(gl.TEXTURE_2D, textureID)

	// GL_NEAREST 直接取最近纹素，避免像素风资源出现模糊边缘
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	// GL_CLAMP_TO_EDGE 避免 UV 越界时采到相邻图块颜色
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// 纹理数据按 1 字节对齐上传，避免非 4 字节宽度图片读错行
	// 设置 1 是为了让这个函数“通用”。无论你传进来的是什么样奇葩尺寸、什么样通道数的图片，它都能保证绝对不花屏
	glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	glCtx.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, gl.RGBA, gl.UNSIGNED_BYTE, rgba.Pix)

	// 初始化完成后解绑，避免污染后续状态
	glCtx.BindTexture(gl.TEXTURE_2D, 0)

	return &Texture{
		glCtx:  glCtx,
		id:     textureID,
		width:  width,
		height: height,
		path:   path,
	}, nil
}

// 将像素源矩形转换成 UV 矩形
func textureSourceRectUV(texture *Texture, srcRect mgl32.Vec4) (mgl32.Vec4, error) {
	if texture == nil || texture.width <= 0 || texture.height <= 0 {
		return mgl32.Vec4{}, errors.New("texture is nil")
	}
	if srcRect.Z() <= 0 || srcRect.W() <= 0 {
		return mgl32.Vec4{}, errors.New("source rect width or height is invalid")
	}

	texW := float32(texture.width)
	texH := float32(texture.height)
	return mgl32.Vec4{
		srcRect.X() / texW,
		srcRect.Y() / texH,
		(srcRect.X() + srcRect.Z()) / texW,
		(srcRect.Y() + srcRect.W()) / texH,
	}, nil
}
