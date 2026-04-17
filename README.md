# Tiny Farm

这是一个精简版 Go 游戏客户端骨架，当前重点是把应用启动链路、主循环、相对帧率控制、事件分发和 SDL/OpenGL 初始化先接起来

## 当前结构

```text
main.go                         程序入口
game/entry.go                   游戏层入口和初始场景注册
engine/core                     应用生命周期、主循环、配置、帧率控制
engine/context                  运行时共享上下文
engine/render/opengl            SDL OpenGL 上下文和渲染器骨架
engine/utils/dispatcher         事件分发器
engine/utils/opengl             OpenGL 函数加载和调用接口
config/window.json              窗口、图形和性能配置
config/render.json              OpenGL 上下文初始化参数
```

## 运行

从仓库根目录执行

```powershell
cd D:\github\tiny_farm
$env:PATH='D:\github\tiny_farm;' + $env:PATH
go run .
```

`SDL3.dll` 当前放在仓库根目录，所以运行和测试时需要保证仓库根目录在 `PATH` 中

## 测试

```powershell
cd D:\github\tiny_farm
$env:PATH='D:\github\tiny_farm;' + $env:PATH
go test -p 1 ./...
```

如果系统 Go 构建缓存目录权限异常，可以临时把缓存放到仓库内

```powershell
New-Item -ItemType Directory -Force -Path .gocache, .gotmp | Out-Null
$env:GOCACHE='D:\github\tiny_farm\.gocache'
$env:GOTMPDIR='D:\github\tiny_farm\.gotmp'
$env:PATH='D:\github\tiny_farm;' + $env:PATH
go test -p 1 ./...
```

测试后可以删除 `.gocache` 和 `.gotmp`

## 当前帧率方案

当前使用的是相对帧率方案

每帧根据上一帧到当前帧的实际耗时判断是否补等待时间，不做绝对时间点对齐。等待策略使用 SDL 粗等待加尾段短自旋，目的是减少 CPU 空转，同时尽量降低 `DelayNS` 过冲影响

## SDL 和 OpenGL 生命周期边界

当前所有权约定如下

- `GameApp` 负责 `sdl.Init`、`sdl.CreateWindow`、`sdl.DestroyWindow`、`sdl.Quit`
- `renderContext` 负责 `sdl.GLCreateContext`、`sdl.GLMakeCurrent`、`sdl.GLDestroyContext`
- `gl.NewDefaultContext` 只创建 Go 侧 OpenGL 函数入口表，不创建第二个 SDL OpenGL context
- `gl.Context` 当前没有显式关闭接口，释放时只清空引用

## 已知遗留问题

1. 主循环还没有 SDL 事件轮询

   当前没有 `sdl.PollEvent`，窗口关闭、输入、窗口 resize 都不会被处理。后续需要在主循环前段补事件泵，并把 quit 事件转成 `isRunning = false`

2. 当前没有真正提交渲染帧

   OpenGL context 已经创建，函数入口也已加载，但主循环里还没有 clear、draw 和 swap。窗口可能创建成功但没有稳定显示内容。后续需要给 `GLRenderer` 增加最小 `Render`，至少执行清屏和 `GLSwapWindow`

3. 初始场景注册后还没有执行

   `game` 层会调用 `RegisterSceneSetup`，`GameApp` 也会检查是否已注册，但当前初始化链路里还没有调用这个回调。后续需要在引擎上下文准备好后执行场景装配

4. 配置路径仍然依赖当前工作目录

   当前通过 `config/window.json` 和 `config/render.json` 读取配置，要求从仓库根目录运行。后续可以改成基于可执行文件路径、工作目录探测，或显式配置根目录

5. 配置加载是全量覆盖策略

   `LoadFromFile` 会把 JSON 解析结果整体赋回配置对象。如果配置文件缺字段，缺失字段会用 Go 零值覆盖默认值。当前配置文件字段完整，所以能正常工作，后续可以改成默认值 merge

6. 窗口可见性还依赖运行环境

   Windows 下需要确保 `SDL3.dll` 能被进程加载。当前建议通过设置 `PATH` 解决，后续可以把运行说明、开发脚本或构建输出目录整理清楚
