# 字体与文本渲染实施路线

## 目标

当前 Go 版本需要在已有 2D OpenGL 渲染管线之上补齐字体与文本渲染能力。目标不是一次性完整搬运 `copy_source/TinyFarm` 的 C++ 实现，而是先建立一个可验证的文本显示闭环，再逐步扩展到字体缓存、glyph atlas、文本样式、布局缓存和复杂文本整形。

最终数据流：

```text
UTF-8 文本
  -> FontManager 加载字体和 glyph
  -> glyph atlas 纹理
  -> TextRenderer 生成布局
  -> Renderer.DrawUIText / DrawWorldText
  -> 复用当前 UI pass / world pass 绘制
```

核心边界：

- `FontManager` 负责字体文件、字号、glyph 缓存和 glyph atlas 生命周期
- `TextRenderer` 负责文本测量、布局缓存、样式解析和 UI/world 文本绘制
- `ResourceManager` 是字体资源的统一入口，上层不直接管理字体对象
- `Renderer` 继续负责“怎么画”，文本系统只把 glyph 转成贴图矩形提交给它
- 第一版先支持基础 UTF-8 文本显示，复杂 shaping、双向文本和连字后续再做

## 当前状态

- `assets/fonts/VonwaonBitmap-16px.ttf` 已存在可用字体资源
- `engine/render/opengl/spriteBatch.go` 已支持贴图矩形批处理和 per-vertex color
- `engine/render/opengl/uiPass.go` 已支持独立 UI pass
- `engine/render/renderer.go` 已提供 `DrawTexture`、`DrawWorldTexture`、`DrawUITexture`
- `engine/resource/resourceManager.go` 已管理 texture、sound、music，但尚未接入 font
- `engine/render/opengl/texture.go` 已支持从图片文件创建纹理，也已支持创建空白 RGBA 纹理和更新子区域
- Go 侧尚无字体解析、glyph 位图生成、glyph atlas、文本布局和文本样式配置

## 参考范围

优先参考这些文件：

- `copy_source/TinyFarm/docs/text_rendering.md`
- `copy_source/TinyFarm/config/text_render.json`
- `copy_source/TinyFarm/src/engine/resource/font_manager.h`
- `copy_source/TinyFarm/src/engine/resource/font_manager.cpp`
- `copy_source/TinyFarm/src/engine/render/text_renderer.h`
- `copy_source/TinyFarm/src/engine/render/text_renderer.cpp`
- `copy_source/TinyFarm/src/engine/ui/ui_label.*`
- `copy_source/TinyFarm/src/engine/debug/panels/text_renderer_debug_panel.*`
- `engine/resource/resourceManager.go`
- `engine/render/renderer.go`
- `engine/render/opengl/texture.go`
- `engine/render/opengl/spriteBatch.go`

暂缓完整迁移：

- HarfBuzz shaping 的完整能力
- FreeType C 绑定
- Text debug panel
- UI label / button 的完整组件体系
- 对话系统、tooltip、HUD 等游戏 UI 文本

## copy_source 文本能力盘点

`copy_source/TinyFarm` 的文本系统可以概括为：

```text
FontManager 负责把 glyph 变成可采样 atlas
TextRenderer 负责把 UTF-8 文本变成 glyph 布局并画到屏幕
```

核心能力：

- `FontManager` 按 `(font_id, pixel_size)` 缓存 `Font`
- `Font` 封装 `FT_Face`、`hb_font_t`、字体度量、glyph cache 和 atlas pages
- `FontGlyph` 保存 glyph 所在纹理页、尺寸、bearing、advance 和 UV
- `TextRenderer` 使用 HarfBuzz 把 UTF-8 文本整形成 glyph 序列
- `TextRenderer` 按 `(font_id, font_size, text, layout_options)` 缓存布局
- UI 文本走 `drawUITexture`，世界文本走 `drawTexture`
- `config/text_render.json` 管理默认方向、语言、HarfBuzz feature、样式、阴影和布局参数
- 字体卸载或清空时，`TextRenderer` 清理布局缓存，避免引用失效

Go 版迁移时应保留这些边界，但第一版可以弱化 shaping：

- 先按 rune 顺序布局，支持中文和英文基础显示
- 后续再接 HarfBuzz 或等价 shaping 库处理复杂文本

## 阶段 1：OpenGL atlas 纹理能力

