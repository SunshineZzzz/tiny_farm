# 渲染器实施路线

## 目标

当前 Go 版本先完成一个可验证的 2D OpenGL 渲染闭环，再逐步参考 `copy_source/TinyFarm` 扩展到精灵、贴图、离屏渲染和后处理。

参考项目已经包含完整 C++ 渲染管线，但当前 Go 项目仍是客户端骨架。迁移时优先建立稳定边界，避免一次性搬入过多模块。

## 当前状态

- `engine/render/opengl/renderContext.go` 已负责 SDL OpenGL 上下文创建、函数加载和销毁
- `engine/render/opengl/viewportManager.go` 已负责 drawable 像素尺寸到逻辑分辨率的 letterbox viewport
- `engine/render/opengl/glRenderer.go` 已支持默认帧缓冲清屏、窗口缓冲交换和纯色矩形批量提交
- `engine/render/opengl/shaderProgram.go` 已支持从源码编译和链接 OpenGL shader program
- `engine/render/opengl/spriteBatch.go` 已支持简化版纯色矩形和贴图 SpriteBatch
- `engine/render/opengl/texture.go` 已支持 PNG 解码、OpenGL texture 创建和 src rect 到 UV 换算
- `engine/render/opengl/scenePass.go` 已支持 logical size FBO、color texture 和场景离屏清屏
- `engine/render/renderer.go` 已提供最小 facade，当前把 `game` 层和 `engine/render/opengl` 解耦
- `engine/render/camera.go` 已支持最小 Camera、world 到 logical 变换、viewport clipping 和 pixel snap
- `config/render.json` 已支持 `debug_context`，开发阶段可启用 OpenGL 调试包装
- `engine/core/gameApp.go` 的 `render()` 已接入 `Clear()` 和 `Present()`

## 参考范围

优先参考这些文件：

- `copy_source/TinyFarm/docs/rendering.md`
- `copy_source/TinyFarm/docs/resolution_and_viewport.md`
- `copy_source/TinyFarm/src/engine/render/opengl/gl_renderer.*`
- `copy_source/TinyFarm/src/engine/render/opengl/render_context.*`
- `copy_source/TinyFarm/src/engine/render/opengl/viewport_manager.*`
- `copy_source/TinyFarm/src/engine/render/opengl/shader_program.*`
- `copy_source/TinyFarm/src/engine/render/opengl/shader_library.*`
- `copy_source/TinyFarm/src/engine/render/opengl/sprite_batch.*`
- `copy_source/TinyFarm/src/engine/render/opengl/scene_pass.*`

暂缓参考这些模块：

- `lighting_pass.*`
- `emissive_pass.*`
- `bloom_pass.*`
- `composite_pass.*`
- `imgui_layer.*`
- UI 框架、ECS 渲染系统、资源管理器的完整实现

这些模块依赖较多，应在基础 sprite 管线稳定后再迁移。

## 阶段 1：清屏与 Present

目标是验证窗口、OpenGL context、函数加载、viewport 和主循环全部打通。

要做：

- 在 `GLRenderer` 增加 `Clear()` 方法
- 在 `GLRenderer` 增加 `Present()` 方法
- 在 `GameApp.render()` 调用 `Clear()` 和 `Present()`
- 清屏颜色先使用固定值，后续再进入配置

验收：

- `go test ./...` 通过
- `go run .` 能打开窗口
- 窗口背景颜色稳定可见
- `debug_context=true` 时控制台能看到 OpenGL 调用告警

## 阶段 2：直接绘制纯色矩形

目标是让渲染器能画出第一个可控图形。

要做：

- 新增 `shaderProgram.go`，封装 shader 编译、link、uniform 查询和释放
- 新增最小 sprite shader
- 在 `GLRenderer` 中创建 VAO/VBO/EBO
- 增加 `DrawRect` 或等价 API
- 先直接绘制到默认 framebuffer，不引入 FBO

验收：

- 窗口中能看到固定位置的纯色矩形
- 窗口 resize 后矩形仍按 logical 坐标稳定显示
- `debug_context=true` 时没有 OpenGL error panic

## 阶段 3：简化版 SpriteBatch

目标是把单次绘制扩展成可批量提交的精灵管线。

要做：

- 新增 `spriteBatch.go`
- 每个 sprite 生成 4 个顶点和 6 个索引
- CPU 端缓存 vertices、indices 和 draw commands
- `Flush()` 时统一上传 VBO/EBO 并调用 `DrawElements`
- 只合并提交顺序中连续使用同一纹理的 draw command
- 先支持纯色矩形和默认白纹理

暂时不做：

- 渐变
- 翻转
- 旋转
- 自发光
- pass 统计面板

验收：

- 一帧可提交多个矩形
- draw order 与提交顺序一致
- 连续相同纹理可以合并 draw call
- `go test ./...` 通过

## 阶段 4：纹理绘制

目标是支持图片精灵，为地图、角色和 UI 做准备。

要做：

- 增加 PNG 解码和 OpenGL texture 创建
- 设置纹理参数，`NEAREST` 和 `CLAMP_TO_EDGE`
- 增加 `DrawTexture(texture, dstRect, uvRect)`
- 支持 src rect 到 UV 的换算

