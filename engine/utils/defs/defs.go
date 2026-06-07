package defs

// 标识资源映射中的稳定语义 key
//
// 当前先用字符串保持调用和测试简单，后续如需数字 ID 可在内部补 hash
type ResourceKey string

// 动作名称在 Go 侧的标识
//
// 当前直接使用配置中的动作名称，避免为精简客户端额外引入哈希层
type ActionID string
