# 字体与文本渲染路线图

## 对齐目标

本文档只对齐 `copy_source/TinyFarm` 的文本渲染主线，不把 UI 组件体系、按钮文字等能力提前纳入当前阶段

`copy_source` 的文本系统边界是：

```text
ResourceManager
  -> FontManager 管理 Font、glyph cache、glyph atlas
  -> TextRenderer 通过 HarfBuzz 生成 glyph layout
  -> GLRenderer 绘制 UI/world glyph 贴图
```

Go 版当前目标不是一次性完整搬运 C++ 版本，而是按同一边界逐步补齐：

- `FontManager` 负责字体资源、glyph 光栅化、glyph atlas 生命周期
- `TextRenderer` 负责文本测量、布局缓存、样式解析、UI/world 文本绘制
- `ResourceManager` 是字体资源统一入口
- `Renderer` 只负责提交 texture rect，不关心文本语义

## copy_source 实际能力

参考文件：

- `copy_source/TinyFarm/docs/text_rendering.md`
- `copy_source/TinyFarm/config/text_render.json`
- `copy_source/TinyFarm/src/engine/resource/font_manager.h`
- `copy_source/TinyFarm/src/engine/resource/font_manager.cpp`
- `copy_source/TinyFarm/src/engine/render/text_renderer.h`
- `copy_source/TinyFarm/src/engine/render/text_renderer.cpp`
- `copy_source/TinyFarm/src/engine/utils/defs.h`

参考实现已经具备：

- `FontManager` 按 `(font_id, pixel_size)` 缓存字体
- `Font` 封装 `FT_Face`、`hb_font_t`、字体度量、glyph cache、atlas pages
- `FontGlyph` 记录 texture、size、bearing、advance、uv_rect
- glyph cache 的 key 是 HarfBuzz 输出的 glyph index，不是 rune
- `TextRenderer` 使用 HarfBuzz 对 UTF-8 执行 shaping
- `TextLayout` 保存 `font`、`size`、`glyphs`、`line_count`、`usage_frame`
- `LayoutKey` 包含 `font_id`、`font_size`、`text`、`layout_options`
- `layout_cache_` 有容量限制和 LRU 风格裁剪
- 字体卸载或清空时，TextRenderer 监听事件并清理布局缓存
- `config/text_render.json` 支持 direction、language、features、layout_cache_capacity、styles
- 文本样式支持 color、shadow、layout
- 默认 UI/world style
- `layout_revision` 用于外部知道样式或布局相关配置已变化
- `drawUIText` 走 UI pass，`drawText` 走 world pass

参考实现当前 `LayoutOptions` 只有：

```cpp
float letter_spacing;
float line_spacing_scale;
glm::vec2 glyph_scale;
```

因此以下能力不属于当前 copy_source 文本主线：

- UI Label/Button text 集成

这些以后可以作为 UI 系统能力另开阶段，但不放进当前对齐路线

copy_source 当前存在局部业务层换行：

- `ItemTooltipUI::wrapText` 按像素宽度测量并插入换行
- `DialogueBubble::onShowEvent` 按字符数做简单换行，源码注释已标明 CJK/混排可能溢出

这类换行不是 `TextRenderer` 的通用 `LayoutOptions` 能力，因此不纳入本文档的文本渲染主线

## Go 当前已完成

当前 Go 代码中已经存在：

### 1. OpenGL 动态纹理能力

已完成：

- `Renderer.CreateEmptyTexture`
- `Texture.UpdateRGBA`
- atlas 子区域上传
- 动态纹理上传使用左上原点语义并转换到 OpenGL 上传坐标
- `Texture.Close` 可释放 atlas texture

对应文件：

- `engine/render/renderer.go`
- `engine/render/opengl/texture.go`

### 2. ResourceManager 接入 FontManager

已完成：

