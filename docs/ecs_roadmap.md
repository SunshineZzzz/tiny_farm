# ECS 接入路线

## 目标

当前 Go 版本使用 `github.com/yohamta/donburi` 接入 ECS，但不一次性照搬教程和
`copy_source/TinyFarm` 的完整实现。

第一条主线只建立一个可见、可测试的闭环：

`游戏层创建实体 -> 输入系统写速度 -> 移动系统写位置 -> 渲染系统提交矩形`

完成该闭环后，再按真实玩法需求增加精灵、动画、碰撞、地图、实体工厂和场景管理。

## 当前进度

- 阶段 0 已完成：运行时 Context、ECS World 和游戏层装配入口已经接通
- 阶段 1 主体已完成：最小组件和演示实体已经落地，组件读写和组合查询测试待补
- 阶段 2 主体已完成：MovementSystem 已接入 `GameApp.update()`，系统单元测试待补
- 阶段 3 尚未开始：当前渲染仍使用 `GameApp.render()` 中的硬编码演示绘制
- 当前主循环继续使用相对帧率方案

## 结论

现在适合引入 ECS，但只适合引入最小骨架。

接入前的基础条件：

- 当前已经有主循环、`deltaTime`、输入、相机、渲染和资源管理器，可以支撑 ECS 系统运行
- 接入前 `GameApp.update()` 还是空实现，`render()` 中仍是硬编码演示绘制，适合逐步替换
- 当前没有可用的 Scene/SceneManager，不能直接采用参考项目的“一场景一 World”完整结构
- 接入前 `engine/context.Context` 还是空结构，需要先建立窄口运行时服务上下文
- 接入前 `sceneSetup` 只有注册没有执行，需要建立实际的游戏层装配入口
- 地图加载、碰撞、动画、蓝图和存档尚未形成稳定基础设施，过早迁移对应组件和系统只会制造空壳

因此先让 `GameApp` 临时持有一个 World，等 Scene 生命周期落地后，再把 World 的所有权迁移到 Scene。

## 参考边界

教程入口：

- `https://cppgamedev.top/courses/opengl-tiny-farm/parts/part-08`

本地参考项目优先阅读：

- `copy_source/TinyFarm/docs/ecs.md`
- `copy_source/TinyFarm/docs/game_scene.md`
- `copy_source/TinyFarm/src/engine/scene/scene.*`
- `copy_source/TinyFarm/src/game/scene/game_scene.*`
- `copy_source/TinyFarm/src/engine/component/transform_component.h`
- `copy_source/TinyFarm/src/engine/component/velocity_component.h`
- `copy_source/TinyFarm/src/engine/component/render_component.h`
- `copy_source/TinyFarm/src/engine/system/movement_system.*`
- `copy_source/TinyFarm/src/engine/system/render_system.*`
- `copy_source/TinyFarm/src/engine/system/remove_entity_system.*`

参考项目最值得保留的是以下原则，而不是 C++/EnTT 的具体 API：

- `Context` 保存跨场景服务
- World/Registry 保存场景状态
- 通用组件和系统放在 `engine`
- 玩法组件和系统放在 `game`
- 系统顺序显式编排
- 结构性修改集中处理，避免在查询迭代中随意删除实体或组件

## Donburi 使用策略

建议依赖：

```text
github.com/yohamta/donburi
```

接入时固定一个明确版本，不跟随 `latest` 漂移。

第一阶段只使用 donburi 核心包：

- `donburi.NewWorld`
- `donburi.NewComponentType`
- `donburi.NewTag`
- `donburi.NewQuery`
- `filter.Contains`

暂时不使用：

- `github.com/yohamta/donburi/ecs` 的实验性系统调度层
- `features/transform` 的父子层级
- donburi events
- ordered query
- debug UI
- 序列化辅助

当前主循环已有明确时序，直接调用系统函数更容易理解、测试和控制：

```go
playerControlSystem.Update(world, ctx, deltaTime)
movementSystem.Update(world, deltaTime)
```

查询应在包级或系统构造阶段创建并复用，不要每帧重复创建同一个 Query。

## 所有权边界

### 当前过渡阶段

- `GameApp` 持有唯一 `donburi.World`
- `Context` 暴露输入、渲染、资源、相机、事件和游戏状态等服务
- 游戏层装配函数接收 World 和 Context，创建初始实体
- `GameApp.update()` 显式执行更新系统
- `GameApp.render()` 在 `BeginFrame/Clear` 之后执行 ECS 渲染系统

### Scene 落地后

- 每个 Scene 持有自己的 `donburi.World`
- Scene 创建并持有本场景需要的系统
- Scene 决定系统执行顺序
- `Context` 不持有 World，避免把场景状态错误提升为全局状态
- Scene 销毁时整体释放 World，不复用旧实体引用

不要把 World 塞进全局单例，也不要让 `ResourceManager`、`Renderer` 反向依赖 ECS。

## 建议目录

```text
engine/
  context/
    context.go
  ecs/
    component/
      transformComponent.go
      velocityComponent.go
      shapeRenderComponent.go
      tags.go
    system/
      movementSystem.go
      render.go
      remove_entity.go

game/
  component/
    tags.go
  system/
    player_control.go
  world/
    bootstrap.go
```

