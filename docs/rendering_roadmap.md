# 渲染器实施路线

## 目标

当前 Go 版本先完成一个可验证的 2D OpenGL 渲染闭环，再逐步参考 `copy_source/TinyFarm` 扩展到精灵、纹理、离屏渲染和后处理。

参考项目已经包含完整 C++ 渲染管线，但当前 Go 项目仍是客户端骨架。迁移时优先建立稳定边界，避免一次性搬入过多模块。

## 当前状态

- `engine/render/opengl/renderContext.go` 已负责 SDL OpenGL 上下文创建、函数加载和销毁
- `engine/render/opengl/viewportManager.go` 已负责 drawable 像素尺寸到逻辑分辨率的 letterbox viewport
- `engine/render/opengl/glRenderer.go` 已支持默认帧缓冲清屏、窗口缓冲交换和纯色矩形批量提交
- `engine/render/opengl/shaderProgram.go` 已支持从源码编译和链接 OpenGL shader program
- `engine/render/opengl/spriteBatch.go` 已支持简化版纯色矩形 SpriteBatch
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

建议行为：

```go
func (gr *GLRenderer) Clear() {
    gr.viewportManager.update()
    gl.ClearColor(...)
    gl.Clear(gl.COLOR_BUFFER_BIT)
}

func (gr *GLRenderer) Present() {
    gr.renderCtx.swapWindow()
}
```

验收：

- `go test ./...` 通过
- `go run .` 能打开窗口
- 窗口背景颜色稳定可见
- `debug_context=true` 时控制台能看到 OpenGL 调用名

## 阶段 2：直接绘制纯色矩形

目标是让渲染器能画出第一个可控图形。

要做：

- 新增 `shaderProgram.go`，封装 shader 编译、link、uniform 查询和释放
- 新增最小 sprite shader
- 在 `GLRenderer` 中创建 VAO/VBO/EBO
- 增加 `DrawRect(x, y, w, h, color)` 或等价 API
- 先直接绘制到默认 framebuffer，不引入 FBO

建议 shader 位置：

- `assets/shaders/sprite.vert`
- `assets/shaders/sprite.frag`

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
- 设置纹理参数：`NEAREST` 和 `CLAMP_TO_EDGE`
- 增加 `DrawTexture(texture, dstRect, uvRect)`
- 支持 src rect 到 UV 的换算

参考：

- `copy_source/TinyFarm/src/engine/resource/texture_manager.*`
- `copy_source/TinyFarm/docs/rendering.md` 中的 SpriteBatch/UV/Pixel Snap 说明

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
- 暂时用 blit 或简化 composite 把 FBO 输出到 letterbox viewport

参考：

- `copy_source/TinyFarm/src/engine/render/opengl/scene_pass.*`
- `copy_source/TinyFarm/docs/resolution_and_viewport.md`

验收：

- 窗口任意缩放时画面等比 letterbox
- 逻辑分辨率不随窗口尺寸改变
- 默认 framebuffer 的黑边区域被正确清理

## 阶段 6：Renderer 外观层

目标是区分“游戏层想画什么”和“OpenGL 后端怎么画”。

要做：

- 新增 `engine/render/renderer.go`
- 对外提供 `DrawSprite`、`DrawRect`、`BeginFrame`、`Present` 等高层 API
- OpenGL 细节留在 `engine/render/opengl`
- 预留相机、视口剔除和排序入口

参考：

- `copy_source/TinyFarm/src/engine/render/renderer.*`
- `copy_source/TinyFarm/src/engine/system/render_system.*`

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

参考：

- `copy_source/TinyFarm/src/engine/render/camera.*`
- `copy_source/TinyFarm/src/engine/render/opengl/gl_renderer.*`

验收：

- 相机移动时画面跟随
- 像素风资源移动时不明显抖动
- 视野外 sprite 不提交给后端

## 阶段 8：UI Pass、Composite 和后处理

目标是在基础渲染稳定后再接完整多 pass 管线。

建议顺序：

1. `UIPass`
2. `CompositePass`
3. `LightingPass`
4. `EmissivePass`
5. `BloomPass`

每个 pass 都应独立可关，避免调试困难。

验收：

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

阶段 1 已完成，阶段 2 已完成最小纯色矩形闭环，阶段 3 已完成简化版纯色矩形 SpriteBatch。下一步建议进入阶段 4：

- 增加 PNG 解码和 OpenGL texture 创建
- 支持 `DrawTexture(texture, dstRect, uvRect)`
- 支持 src rect 到 UV 的换算