- `ResourceManager` 持有 `fonts *fontManager`
- `NewResourceManager(renderer, dispatcher...)` 初始化字体管理器并可选接入资源事件
- `LoadFont(key, pixelSize, paths...)`
- `FontDebugInfo`
- `Clear()` 清理字体缓存

对应文件：

- `engine/resource/resourceManager.go`
- `engine/resource/fontManager.go`
- `engine/resource/debugInfo.go`
- `engine/abstract/abstract.go`

### 3. Go 版 Font / FontGlyph / glyph atlas

已完成：

- 使用 Go 字体库解析 TTF
- 按 `key + pixelSize` 缓存字体
- 保存 ascender、descender、lineHeight、pixelSize
- rune 级 glyph cache
- glyph 位图转 RGBA
- atlas page 行分配
- atlas 空间不足时创建新 page
- `GlyphTexture`、`GlyphSize`、`GlyphBearing`、`GlyphAdvance`、`GlyphUVRect`
- 缺字时 fallback 到 `?`

与 copy_source 差异：

- Go 当前是 `rune -> glyph`
- copy_source 是 HarfBuzz shaping 后的 `glyph_index -> glyph`
- Go 当前没有 `hb_font_t`
- Go 当前没有 FreeType C 绑定

### 4. TextRenderer 最小闭环

已完成：

- `NewTextRenderer(resourceManager, renderer, dispatcher...)`
- `MeasureText`
- `DrawUIText`
- `DrawWorldText`
- `TextLayout`
- `GlyphPlacement`
- `LayoutKey`
- `layout_cache`
- `layout_cache_capacity`
- `usage_frame`
- 超容量时移除最久未使用 layout
- `config/text_render.json`
- `TextRenderer.loadConfig`
- 内建 `ui/default` 和 `world/default`
- `default_style_keys`
- styles 中的 color、shadow、layout
- 样式或布局配置变更时递增 `layout_revision`
- `TextStyleKey`
- `TextRenderOverrides`
- `SetTextStyle`
- `GetTextStyle`
- `HasTextStyle`
- `ListTextStyleKeys`
- `DefaultUIStyleKey`
- `DefaultWorldStyleKey`
- `LayoutRevision`
- 颜色调制
- `ColorOptions`
- 渐变颜色 `start_color` / `end_color` / `use_gradient` / `angle_radians`
- 阴影
- 多行 `\n`
- `LetterSpacing`
- `LineSpacingScale`
- `GlyphScale`
- UI/world 坐标通路区分
- demo 中可渲染基础 UI 文本

对应文件：

- `engine/render/textRenderer.go`
- `engine/core/gameApp.go`

与 copy_source 差异：

- Go 当前可从调用参数里的 style key 获取样式
- Go 当前 `DrawUIText` 可携带 `FontPath` 并隐式加载字体
- copy_source 的 `drawTextInternal` 不携带 `font_path`，通常先通过 `getTextSize(..., font_path)` 或资源层预加载字体
- Go 当前布局缓存保存 HarfBuzz shaping 后的 glyph index 布局

## Go 当前未实现

以下是 copy_source 已有、Go 还未实现或未完整实现的能力，后续应按优先级补齐：

### 1. HarfBuzz shaping

已完成：

- 使用 `github.com/go-text/typesetting/harfbuzz` 作为 Go 侧 HarfBuzz 等价实现
- `Font` 持有 HarfBuzz font 等价对象
- `TextRenderer` 解析 direction / language / features
- `shapeLine` 按 HarfBuzz 输出的 glyph index 和 position 生成布局
- 布局使用 x_advance / y_advance
- 布局使用 x_offset / y_offset
- kerning、ligature、复杂脚本、RTL 等能力交给 HarfBuzz shaping 结果处理

参考：

- `Font::getHBFont`
- `Font::getGlyphByIndex`
- `TextRenderer::shapeLine`

### 2. glyph index 级 FontGlyph

已完成：

