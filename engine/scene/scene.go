package scene

// 场景接口
type IScene interface {
	// 返回用于日志和调试显示的场景名称
	Name() string
	// 初始化场景持有的资源和运行时状态
	Init() error
	// 使用当前帧间隔更新场景逻辑
	Update(deltaTime float64)
	// 提交当前场景的绘制命令
	Render() error
	// 清理场景持有的资源和运行时状态
	Close()
}
