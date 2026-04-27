package opengl

import (
	"errors"
	"fmt"

	gl "tiny_farm/engine/utils/opengl"
)

// 统一管理内置 shader program 生命周期
//
// 只负责创建、缓存和释放，不合并不同 pass 的 shader 语义
type shaderLibrary struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 已创建的内置 shader program
	programs map[shaderID]*shaderProgram
}

// 创建 shader 管理器
func newShaderLibrary(glCtx gl.Context) (*shaderLibrary, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}

	return &shaderLibrary{
		glCtx:    glCtx,
		programs: make(map[shaderID]*shaderProgram),
	}, nil
}

// 返回指定 shader program，首次访问时从内置源码创建
func (l *shaderLibrary) get(id shaderID) (*shaderProgram, error) {
	if l == nil || l.glCtx == nil {
		return nil, errors.New("shader library is nil")
	}

	if program := l.programs[id]; program != nil {
		return program, nil
	}

	source, ok := builtinShaderSources[id]
	if !ok {
		return nil, fmt.Errorf("shader source is not registered, id=%s", id)
	}

	program, err := newShaderProgram(l.glCtx, source.vertex, source.fragment)
	if err != nil {
		return nil, err
	}
	l.programs[id] = program

	return program, nil
}

// 释放所有已创建的 shader program
func (l *shaderLibrary) clean() {
	if l == nil {
		return
	}

	for id, program := range l.programs {
		if program != nil {
			program.clean()
		}
		delete(l.programs, id)
	}
}