目标是让字体系统能创建空白 atlas，并把 glyph 位图写入指定区域。

当前状态：已完成底层能力，后续 `FontManager` 可以通过 `Renderer.CreateEmptyTexture` 创建 atlas，并通过 `Texture.UpdateRGBA` 写入 glyph 像素。

要做：

- 在 `engine/render/opengl/texture.go` 增加从内存创建 texture 的内部能力
- 支持创建空 RGBA texture，尺寸由字体 atlas 决定
- 支持 `TexSubImage2D` 更新 atlas 子区域
- atlas 纹理建议使用 `LINEAR` 或按像素字体需求可配置为 `NEAREST`
- 保持 `CLAMP_TO_EDGE`
- 不向 game 层暴露 OpenGL texture ID

建议接口边界：

```go
func newEmptyTexture(glCtx gl.Context, width, height int32, filter uint32) (*Texture, error)
func (t *Texture) UpdateRGBA(x, y, width, height int32, pixels []byte) error
```

验收：

- 能创建一张空 atlas 纹理
- 能向 atlas 指定区域上传 RGBA 像素
- `Texture.Close()` 能正确释放 atlas 纹理
- `go test ./...` 通过

## 阶段 2：FontManager 与字体缓存

目标是建立字体资源层，按字体 key 和字号缓存字体对象。

当前状态：已完成字体文件级缓存、`ResourceManager` facade 接入和字体调试信息。glyph 缓存、字体度量解析和 atlas 纹理生命周期仍留到后续阶段。

建议新增文件：

- `engine/resource/fontManager.go`
- `engine/resource/fontDebugInfo.go`

核心类型：

```go
type FontKey struct {
    Key  ResourceKey
    Size int
}

type FontGlyph struct {
    Texture *render.Texture
    Size    mgl32.Vec2
    Bearing mgl32.Vec2
    Advance float32
    UVRect  mgl32.Vec4
}

type Font struct {
    key        ResourceKey
    pixelSize  int
    ascender   float32
    descender  float32
    lineHeight float32
}
```

要做：

- `FontManager` 内部持有 `map[FontKey]*Font`
- `LoadFont(key, path, pixelSize)` 命中缓存时直接返回
- `GetFont(key, pixelSize)` 只查缓存，不隐式把 key 当路径
- `UnloadFont(key, pixelSize)` 释放该字号的 atlas 纹理
- `ClearFonts()` 释放全部字体缓存
- `ResourceManager.Clear()` 时一并清理字体

字体解析方案：

- 第一版优先使用 Go 库解析 TTF，避免立刻引入 FreeType/HarfBuzz C 依赖
- 可选方向是 `golang.org/x/image/font/sfnt` 或等价 Go 字体库
- 如果第一版库不能覆盖中文 glyph 渲染，再评估 FreeType 绑定

验收：

- 同一个 key 和字号重复加载不会创建第二个 `Font`
- 不同字号分别缓存
- 卸载和清空会释放 atlas 资源
- 字体加载失败返回明确错误，不 panic

## 阶段 3：glyph 获取、缓存与 atlas 分配

目标是把单个 rune 或 glyph index 转成可绘制的 `FontGlyph`。

当前状态：已完成 rune 级 glyph 光栅化、glyph cache、atlas page 行分配、atlas 子区域上传和字体 debug info 统计。当前仍未接入 HarfBuzz，后续可将缓存键从 rune 扩展到 shaping 输出的 glyph index。

参考 `copy_source` 的 atlas 策略：

- 每个 `Font` 持有多个 atlas page
- 小字号可用 512x512
- 中等字号可用 1024x1024
- 大字号可用 2048x2048
- atlas 按行分配区域，每个 glyph 周围保留 1 像素 padding

要做：

- `Font` 内部维护 glyph cache
- 缓存键第一版可用 `rune`
- glyph 缓存未命中时渲染 glyph 位图
- 将灰度 glyph 位图转换成 RGBA，RGB 为白色，A 为 glyph alpha
- 把 RGBA 数据上传到 atlas
- 记录 `UVRect`、`Size`、`Bearing`、`Advance`
- 对缺失 glyph 使用替换字符或 `?` 兜底

验收：

- 能加载 ASCII 字符 glyph
- 能加载中文字符 glyph
- 重复绘制同一字符不重复写入 atlas
- atlas 空间不足时能创建新 page
- glyph debug info 能看到 glyph 数和 atlas page 数

