# UI 框架基础路线图

## 目标

参考教程 `https://cppgamedev.top/courses/opengl-tiny-farm/parts/part-17` 和配套源码 `copy_source/TinyFarm` 的 UI 模块，给当前 Go 版本规划一条“先基础框架、后玩法控件”的迁移路线。

本路线图只描述后续实现顺序和验收标准，不包含代码实现。

核心目标是打通这条链路：

```text
Scene
  -> UIManager
  -> UIElement tree
  -> Panel / Image / Label / Button
  -> logical mouse hit-test
  -> hover / pressed / click
  -> preset / sound feedback
```

## 参考范围

教程入口：

- `https://cppgamedev.top/courses/opengl-tiny-farm/parts/part-17`

配套源码优先参考：

- `copy_source/TinyFarm/src/engine/ui/ui_element.h`
- `copy_source/TinyFarm/src/engine/ui/ui_manager.h`
- `copy_source/TinyFarm/src/engine/ui/ui_interactive.h`
- `copy_source/TinyFarm/src/engine/ui/ui_button.h`
- `copy_source/TinyFarm/src/engine/ui/ui_panel.h`
- `copy_source/TinyFarm/src/engine/ui/ui_image.h`
- `copy_source/TinyFarm/src/engine/ui/ui_label.h`
- `copy_source/TinyFarm/src/engine/ui/layout/`
- `copy_source/TinyFarm/src/engine/ui/state/`
- `copy_source/TinyFarm/src/engine/ui/behavior/`
- `copy_source/TinyFarm/src/engine/ui/ui_preset_manager.h`

Go 侧现有基础：

- `engine/input/input_manager.go` 已有逻辑鼠标坐标、动作状态查询、`OnAction()` 回调和 `mouse_left` 默认映射补全
- `engine/render/renderer.go` 已有 UI 坐标系矩形、贴图、source rect 和颜色调制绘制入口
- `engine/render/textRenderer.go` 已有 `MeasureText()`、`DrawUIText()`、文本样式和布局缓存
- `engine/scene/sceneManager.go` 已支持只更新栈顶场景、按栈顺序渲染场景
- `game/scene/gameScene.go` 当前还没有 UI manager 接入点
- `assets/data/ui_button_presets.json`、`assets/data/ui_image_presets.json` 已存在，可作为 preset 阶段的数据输入

## 架构边界

- `engine/ui` 负责元素树、布局、命中测试、交互状态、基础控件和 preset 解析
- `engine/render` 只负责通用渲染数据和绘制能力，例如 image 描述、source rect、nine-slice patch，不反向依赖 UI
- `game/scene` 按场景持有 UI manager，先不强行修改 `IScene` 接口
- `game/ui` 留给后续游戏玩法 UI，例如 HUD、背包、物品栏、对话框
- 调试 UI、拖拽系统、自定义鼠标、热重载不进入“UI 框架基础”首轮范围

## 阶段 0：对齐素材和数据契约

当前状态：已完成

要确认：

- `ui_button_presets.json` 和 `ui_image_presets.json` 的字段与 copy_source preset 概念是否一致
- 图片 preset 是否需要 texture key、source rect、颜色、nine-slice margin
- 按钮 preset 是否需要 normal、hover、pressed、disabled 四态视觉
- hover/click 音效 key 是否已经在资源映射里存在

验收：

- 输出一份字段映射说明，明确哪些字段首轮支持、哪些字段暂缓
- 不修改运行时代码

已确认的数据契约：

- button preset 和 image preset 的根节点都是对象，使用字符串 key 作为 preset 标识
- image definition 至少提供 `path` 或 `id`，`source` 使用 `[x, y, width, height]`
- `source` 宽高必须大于 0 才视为有效源矩形
- image definition 支持 `flipped`，默认值为 `false`
- image preset 的 `nine_slice` 放在自身，button 的 `nine_slice` 放在顶层并传播到四态图片
- button preset 必须提供 `images.normal`
- `images.hover`、`images.pressed`、`images.disabled` 都是可选字段，缺失时回退到已有状态图片
- button `label` 支持 `text`、`font_path`、`font_size`、`color`、`offset`
- `overrides.hover`、`overrides.pressed`、`overrides.disabled` 首轮只覆盖 label 的 `color` 和 `offset`
- `sounds.hover` 和 `sounds.click` 缺省时使用默认音效，字符串表示覆盖，`null` 表示禁用
- 默认 UI 音效使用资源 key `ui_hover` 和 `ui_click`
- 首轮不实现 preset 热重载、编辑器、自定义鼠标和玩法 UI

## 阶段 1：UIElement 树和布局基础

当前状态：已完成

对齐 `ui_element` 的最小能力：

- 元素基础属性：position、size、visible、needRemove、orderIndex、id
- 父子结构：add child、remove child、clear children、按 orderIndex 排序
- 布局属性：anchorMin、anchorMax、pivot、padding、margin
- 计算属性：screen position、bounds、content bounds、layout dirty
- 基础流程：递归 update、递归 render、render self hook
- 命中测试：point inside 和从后往前查找最上层交互元素

验收：

- 布局计算和 hit test 用纯单元测试覆盖
- 不依赖 SDL、OpenGL、真实纹理或真实字体
- 子节点排序和可见性规则清晰可测

## 阶段 2：Panel、Image、Label 基础控件

当前状态：已完成

先实现不可交互控件，验证 UI 坐标系绘制链路：

