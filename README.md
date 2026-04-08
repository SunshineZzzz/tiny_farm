# Tiny Farm Go Framework

这个仓库实现了一个对齐课程第 03 节思路的 Go 版游戏基础框架，核心目标不是先上图形库，而是先把启动链路和每帧节拍搭稳：

- `main -> game.Run -> core.App`
- 初始场景由游戏层注入，而不是引擎层硬编码
- 每帧顺序固定为 `Time -> Input -> Update -> Render -> Dispatch`
- `SceneManager` 通过 pending action 统一处理 Push/Pop/Replace

## 目录

```text
cmd/tinyfarm         程序入口
engine/config        配置
engine/core          App / Context
engine/event         trigger + queue dispatcher
engine/input         输入管理（当前用帧数上限模拟 Quit）
engine/render        渲染接口与控制台渲染器
engine/scene         Scene 抽象与 SceneManager
engine/timeutil      delta time 时钟
engine/utils         事件定义
game                 游戏层入口与初始场景注入
game/scenes          TitleScene
```

## 运行

```bash
go run ./cmd/tinyfarm
go run ./cmd/tinyfarm --frames=3
```

默认会进入 `TitleScene`，跑若干帧后自动退出，方便你先验证从入口到第一帧的完整控制流。

## 下一步

如果你要继续往课程后续章节推进，建议按下面顺序扩展：

1. 把 `console renderer` 替换为真实图形后端，例如 Ebiten、SDL2 或 OpenGL 绑定。
2. 给 `input.Manager` 接入真实窗口事件，而不是用帧数触发退出。
3. 在 `Context` 里继续注入资源管理、音频、摄像机和 ECS world。
4. 为 `SceneManager`、Dispatcher、输入映射补更多测试。