- 根据 glyph index 加载 glyph
- glyph cache key 从 rune 切到 glyph index
- HarfBuzz 输出 glyph id 后查询 atlas glyph

参考：

- `Font::getGlyphByIndex`
- `Font::loadGlyph(uint32_t glyph_index)`

### 3. 渐变颜色

已完成：

- `ColorOptions`
- `start_color`
- `end_color`
- `use_gradient`
- `angle_radians`
- sprite batch 按四角投影生成顶点渐变色
- `TextRenderer` 绘制 glyph 时传递颜色参数

### 4. Debug panel

未实现：

- Text debug panel
- style 实时编辑
- layout_revision 展示
- layout cache 状态展示

参考：

- `src/engine/debug/panels/text_renderer_debug_panel.*`

当前 Go 可以先不做 Debug UI，因为 Go 侧 UI 系统还没开始

## 明确暂不做

以下 copy_source 已有能力不在当前 TextRenderer 对齐路线中：

- UI Label
- UIButton text
- tooltip 局部 word wrap
- dialogue bubble 简单换行

这些不是 copy_source 当前 `TextRenderer` 主线的 `LayoutOptions` 能力。等 UI 系统推进到对应阶段再做

## 后续实施路线

### 阶段 A：整理当前最小闭环

目标：确认当前已提交实现只保留 copy_source 主线中的基础能力

要做：

- 保持 `LayoutOptions` 只包含 `LetterSpacing`、`LineSpacingScale`、`GlyphScale`
- 保持 `LayoutOptions` 边界不扩展到 UI 组件能力
- 保留 demo 只用于验证基础文字渲染
- 不引入 UI Label / UIButton text / 业务层换行

验收：

- `go test ./...`
- `go build .`
- demo 能看到 UI 文本

### 阶段 B：Layout cache

目标：先对齐 copy_source 的布局缓存，不引入 shaping

已完成：

- 在 `TextRenderer` 内增加内部 `TextLayout`
- 增加 `GlyphPlacement`
- 增加 `LayoutKey`
- 缓存 key 包含 `fontKey`、`pixelSize`、`text`、`LayoutOptions`
- `MeasureText` 和 `DrawText` 共用 layout cache
- 增加 `layout_cache_capacity`
- 增加 `usage_frame`
- 超容量时移除最久未使用 layout

验收：

- 重复测量同一文本命中缓存
- layout 参数变化不复用旧缓存
- 容量限制生效

已执行：

- `go test ./...`
- `go build -o $env:TEMP\tiny_farm_check.exe .`

### 阶段 C：字体事件与缓存失效

目标：对齐 copy_source 的字体生命周期安全

已完成：

- 增加 `FontUnloadedEvent`
- 增加 `FontsClearedEvent`
- `ResourceManager.UnloadFont` 时发出单字体卸载事件
- `ResourceManager.Clear` / `ClearFonts` 时发出字体清空事件
- `TextRenderer` 监听事件并清理 layout cache

验收：

- 卸载字体后不会保留引用旧 glyph 的 layout
- 清空字体后 layout cache 为空
- `go test ./...`

已执行：

- `go build -o $env:TEMP\tiny_farm_check.exe .`

当前环境限制：

- `go test ./...` 在 `engine/resource` 测试包重新编译时会加载 SDL3 动态库
- 当前机器缺少 `SDL3.dll`，因此测试进程在包初始化阶段失败

### 阶段 D：text_render.json 与内建样式

目标：对齐 copy_source 的配置入口和默认样式

已完成：

- 新增 `config/text_render.json`
- `TextRenderer` 增加 `loadConfig`
- 解析 `layout_cache_capacity`
- 解析 `default_style_keys`
- 解析 `styles`
- 建立内建 `ui/default` 和 `world/default`
- 样式包含 color、shadow、layout
- 样式变更时递增 `layout_revision`

暂不启用：

- direction
- language
- features

这些等 HarfBuzz 阶段启用

验收：

