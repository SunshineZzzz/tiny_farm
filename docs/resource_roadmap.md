# Resource 实施路线

## 目标

当前 Go 版本先实现一个最小可用的资源外观层，把资源加载、缓存、查找和释放从 `Renderer`、后续 `AudioPlayer` 以及游戏逻辑里拆出来。
参考 `copy_source/TinyFarm` 的 Resource 系统，但当前只迁移 `texture`、`audio` 和 `ResourceManager`，字体、UI preset、地图、自动图块等资源类型后续再扩展。

核心边界：

- `ResourceManager` 是对外 facade，游戏层和系统层优先只依赖它
- `TextureManager` 只负责纹理资源的加载、缓存、尺寸、卸载和调试信息
- `AudioManager` 只负责音频文件解码后的 buffer 缓存，具体播放由后续 AudioPlayer 负责
- `Renderer` 继续负责“怎么画”，不负责资源缓存策略
- 后续 AudioPlayer 负责“怎么播”，不负责资源映射和缓存策略

## 当前状态

- `assets/data/resource_mapping.json` 已包含 `sound`、`music`、`texture` 三类 key 到文件路径的映射
- `engine/render/renderer.go` 已提供 `LoadTexture(path)`，但纹理生命周期仍由创建方手动管理
- `engine/render/opengl/texture.go` 已支持 PNG 解码、OpenGL texture 创建、尺寸查询和释放
- `engine/core/gameApp.go` 当前直接通过 `renderer.LoadTexture("assets/tests/Button Normal.png")` 加载演示纹理
- Go 侧尚无 `engine/resource` 包，也没有统一资源 debug info
- Go 侧尚无音频解码和播放抽象，SDL 已在启动时初始化 `sdl.InitAudio`

## 参考范围

优先参考这些文件：

- `copy_source/TinyFarm/docs/resources.md`
- `copy_source/TinyFarm/src/engine/resource/resource_manager.*`
- `copy_source/TinyFarm/src/engine/resource/texture_manager.*`
- `copy_source/TinyFarm/src/engine/resource/audio_manager.*`
- `copy_source/TinyFarm/src/engine/resource/resource_debug_info.h`
- `github.com/gopxl/beep/v2`
- `assets/data/resource_mapping.json`
- `engine/render/renderer.go`
- `engine/render/opengl/texture.go`

暂缓迁移：

- `FontManager`
- `UIPresetManager`
- `AutoTileLibrary`
- Debug UI 面板
- AudioPlayer 和混音播放链路
- 资源热重载、异步加载、引用计数驱逐策略

## API 边界

建议 Go 侧目录结构：

- `engine/resource/resourceManager.go`
- `engine/resource/textureManager.go`
- `engine/resource/audioManager.go`
- `engine/resource/resourceMapping.go`
- `engine/resource/debugInfo.go`

建议对外接口先保持窄口：

```go
type ResourceManager struct {
    textures *TextureManager
    audio    *AudioManager
}

func NewResourceManager(renderer *render.Renderer) (*ResourceManager, error)
func (m *ResourceManager) LoadResources(path string) error
func (m *ResourceManager) Clear()

func (m *ResourceManager) LoadTexture(key ResourceKey, path string) (*render.Texture, error)
func (m *ResourceManager) GetTexture(key ResourceKey) (*render.Texture, bool)
func (m *ResourceManager) TextureSize(key ResourceKey) (mgl32.Vec2, bool)
func (m *ResourceManager) UnloadTexture(key ResourceKey)

func (m *ResourceManager) LoadSound(key ResourceKey, path string) (AudioBufferHandle, error)
func (m *ResourceManager) GetSound(key ResourceKey) (AudioBufferHandle, bool)
func (m *ResourceManager) UnloadSound(key ResourceKey)

func (m *ResourceManager) LoadMusic(key ResourceKey, path string) (AudioBufferHandle, error)
func (m *ResourceManager) GetMusic(key ResourceKey) (AudioBufferHandle, bool)
func (m *ResourceManager) UnloadMusic(key ResourceKey)
```

`ResourceKey` 建议先用 `string`，不要急着移植 C++ 的 `entt::hashed_string` 策略。
原因是 Go 当前没有 ECS/resource ID 体系，字符串 key 更容易测试和排查；后续如果需要数字 ID，可以在 `ResourceKey` 内部增加稳定 hash，而不影响外部调用。

## 阶段 1：资源映射解析

目标是先把 `resource_mapping.json` 的格式固定下来，并能被测试覆盖。

要做：

- 新增 `resourceMapping` 结构体，解析 `sound`、`music`、`texture`
- 对 `ui_button_presets`、`ui_image_presets` 先保留字段但不加载
- 校验 section 类型，错误信息带上字段名和文件路径
- 路径先保持配置里的相对路径，默认从仓库根目录运行

验收：

- `go test ./...` 通过
- 能解析当前 `assets/data/resource_mapping.json`
- 非对象 section、非字符串 value、缺失文件都有测试

## 阶段 2：TextureManager

目标是把纹理缓存从 `Renderer.LoadTexture` 的调用方挪到资源层。

要做：

