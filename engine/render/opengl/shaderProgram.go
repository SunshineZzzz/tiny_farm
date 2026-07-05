package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"
)

// 封装 OpenGL 着色器程序的编译、链接和释放
//
// 从内存源码创建程序，不负责文件加载或热重载
type shaderProgram struct {
	// OpenGL program 对象句柄
	id uint32
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
}

// 从顶点和片段着色器源码创建着色器程序
func newShaderProgram(glCtx gl.Context, vertexSource, fragmentSource string) (*shaderProgram, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	sp := &shaderProgram{glCtx: glCtx}
	if err := sp.loadFromSources(vertexSource, fragmentSource); err != nil {
		sp.clean()
		return nil, err
	}

	return sp, nil
}

// 释放 OpenGL program 对象
func (sp *shaderProgram) clean() {
	if sp == nil || sp.glCtx == nil || sp.id == 0 {
		return
	}

	sp.glCtx.DeleteProgram(sp.id)
	sp.id = gl.ZERO
}

// 绑定当前 program 作为后续绘制使用的着色器
func (sp *shaderProgram) use() {
	if sp == nil || sp.glCtx == nil {
		return
	}

	sp.glCtx.UseProgram(sp.id)
}

// 查询 uniform 位置
func (sp *shaderProgram) uniformLocation(name string) int32 {
	if sp == nil || sp.glCtx == nil || sp.id == 0 {
		return -1
	}

	return sp.glCtx.GetUniformLocation(sp.id, name)
}

// 编译并链接顶点、片段着色器源码
func (sp *shaderProgram) loadFromSources(vertexSource, fragmentSource string) error {
	vertexShader, err := sp.compileShader(gl.VERTEX_SHADER, vertexSource)
	if err != nil {
		return err
	}
	defer sp.glCtx.DeleteShader(vertexShader)

	fragmentShader, err := sp.compileShader(gl.FRAGMENT_SHADER, fragmentSource)
	if err != nil {
		return err
	}
	defer sp.glCtx.DeleteShader(fragmentShader)

	program := sp.glCtx.CreateProgram()
	if program == 0 {
		return fmt.Errorf("create shader program failed, error=%v", sp.glCtx.GetError())
	}

	sp.glCtx.AttachShader(program, vertexShader)
	sp.glCtx.AttachShader(program, fragmentShader)
	sp.glCtx.LinkProgram(program)

	if sp.glCtx.GetProgrami(program, gl.LINK_STATUS) == gl.FALSE {
		log := sp.glCtx.GetProgramInfoLog(program)
		sp.glCtx.DeleteProgram(program)
		return fmt.Errorf("link shader program failed: %s", log)
	}

	sp.clean()
	sp.id = program
	return nil
}

// 创建并编译单个 shader 对象
func (sp *shaderProgram) compileShader(shaderType uint32, source string) (uint32, error) {
	shader := sp.glCtx.CreateShader(shaderType)
	if shader == 0 {
		return 0, fmt.Errorf("create shader failed, type=%v, error=%v", shaderType, sp.glCtx.GetError())
	}

	sp.glCtx.ShaderSource(shader, source)
	sp.glCtx.CompileShader(shader)

	if sp.glCtx.GetShaderi(shader, gl.COMPILE_STATUS) == gl.FALSE {
		log := sp.glCtx.GetShaderInfoLog(shader)
		sp.glCtx.DeleteShader(shader)
		return 0, fmt.Errorf("compile shader failed, type=%v, log=%s", shaderType, log)
	}

	return shader, nil
}