验收：

- 能绘制一张 PNG
- 能从同一张图集中裁剪指定区域
- 像素风资源不糊边

## 阶段 5：ScenePass 与逻辑分辨率 FBO

目标是把世界绘制从默认 framebuffer 移到 logical size 的离屏缓冲。

要做：

- 新增 `scenePass.go`
- 创建 logical size FBO 和 color texture
- Scene sprite 先画到 FBO
- 暂时用最小 composite 把 FBO 输出到 letterbox viewport

验收：

- 窗口任意缩放时画面等比 letterbox
- 逻辑分辨率不随窗口尺寸改变
- 默认 framebuffer 的黑边区域被正确清理

## 阶段 6：Renderer 外观层

目标是区分“游戏层想画什么”和“OpenGL 后端怎么画”。

要做：

- 新增 `engine/render/renderer.go`
- 对外提供 `DrawRect`、`DrawTexture`、`BeginFrame`、`Present` 等高层 API
- OpenGL 细节留在 `engine/render/opengl`
- 预留相机、视口裁剪和排序入口

验收：

- `game` 层不直接依赖 OpenGL 包
- 后续可以接 ECS 渲染系统

## 阶段 7：相机、裁剪和像素对齐

目标是让世界坐标稳定映射到 logical 坐标。

要做：

- 新增 Camera 类型
- 实现 world 到 logical 的 view projection
- 实现 viewport clipping
- 实现 pixel snap 开关

验收：

- 相机移动时画面跟随
- 像素风资源移动时不明显抖动
- 视野外 sprite 不提交给后端

## 阶段 8：多 pass 管线

阶段 8 不再作为一次性任务推进，必须拆分实施。原因是 UI、合成、光照和后处理的依赖层次差异较大，一次性接入会放大状态管理和排错成本。

对照依据：

- 课程文档镜像：`copy_source/TinyFarm/docs/rendering.md` 第 8 节
- C++ 参考实现：`copy_source/TinyFarm/src/engine/render/opengl/gl_renderer.cpp`
- 当前 Go 实现：`engine/render/opengl/glRenderer.go`

课程文档与 `copy_source` 当前对应的完整顺序是：

1. `ScenePass`
2. `LightingPass`
3. `EmissivePass`
4. `BloomPass`
5. `CompositePass`
6. `UIPass`

当前 Go 版本与课程实现的差异：

- 已有 `ScenePass`，并且已经在 `Present()` 中完成“场景离屏渲染 → 输出到默认帧缓冲”
- 还没有独立的 `UIPass`
- 还没有独立的 `CompositePass`
- 还没有 `LightingPass`
- 还没有 `EmissivePass`
- 还没有 `BloomPass`
- 还没有多 pass 统一资源装配、开关与调试统计

因此，阶段 8 的任务不是“新增某一个 pass”，而是把当前 Go 版“ScenePass + 最小回贴屏幕”演进为与课程文档一致的多 pass 架构。

### 阶段 8 总体任务拆分

建议严格按下面顺序推进：

1. 先拆出 `CompositePass`
2. 再拆出 `UIPass`
3. 再接 `LightingPass`
4. 再接 `EmissivePass`
5. 最后接 `BloomPass`

说明：

- 课程文档里的最终执行顺序是 `CompositePass` 先于 `UIPass`
- 当前 Go 版虽然文档里先写了 `UIPass`，但从工程依赖看，应该先把“谁负责把 Scene 输出到屏幕”这件事从 `GLRenderer` 主流程里剥离出来
- 没有独立 `CompositePass`，后面的 `Lighting/Emissive/Bloom` 很容易全部耦合进 `GLRenderer.Present()`，后续结构会变乱

### 阶段 8 对照表

| 子阶段 | 课程 / copy_source 目标 | 当前 Go 状态 | 主要差距 |
| --- | --- | --- | --- |
| 8.1 `CompositePass` | 合成 Scene/Lighting/Emissive/Bloom 到默认帧缓冲 viewport | 由 `flushSceneTexture()` 直接回贴 `scenePass.texture()` | 还没有独立合成入口，也没有多输入纹理约束 |
| 8.2 `UIPass` | UI 精灵绘制到默认帧缓冲 viewport | UI 体系尚未接入渲染 pass | UI 与 Scene 尚未形成独立批处理边界 |
| 8.3 `LightingPass` | 生成 `light_color_tex`，承载环境光、方向光、点光、聚光 | 不存在 | 没有光照缓冲、没有光照提交通道、没有合成入口 |
| 8.4 `EmissivePass` | 生成 `emissive_color_tex`，承载发光遮罩 | 不存在 | 没有自发光缓冲、没有发光提交通道、没有与 Bloom 的衔接 |
| 8.5 `BloomPass` | 对 emissive 做降采样、模糊、上采样并输出 `bloom_tex` | 不存在 | 没有后处理缓冲链，也没有开关与成本控制 |

### 阶段 8.1：CompositePass

目标：