- 新增 `TextureManager`，内部持有 `map[ResourceKey]*textureEntry`
- `textureEntry` 保存 `*render.Texture`、来源路径、宽高、估算显存字节数
- `LoadTexture(key, path)` 命中缓存时直接返回已有纹理
- `GetTexture(key)` 只查缓存，不隐式把 key 当路径加载
- `UnloadTexture(key)` 调用 `Texture.Close()`
- `ClearTextures()` 释放全部纹理
- 保留 `Renderer.LoadTexture(path)` 作为底层创建入口，但游戏层后续不直接使用它

需要配套的小改动：

- 给 `render.Texture` 增加 `Size() mgl32.Vec2` 或等价方法
- 如果不想暴露 OpenGL 细节，继续不暴露 texture ID

验收：

- 重复加载同一个 key 不会创建第二份纹理
- 卸载后再次加载能重新创建纹理
- `ClearTextures()` 后旧 texture 不再可用
- `go test ./...` 通过

## 阶段 3：ResourceManager facade

目标是让上层只访问 `ResourceManager`，具体 manager 作为内部实现细节。

要做：

- 新增 `ResourceManager`，构造时接收 `*render.Renderer`
- `LoadResources(path)` 按 mapping 预加载 texture/sound/music
- 对外提供 texture/sound/music 的 load/get/unload/clear 方法
- `Clear()` 按音频、纹理的顺序释放缓存
- `GameApp` 持有 `resourceManager`，演示纹理改成从资源层取

验收：

- `GameApp` 不再直接管理演示纹理生命周期
- `renderer.Close()` 前先 `resourceManager.Clear()`
- `go test ./...` 通过
- `go run .` 能正常显示当前演示纹理

## 阶段 4：AudioManager

目标是先建立音频 buffer 资源缓存，不做播放。
Go 侧音频解码和 buffer 抽象优先使用 `github.com/gopxl/beep/v2`，用于覆盖当前资产里的 `wav`、`mp3`、`ogg` 等格式。
资源层只持有可复用的音频 buffer 或 handle，不直接调用 speaker 播放接口。

要做：

- 新增 `AudioBufferHandle`，内部优先包装 `*beep.Buffer`、`beep.Format` 和 `sourcePath`
- 新增 `AudioManager`，分别缓存 `sounds` 和 `music`
- `LoadSound` / `LoadMusic` 命中缓存时直接返回已有 buffer
- `GetSound` / `GetMusic` 只查缓存
- 解码方案单独封装在资源层内部，避免播放系统依赖文件格式细节

开放问题：

- `beep/speaker` 是否作为后续 AudioPlayer 的播放后端，需要在播放链路阶段确认
- 如果 beep 解码格式覆盖不足，再针对具体格式补充专用解码器
- 重采样、音量、循环和淡入淡出策略留给后续 AudioPlayer 阶段处理

验收：

- 能加载至少一个 `wav` 音效
- 不支持格式返回明确错误，不 panic
- `sound` 和 `music` 缓存互相隔离
- `go test ./...` 通过

## 阶段 5：调试信息

目标是对齐参考实现的可观测性，但不引入 Debug UI。

要做：

- 新增 `TextureDebugInfo`
- 新增 `AudioDebugInfo`
- `ResourceManager.TextureDebugInfo()` 返回按 key 排序的纹理信息
- `ResourceManager.SoundDebugInfo()` 和 `MusicDebugInfo()` 返回按 key 排序的音频信息
- 信息先包含 key、sourcePath、size 或 duration、memoryBytes

验收：

- 测试里能稳定比较 debug info 顺序和字段
- 加载、卸载、清空后 debug info 数量正确
- `go test ./...` 通过

## 阶段 6：接入启动链路

目标是让资源预加载成为应用启动的一部分。

要做：

- 在 `GameApp.init()` 创建 renderer 后创建 ResourceManager
- 调用 `LoadResources("assets/data/resource_mapping.json")`
- 演示绘制改为使用 `GetTexture("title-bg")` 或测试纹理 key
- 关闭阶段先清资源，再关 renderer
- README 或后续文档说明资源配置仍要求从仓库根目录运行

验收：

- `go test ./...` 通过
- `go run .` 正常打开窗口
- 缺失 mapping 文件时启动策略明确：开发阶段建议返回错误并停止启动

## 后续扩展

后续类型应该继续挂在 `ResourceManager` facade 下，不让游戏层直接接触子 manager：

- `FontManager`：字体文件、字号、glyph atlas 生命周期
- `UIPresetManager`：按钮和图片预设 json
- `MapResourceManager`：Tiled world/map/tileset 加载与缓存
- `AutoTileLibrary`：自动图块规则
- `AudioPlayer`：播放、停止、循环、音量、淡入淡出和声道管理
- Debug UI：展示资源缓存、内存估算、来源路径和异常资源

迁移原则：

- 新资源类型先进入 `engine/resource`
- 具体使用方只拿到稳定 handle，不拥有释放底层对象的职责
- 只有 `ResourceManager` 负责统一清理缓存
- 资源层不直接做渲染提交，也不直接发起音频播放
