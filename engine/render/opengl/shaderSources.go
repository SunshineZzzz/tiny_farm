package opengl

// 内置 shader 标识
//
// 即使当前源码内容相同，也按渲染职责保留独立 shader，避免后续 pass 演进互相耦合
type shaderID string

// 内置 shader 关键字
const (
	shaderSceneSprite shaderID = "scene_sprite"
	shaderComposite   shaderID = "composite"
	shaderUI          shaderID = "ui"
)

// 内置 shader 源码集合
type shaderSource struct {
	// 顶点着色器源码
	vertex string
	// 片段着色器源码
	fragment string
}

// 内置 shader 映射
var builtinShaderSources = map[shaderID]shaderSource{
	shaderSceneSprite: {
		vertex:   sceneSpriteVertexShaderSource,
		fragment: sceneSpriteFragmentShaderSource,
	},
	shaderComposite: {
		vertex:   compositeVertexShaderSource,
		fragment: compositeFragmentShaderSource,
	},
	shaderUI: {
		vertex:   uiVertexShaderSource,
		fragment: uiFragmentShaderSource,
	},
}

// 场景精灵顶点着色器源码
const sceneSpriteVertexShaderSource = `
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

// 场景精灵片段着色器源码
const sceneSpriteFragmentShaderSource = `
#version 330 core
in vec2 vUV;
in vec4 vColor;

uniform sampler2D uTexture;
uniform bool uUseTexture;

out vec4 FragColor;

void main() {
	if (uUseTexture) {
		FragColor = texture(uTexture, vUV) * vColor;
	} else {
		FragColor = vColor;
	}
}
`

// 合成 pass 顶点着色器源码
const compositeVertexShaderSource = `
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

// 合成 pass 片段着色器源码
const compositeFragmentShaderSource = `
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

// UI 顶点着色器源码
const uiVertexShaderSource = `
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

// UI 片段着色器源码
const uiFragmentShaderSource = `
#version 330 core
in vec2 vUV;
in vec4 vColor;

uniform sampler2D uTexture;
uniform bool uUseTexture;

out vec4 FragColor;

void main() {
	if (uUseTexture) {
		FragColor = texture(uTexture, vUV) * vColor;
	} else {
		FragColor = vColor;
	}
}
`