- 配置缺失时使用内建默认样式
- 配置错误有明确错误
- 样式 layout 变化会清 layout cache

已执行：

- `go build -o $env:TEMP\tiny_farm_check.exe .`
- `go test ./engine/utils/dispatch ./engine/utils/event ./engine/abstract ./engine/context`

当前环境限制：

- `go test ./...` 在 `engine/resource` 测试包重新编译时会加载 SDL3 动态库
- 当前机器缺少 `SDL3.dll`，因此测试进程在包初始化阶段失败

### 阶段 E：Text style API

目标：对齐 copy_source 的样式访问接口

已完成：

- `SetTextStyle`
- `GetTextStyle`
- `HasTextStyle`
- `ListTextStyleKeys`
- `DefaultUIStyleKey`
- `DefaultWorldStyleKey`
- `LayoutRevision`
- `TextStyleKey`
- `TextRenderOverrides`
- `resolveTextRenderParams`

验收：

- 可通过 style key 绘制 UI/world 文本
- override 可覆盖颜色、阴影和 glyph scale
- style 变化可驱动 `layout_revision`

已执行：

- `go build -o $env:TEMP\tiny_farm_check.exe .`
- `go test ./engine/utils/dispatch ./engine/utils/event ./engine/abstract ./engine/context`

当前环境限制：

- `go test ./...` 在 `engine/resource` 测试包重新编译时会加载 SDL3 动态库
- 当前机器缺少 `SDL3.dll`，因此测试进程在包初始化阶段失败

### 阶段 F：HarfBuzz shaping 选型与接入

目标：补齐 copy_source 的核心文本整形能力

已完成：

- 选用 `github.com/go-text/typesetting/harfbuzz`
- `Font` 持有 HarfBuzz font 等价对象
- `TextRenderer` 增加 direction/language/features
- `shapeLine` 输出 glyph index 和 position
- `Font` 支持按 glyph index 取 glyph
- 从 rune layout 切换到 shaped glyph layout

说明：

- layout cache key 仍按 copy_source 主线保持为 font key、字号、文本和 `LayoutOptions`
- direction/language/features 变更时清空 layout cache

验收：

- kerning 由 shaping 结果体现
- ligature 可生效
- 基础中文、英文显示不回退
- 不手写 shaping

已执行：

- `go build -o $env:TEMP\tiny_farm_check.exe .`
- `go test ./engine/utils/dispatch ./engine/utils/event ./engine/abstract ./engine/context ./engine/render`

当前环境限制：

- `go test ./...` 在 `engine/resource` 测试包重新编译时会加载 SDL3 动态库
- 当前机器缺少 `SDL3.dll`，因此测试进程在包初始化阶段失败

### 阶段 G：Debug 信息增强

目标：先补非 UI 版调试信息，Debug UI 以后做

已完成：

- 输出 layout cache 容量、条目数
- 输出 layout revision
- 输出 text styles 列表
- 字体 debug info 保持 key、字号、glyph 数、atlas page、估算内存
- `TextRenderer.DebugInfo`
- 启动时输出 text renderer debug info

验收：

- 可以通过日志或 debug info 判断字体/glyph/layout cache 是否增长

已执行：

- `go test ./engine/render`
- `go build .`

## 近期优先级

TextRenderer 主线已完成。下一步建议转入 UI 系统阶段：

1. `UILabel`
2. `UIButton text`
3. `tooltip/dialogue 局部换行`

继续不建议作为当前 TextRenderer 主线补：

- ellipsis
- 文本框
- fallback font chain

这些在 copy_source 自有代码里没有对应的通用实现

## 提交说明要求

涉及这条路线的提交说明应至少写清楚：

- 本次对齐的是 copy_source 的哪个模块
- 当前是否仍然是 rune 级过渡布局
- 是否引入 layout cache 或 shaping
- 实际执行过的命令
- 当前帧率方案仍是相对帧率方案
