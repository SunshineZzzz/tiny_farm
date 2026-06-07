package scene

import (
	"errors"
)

// 表示等待在更新安全时点执行的场景栈操作
type pendingAction int

const (
	// 没有操作
	pendingNone pendingAction = iota
	// 压栈操作
	pendingPush
	// 出栈操作
	pendingPop
	// 清空场景栈并压入新场景
	pendingReplace
)

// 管理场景栈并在更新安全时点应用切换请求
type SceneManager struct {
	// 按压入顺序保存场景，切片末尾是当前活动场景
	stack []IScene
	// 保存本帧等待执行的场景栈操作
	pending pendingAction
	// 保存 Push 或 Replace 操作需要初始化的新场景
	pendingScene IScene
}

// 创建空场景管理器
func NewSceneManager() *SceneManager {
	return &SceneManager{}
}

// 返回当前活动的栈顶场景
func (m *SceneManager) Current() IScene {
	if m == nil || len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

// 立即压入初始场景
//
// 该入口用于应用初始化，运行期间切换应使用 Request 系列方法
func (m *SceneManager) PushInitial(next IScene) error {
	if m == nil {
		return errors.New("scene manager is nil")
	}
	return m.push(next)
}

// 请求在当前更新结束后压入场景
func (m *SceneManager) RequestPush(next IScene) bool {
	if m == nil || next == nil || m.pending != pendingNone {
		return false
	}
	m.pending = pendingPush
	m.pendingScene = next
	return true
}

// 请求在当前更新结束后弹出栈顶场景
func (m *SceneManager) RequestPop() bool {
	if m == nil || m.pending != pendingNone {
		return false
	}
	m.pending = pendingPop
	m.pendingScene = nil
	return true
}

// 请求在当前更新结束后替换全部场景
func (m *SceneManager) RequestReplace(next IScene) bool {
	if m == nil || next == nil || m.pending != pendingNone {
		return false
	}
	m.pending = pendingReplace
	m.pendingScene = next
	return true
}

// 更新栈顶场景并应用本帧请求的切换
func (m *SceneManager) Update(deltaTime float64) error {
	if m == nil {
		return nil
	}
	if current := m.Current(); current != nil {
		current.Update(deltaTime)
	}
	return m.processPending()
}

// 按从底到顶的顺序渲染场景栈
func (m *SceneManager) Render() error {
	if m == nil {
		return nil
	}
	for _, current := range m.stack {
		if current == nil {
			continue
		}
		if err := current.Render(); err != nil {
			return err
		}
	}
	return nil
}

// 从栈顶到底部清理全部场景
func (m *SceneManager) Close() {
	if m == nil {
		return
	}
	for len(m.stack) > 0 {
		index := len(m.stack) - 1
		m.stack[index].Close()
		m.stack = m.stack[:index]
	}
	m.pending = pendingNone
	m.pendingScene = nil
}

// 取出并清空待处理操作，再执行对应的场景栈修改
//
// 先清空 pending 状态，避免新场景初始化期间残留旧操作
func (m *SceneManager) processPending() error {
	action := m.pending
	next := m.pendingScene
	m.pending = pendingNone
	m.pendingScene = nil

	switch action {
	case pendingPush:
		return m.push(next)
	case pendingPop:
		m.pop()
	case pendingReplace:
		m.Close()
		return m.push(next)
	}
	return nil
}

// 初始化场景并在成功后压入栈顶
//
// 初始化失败时立即清理场景，避免半初始化状态进入场景栈
func (m *SceneManager) push(next IScene) error {
	if next == nil {
		return errors.New("scene is nil")
	}
	if err := next.Init(); err != nil {
		next.Close()
		return err
	}
	m.stack = append(m.stack, next)
	return nil
}

// 清理并移除当前栈顶场景
//
// 空栈调用保持无操作，退出应用等策略由上层决定
func (m *SceneManager) pop() {
	if len(m.stack) == 0 {
		return
	}
	index := len(m.stack) - 1
	m.stack[index].Close()
	m.stack = m.stack[:index]
}