目录名使用 `engine/ecs` 是为了表达这些组件和系统依赖 donburi。不要创建一个新的自研 World
包装层，除非后续确实需要隔离 donburi API。

## 阶段 0：打通游戏层装配入口

目标是先建立 ECS 可以接入的生命周期位置。

要做：

- 让 `GameApp.init()` 在所有基础服务初始化完成后创建 `Context`
- 实际调用已注册的 `sceneSetup`
- 调整装配函数签名，使其能够访问 World 和 Context
- 明确初始化失败如何返回错误，避免装配函数只能记录日志
- 给 Context 提供窄口服务访问，不直接暴露 `GameApp`

建议 Context 首批字段：

```go
type Context struct {
    Input     *input.InputManager
    Renderer  *render.Renderer
    Resources *resource.ResourceManager
    Camera    *render.Camera
    Dispatcher *dispatch.Dispatcher
    GameState abstract.IGameState
}
```

验收：

- 游戏层装配回调只执行一次
- 回调执行时所有字段均已初始化
- 初始化失败时应用退出且资源正常释放
- `go test ./...` 通过

## 阶段 1：World 与最小组件

目标是验证 donburi 的创建、查询和组件读写，不接玩法复杂度。

新增通用组件：

```go
type TransformComponent struct {
    Position mgl32.Vec2
    Scale    mgl32.Vec2
    Rotation float32
}

type VelocityComponent struct {
    Value mgl32.Vec2
}

type ShapeRenderComponent struct {
    Size  mgl32.Vec2
    Color mgl32.Vec4
}
```

首批只使用纯色矩形，暂时不引入 `Sprite`、纹理 key、pivot、图层和深度。

要做：

- `GameApp` 创建一个 World
- 游戏层创建一个带 `Transform + Velocity + ShapeRender` 的演示实体
- 为组件类型设置稳定、可读的名称
- 增加 World 创建和组件读写测试

验收：

- 可以创建实体并读回三个组件
- 查询只返回满足组件组合的实体
- 测试不需要 SDL 窗口和 OpenGL 上下文

## 阶段 2：MovementSystem

目标是让 ECS 第一次驱动状态变化。

规则：

```text
Position += Velocity * deltaTime
```

要做：

- 在 `engine/ecs/system` 增加移动系统
- 查询 `Transform + Velocity`
- 使用传入的 `deltaTime`，不在系统内部读取时钟
- 给零速度、正负速度和不同 `deltaTime` 增加单元测试
- 从 `GameApp.update()` 显式调用该系统

暂时不做：

- 碰撞
- 空间索引
- `TransformDirtyTag`
- 固定时间步
- 插值和预测

验收：

- 演示实体的位置随帧更新
- 系统测试可完全脱离渲染运行
- 当前主循环继续使用相对帧率方案

## 阶段 3：ShapeRenderSystem

目标是用 ECS 替换一部分 `GameApp.render()` 的硬编码矩形。

要做：

- 查询 `Transform + ShapeRender`
- 将组件转换为 `Renderer.DrawWorldRect` 调用
- 保持 `BeginFrame -> Clear -> ECS RenderSystem -> Present` 顺序
- 先保持查询自然顺序，不承诺实体间稳定绘制顺序
- 逐步删除已经由 ECS 实体替代的演示矩形

系统只负责从组件生成绘制命令，不负责：

- 加载资源
- 创建纹理
- 调用 `Present`
- 修改游戏状态
- 持有 World

验收：

- 屏幕上的矩形位置来自 ECS `Transform`
- MovementSystem 更新后，矩形在同一帧的新位置渲染
- ECS 渲染系统可以用假的绘制接口做单元测试，或至少把命令生成部分拆成纯逻辑测试

## 阶段 4：PlayerControlSystem

目标是建立第一条真实输入链路：

`InputManager -> Velocity -> MovementSystem -> Transform -> RenderSystem`

要做：

- 在 `game/component` 增加 `PlayerTag`
- 在 `game/system` 增加 `PlayerControlSystem`
- 查询 `PlayerTag + Velocity`
- 根据已有 action 状态设置速度
- 无方向输入时把速度归零
- 对角移动是否归一化要形成明确规则并测试
- 速度数值先放在系统配置或简单 gameplay 组件中

系统顺序：

```text
PlayerControlSystem
MovementSystem
ShapeRenderSystem
```

验收：

- 一个带 `PlayerTag` 的实体可以由输入移动
- 非玩家实体不受 PlayerControlSystem 影响
- 输入、移动和渲染职责没有混在同一个系统中

## 阶段 5：最小生命周期与删除队列

目标是规范实体删除，提前避免遍历期间结构变更问题。

要做：

- 增加 `NeedRemoveTag`
- 业务系统只标记待删除实体
- `RemoveEntitySystem` 在更新阶段最先或最后集中删除
- 删除前先收集实体，再执行移除
- 不跨帧长期保存 `*donburi.Entry`
- 如需长期引用，只保存 `donburi.Entity`，使用前重新取得 Entry 并检查有效性

