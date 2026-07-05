# Tiny Farm

这是一个使用 Go、SDL3 和 OpenGL 构建的农场游戏客户端，目前已完成主循环、输入、资源、渲染、场景管理和 ECS 基础架构

## 当前架构

```text
main.go                         程序入口
game/entry.go                   游戏层入口和初始场景装配
game/scene                     游戏场景及场景内 ECS World
game/factory                   游戏实体创建工厂
game/system                    玩家控制等游戏层系统
engine/core                    应用生命周期、主循环、配置和帧率控制
engine/context                 跨场景共享的运行时服务上下文
engine/scene                   场景接口、场景栈和安全切换
engine/ecs/component           通用 ECS 组件
engine/ecs/system              移动、删除和精灵渲染系统
engine/input                   SDL 输入事件和动作状态
engine/resource                纹理、音频和字体加载与缓存
engine/render                  相机、文本和通用渲染接口
engine/render/opengl           OpenGL 渲染管线和渲染资源
engine/ui                      UI 元素树、交互、按钮、预设和布局容器
engine/utils/dispatch          事件分发器
config                         窗口、渲染、输入和文本配置
assets                         纹理、音频、字体、地图和数据资源
docs                           ECS、渲染、资源和文本路线文档
```

通用引擎能力放在 `engine/...`，具体游戏逻辑放在 `game/...`

## 主循环

当前每帧按以下顺序执行：

```text
更新时间 -> 轮询输入和 SDL 事件 -> 更新活动场景 -> 渲染场景 -> 提交画面 -> 分发队列事件
```

队列事件在本帧渲染完成后统一分发，避免更新期间递归触发事件导致状态时序不一致

## ECS

项目使用 `github.com/yohamta/donburi` 管理实体和组件

- 每个游戏场景持有独立的 `donburi.World`
- `GameApp` 只管理主循环和跨场景服务，不持有 World
- `GameScene` 显式编排玩家控制、移动、集中删除和渲染系统
- 通用组件和系统放在 `engine/ecs`
- 游戏标签、工厂和玩法系统放在 `game`
- 精灵按 `layer`、`depth` 稳定排序后提交给渲染器
- 纹理只以资源 key 和路径记录在组件中，不把 OpenGL 对象放进 ECS

当前已经打通：

```text
输入 -> Velocity -> MovementSystem -> Transform -> RenderSystem
```

详细阶段和后续边界见 [ECS 接入路线](docs/ecs_roadmap.md)

## 运行

需要 Go 1.26，并从仓库根目录执行：

```powershell
cd D:\github\tiny_farm
$env:PATH='D:\github\tiny_farm;' + $env:PATH
go run .
```

`SDL3.dll` 当前放在仓库根目录，因此运行和测试时需要保证仓库根目录在 `PATH` 中

## 构建与测试

```powershell
cd D:\github\tiny_farm
$env:PATH='D:\github\tiny_farm;' + $env:PATH
go fmt ./...
go test ./...
go build .
```

如果系统 Go 构建缓存目录权限异常，可以临时使用仓库内缓存：

```powershell
New-Item -ItemType Directory -Force -Path .gocache, .gotmp | Out-Null
$env:GOCACHE='D:\github\tiny_farm\.gocache'
$env:GOTMPDIR='D:\github\tiny_farm\.gotmp'
$env:PATH='D:\github\tiny_farm;' + $env:PATH
go test ./...
```

测试后可以删除 `.gocache` 和 `.gotmp`

## 当前帧率方案

当前使用的是相对帧率方案

每帧根据上一帧到当前帧的实际耗时判断是否补等待时间，不做绝对时间点对齐。等待策略使用 SDL 粗等待加尾段短自旋，减少 CPU 空转并尽量降低 `DelayNS` 过冲影响

## 生命周期边界

- `GameApp` 负责 SDL 初始化、窗口、主循环和跨场景服务
- `renderContext` 负责 SDL OpenGL Context 的创建、绑定和销毁
- `Renderer` 负责帧开始、渲染管线和画面提交
- `ResourceManager` 负责纹理、音频和字体的加载、缓存与释放
- `SceneManager` 负责场景栈，并在更新安全时点执行切换
- `GameScene` 负责本场景的 World、系统顺序和实体生命周期
- 应用关闭时先释放场景和资源，再释放渲染器、窗口和 SDL

## 当前边界

1. 主画面仍保留部分硬编码图形、灯光和文本，用于验证渲染管线
2. 配置和资源路径仍依赖从仓库根目录启动
3. 当前只有一个游戏场景，场景栈能力已经建立，但还没有标题、暂停等实际切换流程
4. ECS 基础闭环已经完成，动画、碰撞、地图实体构建和存档将在具体玩法需要时接入
5. `SDL3.dll` 仍由仓库根目录提供，尚未整理独立发布目录