## 阶段 4：TextRenderer 最小闭环

目标是提供文本测量和绘制 API。

建议新增：

- `engine/render/textRenderer.go`

核心类型：

```go
type LayoutOptions struct {
    LetterSpacing    float32
    LineSpacingScale float32
    GlyphScale       mgl32.Vec2
}

type ShadowOptions struct {
    Enabled bool
    Offset  mgl32.Vec2
    Color   mgl32.Vec4
}

type TextRenderParams struct {
    Color  mgl32.Vec4
    Shadow ShadowOptions
    Layout LayoutOptions
}
```

要做：

- `MeasureText(text, fontKey, size, options)` 返回文本尺寸
- `DrawUIText(text, fontKey, size, position, params)` 绘制 UI 文本
- `DrawWorldText(text, fontKey, size, position, params)` 绘制世界文本
- 支持 `\n` 多行
- 支持颜色
- 支持阴影
- 支持字距、行距缩放、glyph 缩放
- 第一版按 rune 顺序布局，不做 HarfBuzz shaping

基础布局公式：

```text
baselineY = ascender + lineIndex * lineHeight * lineSpacingScale
destX = penX + bearingX
destY = baselineY - bearingY
penX += advance + letterSpacing
```

验收：

- 能显示英文
- 能显示中文
- 能显示多行文本
- UI 文本坐标不受 camera 影响
- 世界文本坐标受 camera 影响，并支持 viewport clipping

## 阶段 5：ResourceManager 接入

目标是让字体加载和查询统一挂在资源层下。

修改：

- `engine/resource/resourceManager.go`
- `engine/resource/debugInfo.go`

要做：

- `ResourceManager` 增加 `fonts *fontManager`
- `NewResourceManager(renderer)` 初始化字体管理器
- 增加 `LoadFont`
- 增加 `GetFont`
- 增加 `UnloadFont`
- 增加 `FontDebugInfo`
- `Clear()` 清理字体缓存
- 后续 `assets/data/resource_mapping.json` 可扩展 `font` section

建议接口：

```go
func (m *ResourceManager) LoadFont(key ResourceKey, path string, size int) (*render.Font, error)
func (m *ResourceManager) GetFont(key ResourceKey, size int) (*render.Font, bool)
func (m *ResourceManager) UnloadFont(key ResourceKey, size int)
func (m *ResourceManager) FontDebugInfo() []FontDebugInfo
```

实际类型归属可在实现时调整：字体对象若依赖渲染纹理，建议资源层持有内部字体类型，`TextRenderer` 通过资源层读取。

验收：

- 应用启动可预加载默认字体
- 资源清理时字体 atlas 被释放
- debug info 能展示字体 key、字号、glyph 数、atlas 页和估算内存

## 阶段 6：文本样式配置

目标是对齐 `copy_source/TinyFarm/config/text_render.json` 的样式配置，但第一版只实现已使用字段。

新增：

- `config/text_render.json`
- `engine/render/textRenderConfig.go`

建议第一版配置：

```json
{
  "text_renderer": {
    "layout_cache_capacity": 256,
    "default_style_keys": {
      "ui": "ui/default",
      "world": "world/default"
    },
    "styles": {
      "ui/default": {
        "color": "#FFFFFFFF",
        "shadow": {
          "enabled": true,
          "offset": [1.0, 1.0],
          "color": "#000000FF"
        },
        "layout": {
          "letter_spacing": 0.0,
          "line_spacing_scale": 1.0,
          "glyph_scale": [1.0, 1.0]
        }
      },
      "world/default": {
        "color": "#FFFFFFFF",
        "shadow": {
          "enabled": true,
          "offset": [1.0, 1.0],
          "color": "#000000FF"
        },
        "layout": {
          "letter_spacing": 0.0,
          "line_spacing_scale": 1.0,
          "glyph_scale": [1.0, 1.0]
        }
      }
    }
  }
}
```

保留但暂不生效：

- `direction`
- `language`
- `features`

这些字段等 HarfBuzz shaping 阶段再启用。

验收：

- 缺失配置时使用内建默认样式
- 配置解析错误返回明确错误
- 样式变更能影响新绘制文本
- 配置解析有测试覆盖

## 阶段 7：布局缓存

目标是避免每帧重复计算稳定文本的 glyph placement。

参考 `copy_source` 的 `TextLayout`：