验收：

- 标记实体在规定时点被删除
- 同一帧多个实体删除不漏删、不重复删
- 删除后旧实体引用不会继续参与系统

## 阶段 6：SpriteRenderSystem

进入条件：

- ECS 纯色实体闭环稳定
- ResourceManager 的纹理 key 和缓存行为稳定
- Renderer 的纹理绘制 API 不再频繁变化

新增组件建议：

```go
type Sprite struct {
    TextureKey resource.ResourceKey
    SourceRect mgl32.Vec4
    Size       mgl32.Vec2
    Pivot      mgl32.Vec2
}

type RenderOrder struct {
    Layer int
    Depth float32
}
```

要做：

- Sprite 只保存资源 key 和绘制数据，不保存 OpenGL 对象
- RenderSystem 通过 ResourceManager 查询纹理
- 缺失纹理采用明确策略：跳过并记录一次告警，或绘制占位图
- 真正需要遮挡关系时再引入 layer/depth 排序
- Y-sort 独立成系统或命令生成步骤，不写进 Transform

暂时不直接采用 ordered query。先收集渲染项并排序，规则更直观，也便于同时处理多个组件来源。

## 阶段 7：Scene 与 World 生命周期

进入条件：

- 至少有标题场景和游戏场景的真实切换需求
- 当前临时 World 已经不适合同时承载不同页面状态

要做：

- 增加 `Scene` 接口和 `SceneManager`
- `GameScene` 持有 World 和系统集合
- `TitleScene` 可以不使用 ECS
- 场景更新和渲染由 SceneManager 转发
- 场景切换在安全时点统一应用
- 把 World 所有权从 `GameApp` 迁移到 `GameScene`

迁移完成后的边界：

```text
GameApp: 主循环和全局服务
SceneManager: 场景栈与切换
GameScene: World、系统顺序、玩法状态
Context: 跨场景服务
```

## 阶段 8：按需求扩展，而不是按教程目录扩展

以下能力应由具体玩法需求触发。

### 动画

进入条件：

- SpriteRenderSystem 已稳定
- 角色确实需要帧动画

再增加：

- `Animation`
- `AnimationSystem`
- 动画关键帧事件
- 状态到动画的映射

### 碰撞与空间索引

进入条件：

- 有地图阻挡或实体交互需求
- 直接遍历已经成为明确问题

再增加：

- `AABBCollider` 或 `CircleCollider`
- 碰撞解析器
- 动态空间索引
- `TransformDirtyTag`

### 地图与实体构建器

进入条件：

- Tiled 地图加载链路已确定
- 地图对象到组件的映射规则已明确

再增加：

- 地图实体构建器
- 游戏层 EntityFactory
- blueprint/config 到组件的转换
- 地图切换和地图作用域实体

### 存档

进入条件：

- 组件模型和实体身份规则稳定

不要直接序列化整个 donburi World。存档应使用独立 DTO，明确保存哪些玩法状态，再由加载流程重建实体。

## 当前明确暂缓

- 完整 Actor、NPC、Crop、Inventory、Hotbar 等玩法组件
- 动画事件系统
- 自动图块
- 碰撞和空间索引
- 地图切换
- 昼夜和 ECS 光照系统
- ECS Debug UI
- 父子 Transform
- ECS World 自动序列化
- 多线程系统调度
- 通用 System 接口和复杂 scheduler
- 依赖注入容器

这些能力不是 ECS 接入的前置条件。

## 测试策略

优先测试纯 ECS 逻辑：

- World 创建和组件初始化
- Query 过滤是否正确
- MovementSystem 的数值结果
- PlayerControlSystem 是否只影响玩家
- RemoveEntitySystem 的删除时点
- 渲染命令生成是否正确

集成测试只覆盖关键接点：

- 游戏层装配回调是否执行
- 系统顺序是否符合约定
- update 后 render 读取到最新 Transform
- World 在生命周期结束后不再被使用

每个阶段至少执行：

```text
go fmt ./...
go test ./...
go build .
```

涉及窗口和输入链路时再执行：

```text
go run .
```

## 推荐的第一批提交

### 提交 1：补齐运行时装配入口

- 实现 Context
- 调用 `sceneSetup`
- 为初始化错误和调用次数补测试

### 提交 2：接入 donburi 最小 World

- 固定 donburi 版本
- 增加 Transform、Velocity、ShapeRender
- 创建演示实体
- 增加组件和 Query 测试

### 提交 3：移动系统

- 增加 MovementSystem
- 接入 `GameApp.update()`
- 增加纯逻辑测试

### 提交 4：矩形渲染系统

- 增加 ShapeRenderSystem
- 替换部分硬编码演示绘制
- 验证移动实体可见

### 提交 5：玩家输入闭环

- 增加 PlayerTag 和 PlayerControlSystem
- 完成输入到移动到渲染的闭环

做到提交 5 后，才算真正完成 ECS 的第一阶段接入。此时再决定优先建设 Scene、Sprite
还是碰撞，不需要现在预先实现整套参考项目架构。