- 把“离屏纹理输出到默认帧缓冲”的逻辑从 `GLRenderer.Present()` 主流程中剥离出来
- 建立阶段 8 后续所有 pass 的统一合成入口
- 先只支持 `scene_color_tex` 输入，行为上保持与当前 Go 版一致

要做：

- 新增 `CompositePass`
- 把当前 `flushSceneTexture()` 里的“回贴 scene texture 到 viewport”迁移到 `CompositePass`
- 约定 `CompositePass` 的输入纹理接口，哪怕第一版只接 `scene_color_tex`
- 明确 `CompositePass` 工作在默认帧缓冲和 `letterbox viewport`
- 保持当前没有光照时的输出结果不变

验收：

- `GLRenderer.Present()` 不再直接自己拼 scene 输出矩形
- 默认渲染结果与当前版本一致
- 关闭后续特效时，基础画面仍完全可用

### 阶段 8.2：UIPass

目标：

- 把 UI 渲染从世界场景输出中独立出来
- 明确 UI 使用 logical 坐标，但输出到默认帧缓冲的 viewport
- 建立独立于 ScenePass 的 UI 批处理边界

要做：

- 新增 `UIPass`
- 让 `UIPass` 自己持有 UI 侧的 `SpriteBatch`
- 约定 UI 绘制发生在 `CompositePass` 之后
- 保留 UI 坐标按 logical 设计、最终绘制到 viewport 的口径
- 不在这一阶段引入完整 UI 框架，只先准备 pass 边界和最小调用链

验收：

- UI 与世界场景在渲染路径上彻底分离
- UI 在窗口缩放和 letterbox 下坐标稳定
- `ScenePass` 不再承担 UI 输出职责

### 阶段 8.3：LightingPass

目标：

- 为世界层引入独立光照缓冲和合成逻辑
- 对齐课程文档中的 `light_color_tex`
- 建立最小“光照作为颜色/亮度遮罩参与合成”的闭环

要做：

- 新增 `LightingPass`
- 新增 `light_color_tex` 对应的 logical FBO
- 先支持环境光和最小方向光，点光和聚光可分小步接入
- 在 `CompositePass` 中加入 `scene + light` 的最小合成公式
- 明确哪些数据属于 screen-space，哪些数据属于 world-space

验收：

- 能看到基础环境光变暗或提亮效果
- 光照关闭时，场景退回纯 `ScenePass` 输出
- `LightingPass` 的资源和输入边界独立明确

### 阶段 8.4：EmissivePass

目标：

- 为自发光内容提供独立绘制目标和合成入口
- 对齐课程文档中的 `emissive_color_tex`
- 为 Bloom 提供上游输入

要做：

- 新增 `EmissivePass`
- 新增 `emissive_color_tex` 对应的 logical FBO
- 建立最小发光矩形或发光贴图提交通道
- 在 `CompositePass` 中决定 emissive 的基础合成策略

验收：

- 发光内容可独立开关
- 不开启 Bloom 时，也能看到 emissive 基础效果
- emissive 输出不污染普通 scene color

### 阶段 8.5：BloomPass

目标：

- 在前面 pass 稳定后再接入后处理模糊和辉光
- 对齐课程文档中的 `bloom_tex`
- 把 Bloom 保持为可关的附加效果，而不是基础渲染路径依赖

要做：

- 新增 `BloomPass`
- 只处理 `emissive_color_tex`，不直接处理整张 scene color
- 建立最小降采样、模糊、上采样链路
- 在 `CompositePass` 中加入 bloom 输入和强度参数
- 预留 Bloom 开关和强度配置

验收：

- 无 emissive 时可直接跳过 Bloom
- 关闭 Bloom 时只保留 emissive 基础效果
- Bloom 的加入不破坏已有 Scene/Lighting/UI 输出顺序

### 阶段 8 总体约束

- 每个 pass 独立实现、独立验收、独立可关
- 先做 `CompositePass` 和 `UIPass`，再进入光照和后处理
- 后续新增 pass 时，不回退前面已完成的基础结构
- 每个持有批处理职责的 pass 自己持有自己的 `SpriteBatch`
- `CompositePass` 这类全屏合成 pass 不强制复用 `SpriteBatch`

总体验收口径：

- Scene 和 UI 分层清晰
- 光照、发光、Bloom 关闭时基础画面仍可用
- 各 pass 的资源释放完整

## 实施原则

- 每个阶段都要有可见结果或可测试结果
- 先保持 API 小，再根据真实需求扩展
- 不跨阶段迁移大型模块
- 新增 OpenGL 调用后优先用 `debug_context=true` 跑一遍
- 修改渲染初始化、主循环或 viewport 逻辑后运行 `go test ./...`
- 当前使用的是相对帧率方案，渲染路线说明和提交说明里不要混淆为绝对时间对齐

## 近期最小任务

阶段 1 到阶段 7 已完成。下一步按拆分后的阶段 8 推进：

- 阶段 8.1：新增 `CompositePass`
- 阶段 8.2：新增 `UIPass`
- 阶段 8.3 到 8.5：再继续接 `LightingPass`、`EmissivePass` 和 `BloomPass`