```go
type GlyphPlacement struct {
    Glyph    *FontGlyph
    DestRect mgl32.Vec4
}

type TextLayout struct {
    Size       mgl32.Vec2
    Glyphs     []GlyphPlacement
    LineCount  int
    UsageFrame uint64
}
```

缓存键包含：

- font key
- font size
- text
- letter spacing
- line spacing scale
- glyph scale

要做：

- `TextRenderer` 内部维护 layout cache
- 配置 `layout_cache_capacity`
- 命中缓存时刷新 `UsageFrame`
- 超出容量时移除最久未使用布局
- 字体卸载、字体清空、样式布局参数变化时清缓存

验收：

- 重复测量同一文本命中缓存
- 样式布局参数变化后不会复用旧布局
- 缓存容量限制有效
- `go test ./...` 通过

## 阶段 8：启动链路与演示绘制

目标是在当前客户端中验证文本渲染闭环。

修改：

- `engine/core/gameApp.go`
- `game/entry.go` 或后续场景初始化代码

要做：

- 创建 `TextRenderer`
- 预加载默认字体 `assets/fonts/VonwaonBitmap-16px.ttf`
- 在 demo 渲染中绘制 UI 文本
- 绘制世界文本用于验证 camera 变换
- 绘制中文文本用于验证字体资源
- 绘制阴影文本用于验证样式
- 保留现有矩形、贴图、光照演示

建议验证文本：

```text
Tiny Farm
字体与文本渲染
World Label
Line 1
Line 2
```

验收：

- `go run .` 能看到 UI 文本和世界文本
- 窗口缩放时 UI 文本位置稳定
- camera 变化时世界文本随世界坐标变化
- 文本绘制不破坏已有 scene、lighting、emissive、bloom、ui pass

## 阶段 9：测试与调试信息

目标是先覆盖不依赖真实 OpenGL 上下文的逻辑，再为 atlas 和绘制保留 smoke test。

优先测试：

- 文本配置解析
- 十六进制颜色解析
- layout options sanitize
- layout cache key
- 多行尺寸计算
- `FontKey` 缓存逻辑
- `FontDebugInfo` 排序和字段
- 字体卸载后布局缓存失效

调试信息：

- 字体 key
- 字号
- 来源路径
- glyph 数
- atlas page 数
- atlas 尺寸
- 估算内存

验收：

- `go test ./...` 通过
- `go build .` 通过
- 运行时可通过日志或 debug info 观察字体缓存增长

## 阶段 10：复杂文本整形

目标是在基础文本系统稳定后，再补齐 `copy_source` 的 HarfBuzz 能力。

适用场景：

- 连字
- kerning
- 双向文本
- 阿拉伯文等复杂脚本
- 竖排文本
- 更准确的中文标点排版

要做：

- 评估 Go 侧 HarfBuzz 绑定或 shaping 库
- 启用 `direction`
- 启用 `language`
- 启用 `features`
- 布局缓存键加入 shaping 相关字段
- 字形获取从 rune 改为 glyph index
- 对齐 `copy_source` 的 `shapeLine` 思路

验收：

- `kern=1` 等 feature 生效
- 同一文本在不同 direction/language/features 下缓存隔离
- 基础中文和英文显示不回退

## 实施原则

- 第一版先完成“字体文件 -> glyph atlas -> 文本显示”的闭环
- 不在第一版引入完整 HarfBuzz 和 Debug UI
- 资源生命周期统一归 `ResourceManager`
- OpenGL 句柄不暴露给 game 层
- UI 文本和世界文本明确区分坐标空间
- glyph atlas 是显存资源，必须有 debug info 和释放路径
- 修改渲染初始化、资源释放或主循环后运行 `go test ./...`
- 当前使用的是相对帧率方案，提交说明里不要混淆为绝对时间对齐

## 近期最小任务

建议按下面顺序推进：

1. 在 OpenGL texture 层补空 atlas 创建和子区域上传
2. 实现 `FontManager` 的字体缓存和清理
3. 实现单 glyph 加载、atlas 分配和 `FontGlyph`
4. 实现 `TextRenderer.MeasureText`
5. 实现 `TextRenderer.DrawUIText`
6. 实现 `TextRenderer.DrawWorldText`
7. 接入 `ResourceManager`
8. 新增 `config/text_render.json`
9. 在 demo 中绘制中英文 UI 文本和世界文本
10. 补配置、缓存和资源调试测试