- `Panel`：纯色背景、可选背景图、作为 root element 的默认容器
- `Image`：texture key/path、source rect、颜色调制、可见性
- `Label`：文本内容、font key、字号、颜色、对齐或偏移，复用 `TextRenderer.MeasureText()` 和 `DrawUIText()`

验收：

- 场景里能渲染一个 panel、一个 image、一个 label
- Label 改文本后能更新测量结果
- 控件本身不处理鼠标交互

## 阶段 3：UIManager 接入场景生命周期

当前状态：已完成

对齐 `ui_manager` 的基础职责：

- 每个需要 UI 的 scene 自己持有一个 manager
- manager 持有 root panel，并提供 add、clear、update、render
- update 阶段处理逻辑鼠标位置、hover target、pressed target
- 订阅或查询 `mouse_left` pressed/released，用于 UI 点击状态机
- render 阶段在世界渲染后提交 UI 绘制

验收：

- `GameScene` 可以持有 UI manager 但不改变 `IScene`
- UI 能在世界画面之上绘制
- manager close 时释放输入回调连接，避免场景切换后残留回调

## 阶段 4：UIInteractive 和交互状态

当前状态：已完成

对齐 `ui_interactive` 和 `state/` 的核心行为：

- 支持 Normal、Hover、Pressed、Disabled 状态
- 支持 mouse enter、mouse exit、mouse pressed、mouse released
- 支持 clicked、hover enter、hover leave 回调
- pressed 后释放时，只有释放仍在同一控件内才触发 click
- 交互元素可禁用，禁用后不响应 hover/press/click
- 状态切换只改变 UI 自身，不直接修改游戏业务状态

验收：

- hover、pressed、release、disabled 状态转换有单元测试
- 重叠控件时，orderIndex 更高或更后绘制的控件优先命中
- UI 消费点击时，后续游戏输入穿透策略有明确记录

## 阶段 5：Button 控件

当前状态：已完成

对齐 `ui_button` 的首轮能力：

- 支持 click、hover enter、hover leave 回调
- 支持四态视觉：normal、hover、pressed、disabled
- 支持 label 文本、字体、字号、颜色、偏移
- 支持固定文本布局，`ScaleToFit` 可作为后续增强
- 支持通过 preset key 创建按钮，但首轮只覆盖按钮基础字段

验收：

- 能创建一个带文字的按钮
- hover 和 pressed 状态能切换视觉
- 点击回调只触发一次，不因 held 状态重复触发

## 阶段 6：Image 数据和 nine-slice

当前状态：已完成

对齐 copy_source 的 `render::Image` 和按钮皮肤需求：

- 抽象 image 描述：texture key/path、source rect、color、flip、nine-slice margins
- 普通 image 继续走现有 UI texture 绘制
- nine-slice 只做 patch 计算和绘制分发，不把 UI 逻辑放进 render 层
- 按钮和 panel 可以复用同一套 image 描述

验收：

- 普通贴图绘制行为不回退
- nine-slice patch 计算有单元测试
- 小尺寸目标矩形有清晰处理策略，避免负尺寸 patch

## 阶段 7：UIPresetManager

当前状态：已完成

对齐 `ui_preset_manager`，把硬编码样式移到数据：

- 解析 `assets/data/ui_image_presets.json`
- 解析 `assets/data/ui_button_presets.json`
- 使用字符串 key 作为 Go 侧 preset 主键
- 支持 image preset 复用
- 支持 button preset 引用四态 image、label 样式和 sound events
- preset 加载失败应返回明确错误，避免静默使用半成品样式

验收：

- 能列出并读取已有 button/image preset
- Button 可通过 preset key 创建
- 字段缺失时有默认值或明确错误，不出现半初始化控件

## 阶段 8：UI 音效反馈

当前状态：已完成

对齐 `UIInteractive` 的 sound event 概念：

- 默认 hover 音效：`ui_hover`
- 默认 click 音效：`ui_click`
- 支持按控件覆盖 hover/click 音效
- 支持禁用某个控件的某个音效事件
- 播放失败只记录日志，不影响 UI 状态机

验收：

- hover enter 只播放一次 hover 音效
- click 只在真实点击成立时播放
- 缺资源或播放失败不会阻断回调

## 阶段 9：Layout 容器

当前状态：已完成

对齐 `layout/` 的基础容器能力：

- `StackLayout`：横向/纵向、spacing、主轴对齐、交叉轴对齐
- `GridLayout`：固定列数、cell size、spacing
- 只布局 visible 子元素
- layout 容器只负责子元素位置，不负责业务逻辑

验收：

- 可排列菜单按钮列
- 可排列固定尺寸图标格子
- layout 更新不会破坏子元素手动设置的 size

## 暂缓内容

以下内容属于后续 UI 扩展，不放进“UI 框架基础”首轮：

- `DragBehavior`、`UIDragPreview`、`UIDraggablePanel`
- `UIItemSlot`、Inventory UI、Hotbar UI
- `ProgressBar`、`ScreenFade`
- Debug UI 面板
- 自定义鼠标 cursor
- UI preset 热重载
- UI 编辑器

## 建议实施顺序

1. 先做阶段 0 到阶段 3，拿到可绘制、可挂到场景的 UI 基础
2. 再做阶段 4 到阶段 5，拿到可交互按钮
3. 再做阶段 6 到阶段 8，让按钮样式和音效走数据
4. 最后做阶段 9，为菜单、背包和物品栏铺路

首个可验收里程碑建议定义为：

- `GameScene` 能显示一个 panel、label 和 button
- 鼠标 hover/press/click 状态正确
- button 点击能触发一个日志或测试回调
- 全过程不引入拖拽、背包、物品槽等玩法 UI
