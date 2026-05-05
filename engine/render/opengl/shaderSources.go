package opengl

// 内置 shader 标识
//
// 即使当前源码内容相同，也按渲染职责保留独立 shader，避免后续 pass 演进互相耦合
type shaderID string

// 内置 shader 关键字
const (
	shaderSceneSprite shaderID = "scene_sprite"
	shaderLight       shaderID = "light"
	shaderEmissive    shaderID = "emissive"
	shaderBloom       shaderID = "bloom"
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
	shaderLight: {
		vertex:   lightVertexShaderSource,
		fragment: lightFragmentShaderSource,
	},
	shaderEmissive: {
		vertex:   emissiveVertexShaderSource,
		fragment: emissiveFragmentShaderSource,
	},
	shaderBloom: {
		vertex:   bloomVertexShaderSource,
		fragment: bloomFragmentShaderSource,
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

// 光照 pass 顶点着色器源码
const lightVertexShaderSource = `
#version 330 core
layout(location = 0) in vec2 aPos;
layout(location = 1) in vec2 aUV;

uniform mat4 uViewProj;

out vec2 vUV;

void main() {
	vUV = aUV;
	gl_Position = uViewProj * vec4(aPos, 0.0, 1.0);
}
`

// 光照 pass 片段着色器源码
const lightFragmentShaderSource = `
#version 330 core
in vec2 vUV;

// 灯光的颜色(RGB)
uniform vec3 uLightColor;
// 灯光的强度(亮度系数）
uniform float uLightIntensity;
// 灯光类型: 0点光源, 1聚光灯, 2平行光
uniform int uLightType;
// 聚光灯的朝向向量
uniform vec2 uSpotDir;
// 聚光灯内角余弦值(全亮区域边界)
uniform float uSpotInnerCos;
// 聚光灯外角余弦值(完全黑暗边界)
uniform float uSpotOuterCos;
// 正午混合因子, 0.0黄昏/黎明, 1.0正午
uniform float uMiddayBlend;
// 投影轴方向, 沿该方向的投影值越大, 遮罩越接近1, 越亮
uniform vec2 uDir2D;
// 方向光明暗分界线的位置
uniform float uDirOffset;
// 方向光明暗过渡宽度。越大，明暗变化越柔
uniform float uDirSoftness;

out vec4 FragColor;

// 点光/聚光的径向衰减遮罩
// 潜在约定, 每个光源在自己的局部quad中心
// 返回值 - 是一个[0, 1]的亮度衰减, 1表示最亮(无衰减, 中心), 0表示完全衰减(边缘/外部)
// uv - 当前像素的 UV 坐标
float computeAttenuation(vec2 uv) {
	// 把UV坐标改成以中心为原点
	vec2 d = uv - vec2(0.5);
	// 向量长度*2.0
	// 在[0, 1]的UV方块里, 从中心到上下左右边缘的距离是0.5, 不是1.0
	// *2的目的是, 把UV中心到边缘的最大轴向距离0.5, 放大成半径距离1.0
	float dist = length(d) * 2.0;
	// a是一个线性衰减, 越靠中心越亮, 越靠边缘越暗 
	float a = clamp(1.0 - dist, 0.0, 1.0);
	// 把线性衰减平方一下。这样会让边缘衰减更快，光斑更集中
	return a * a;
}

// 聚光灯的角度遮罩
// 返回值 - 是一个[0, 1]的亮度遮罩, 1.0在聚光灯中心方向内, 完全照亮, 0.0在聚光灯外，完全不照, (0, 1)在边缘过渡区，柔和衰减
// uv - 当前像素的 UV 坐标
// spotDir - 聚光灯朝向
// innerCos - 内角余弦，完全亮区域边界
// outerCos - 外角余弦，完全暗区域边界
float computeSpotMask(vec2 uv, vec2 spotDir, float innerCos, float outerCos) {
	// 把UV坐标改成以中心为原点
	vec2 d = uv - vec2(0.5);
	vec2 dir = normalize(spotDir);
	// 如果距离太近, 就不计算了, 直接假定这个点的方向和聚光灯朝向一致, 避免除0错误
	vec2 l = (length(d) > 1e-5) ? normalize(d) : dir;
	// 计算当前像素方向l和聚光灯朝向dir的夹角余弦值
	float cd = dot(l, dir);
	// 如果两者很接近, 下面的smoothstep会出现除0错误
	if (abs(outerCos - innerCos) < 1e-5) {
		return 1.0;
	}
	// 把cd映射成渐变遮罩, 聚光灯外0, 聚光灯内1, 中间边缘柔和过渡
	// cd <= outerCos  -> 0.0, 聚光灯外
  	// cd >= innerCos  -> 1.0, 聚光灯内圈
	// 中间             -> (0, 1), 边缘柔和过渡
	return smoothstep(outerCos, innerCos, cd);
}

// 方向光的渐变遮罩，比如太阳光从左上照来，屏幕某一侧亮，另一侧暗，中间有一段柔和过渡
// 返回值 - 是一个[0, 1]的亮度遮罩, 0这块没有方向光, 1这块有方向光, 中间值过渡区域
// uv - 当前像素的 UV 坐标
// dir2d - 投影轴方向, 沿该方向的投影值越大, 遮罩越接近1, 越亮
//         注意：它不是物理平行光的“照射方向”，而是屏幕空间明暗渐变的方向。
// offset - 明暗分界线的位置
// softness - 明暗过渡宽度。越大，明暗变化越柔
// middayBlend - 中午混合值。越接近1, 越接近全屏均匀亮, 不再有明显方向渐变。
float computeDirectionalMask(vec2 uv, vec2 dir2d, float offset, float softness, float middayBlend) {
	vec2 nd = normalize(dir2d);
	// uv-vec2(0.5) - UV 坐标系的原点从左下角(0,0)移动到中心(0.5, 0.5)​，就可以区分出光线同侧/背侧(正/负)
	// t - 当前像素在方向光轴线上的位置, 尽量映射到[0, 1]附近, 左侧接近0, 中心接近0.5, 右侧接近1
	float t = dot(nd, uv - vec2(0.5)) + 0.5;
	// 比如, offset=0.5, softness=0.1, edge0=0.4, edge1=0.6
	// 从t=0.4开始进入过渡, 到t=0.6结束过渡
	// t<0.4, 基本是暗
	// t=[0.4, 0.6], 平滑变亮
	// t>0.6, 基本是亮
	float edge0 = clamp(offset - softness, 0.0, 1.0);
	float edge1 = clamp(offset + softness, 0.0, 1.0);
	// 把方向轴上的位置t映射成亮度遮罩, 暗侧为0, 亮侧为1, 中间平滑过渡
	// smoothstep(a, b, x), x<=a返回0, x>=b返回1, 中间返回(0, 1)之间的平滑曲线
	float ramp = smoothstep(edge0, edge1, t);
	// 根据“正午混合值”来决定是否要“消除”掉之前计算出的方向性阴影
	// mix(a, b, x), 线性插值, 在a和b之间, 按比例x取一个值
	return mix(ramp, 1.0, clamp(middayBlend, 0.0, 1.0));
}

void main() {
	vec3 rgb;
	if (uLightType == 2) {
		// 方向光(平行光)
		// 计算方向光的渐变遮罩
		float dirMask = computeDirectionalMask(vUV, uDir2D, uDirOffset, uDirSoftness, uMiddayBlend);
		// 最终光照颜色 = 光的颜色 * 光强 * 当前像素的方向遮罩
		rgb = uLightColor * (uLightIntensity * dirMask);
	} else if (uLightType == 1) {
		// 聚光灯
		// 计算聚光灯的径向衰减遮罩
		float attenuation = computeAttenuation(vUV);
		// 计算聚光灯的角度遮罩
		//         L
        //        /|\
        //       / | \
        //      /  |  \
        //     /   A   \
        //    /    |    \
        //   /     |     \
        //  /      |      \
        // /       B       \
		// 如果是上方情况, 则spotMask都是1.0, 需要径向衰减遮罩来区分不同距离的像素
		float spotMask = computeSpotMask(vUV, uSpotDir, uSpotInnerCos, uSpotOuterCos);
		// 加上 computeAttenuation 后才会变成：
		// L
		// |
		// A   很亮
		// |
		// |
		// B   很暗
		rgb = uLightColor * (uLightIntensity * attenuation * spotMask);
	} else {
	 	// 点光源
		// 计算点光源的径向衰减遮罩
		float attenuation = computeAttenuation(vUV);
		// 当前像素的点光颜色 = 光源颜色 * 光源强度 * 距离衰减
		rgb = uLightColor * (uLightIntensity * attenuation);
	}
	FragColor = vec4(rgb, 1.0);
}
`

// 自发光 pass 顶点着色器源码
const emissiveVertexShaderSource = `
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

// 自发光 pass 片段着色器源码
const emissiveFragmentShaderSource = `
#version 330 core
in vec2 vUV;
in vec4 vColor;

uniform sampler2D uTexture;
uniform bool uUseTexture;

out vec4 FragColor;

void main() {
	vec4 color = vColor;
	if (uUseTexture) {
		color *= texture(uTexture, vUV);
	}
	FragColor = color;
}
`

// Bloom pass 顶点着色器源码
const bloomVertexShaderSource = `
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

// Bloom pass 片段着色器源码
const bloomFragmentShaderSource = `
#version 330 core
in vec2 vUV;
in vec4 vColor;

// 要模糊的纹理
uniform sampler2D uTexture;
// 单个纹理像素在UV空间的大小,
// 假设纹理宽度=800px, 高度=600px
// 则uTexelSize = vec2(1.0/800.0, 1.0/600.0)
// 表示在UV空间移动一个像素所需的步长
uniform vec2 uTexelSize;
// 模糊的方向向量，决定模糊是水平、垂直还是斜向
uniform vec2 uDirection;

out vec4 FragColor;

void main() {
	// 在模糊方向上一个像素的偏移量
	vec2 stepUV = uTexelSize * uDirection;
	// [0.070270, 0.316216, 0.227027, 0.316216, 0.070270], 高斯模糊权重, 采样点数量5TAP
	// 中心点, 权重22.7%, 当前像素距离0.0
	vec3 color = texture(uTexture, vUV).rgb * 0.227027;
	// 高斯模糊本来需要9TAP, 用更少的采样5TAP, 模拟9TAP的效果
	// 因为高斯模糊要求第2个格子比第3个格子更重要(权重更高)。如果你看正中间(1.5), 
	// 两个格子就一样清楚了。所以要稍微往第2个格子挪一点, 算法计算出1.384615
	// texture(uTexture, uv), 代码层需要设置成线性采样, color ≈ (1 - t) * texel(1) + t * texel(2)
	// 
	// 模糊正方向第1、2格的等效采样点, 权重31.6216%
	color += texture(uTexture, vUV + stepUV * 1.384615).rgb * 0.316216;
	// 模糊负方向第1、2格的等效采样点, 权重31.6216%
	color += texture(uTexture, vUV - stepUV * 1.384615).rgb * 0.316216;
	// 模糊正方向第3、4格的等效采样点, 权重7.0270%
	color += texture(uTexture, vUV + stepUV * 3.230769).rgb * 0.070270;
	// 模糊负方向第3、4格的等效采样点, 权重7.0270%
	color += texture(uTexture, vUV - stepUV * 3.230769).rgb * 0.070270;
	// vColor没实际意义, 因为BloomPass复用了SpriteBatch, 所以shader还带着vColor。但实际传入永远是白色
	FragColor = vec4(color * vColor.rgb, 1.0);
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
uniform sampler2D uLightColor;
uniform vec3 uAmbient;
uniform sampler2D uEmissiveColor;
uniform sampler2D uBloomColor;
// 光晕强度
uniform float uBloomStrength;

out vec4 FragColor;

void main() {
	// vColor没实际意义, 因为CompositePass复用了SpriteBatch, 所以shader还带着vColor。但实际传入永远是白色
	vec4 sceneColor = texture(uSceneColor, vUV) * vColor;
	vec3 lightColor = texture(uLightColor, vUV).rgb + uAmbient;
	vec3 emissiveColor = texture(uEmissiveColor, vUV).rgb;
	vec3 bloomColor = texture(uBloomColor, vUV).rgb * uBloomStrength;
	// 最终颜色 = 物体颜色 * 光照强度 + 自发光颜色 + Bloom 颜色
	// * 是调制/缩放，+ 是叠加/增加。
	// sceneColor.rgb * lightColor, 物体本身颜色受光照影响后的结果
	// + emissiveColor, 自发光颜色额外加到最终颜色上
	// + bloomColor, 自发光模糊后的辉光额外加到最终颜色上
	vec3 rgb = sceneColor.rgb * clamp(lightColor, 0.0, 1.0) + emissiveColor + bloomColor;
	FragColor = vec4(clamp(rgb, 0.0, 1.0), sceneColor.a);
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
