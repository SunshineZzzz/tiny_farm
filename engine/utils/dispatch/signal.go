package dispatch

// 保存一组无参槽函数，并在派发时收集返回值
//
// 当前用于对齐 EnTT sigh<bool()> 这类局部信号，默认只在主线程使用
type Signal[R any] struct {
	// 指向最早注册的槽函数
	head *signalSlot[R]
	// 指向最后注册的槽函数，用于按后注册先调用派发
	tail *signalSlot[R]
	// 记录派发过程中失效但尚未摘链的槽函数
	pending map[*signalSlot[R]]struct{}
	// 记录当前嵌套派发层数
	publishes int
}

// 绑定到某个信号的槽函数注册入口
//
// 对外注册槽函数时使用 sink，避免调用方直接操作信号内部连接逻辑
type SignalSink[R any] struct {
	signal *Signal[R]
}

// 表示一条可释放的槽函数连接
type SignalConnection[R any] struct {
	signal *Signal[R]
	slot   *signalSlot[R]
}

// 保存单个槽函数和连接状态
type signalSlot[R any] struct {
	callback  func() R
	connected bool
	previous  *signalSlot[R]
	next      *signalSlot[R]
}

// 返回绑定到当前信号的注册入口
func (s *Signal[R]) Sink() SignalSink[R] {
	return SignalSink[R]{
		signal: s,
	}
}

// 绑定指定信号并返回注册入口
func SignalSinkOf[R any](signal *Signal[R]) SignalSink[R] {
	return SignalSink[R]{
		signal: signal,
	}
}

// 注册一个槽函数
func (s SignalSink[R]) Connect(callback func() R) SignalConnection[R] {
	if s.signal == nil || callback == nil {
		return SignalConnection[R]{}
	}

	return s.signal.connect(callback)
}

// 断开当前注册入口上的全部槽函数
func (s SignalSink[R]) Disconnect() {
	if s.signal == nil {
		return
	}

	for slot := s.signal.head; slot != nil; {
		next := slot.next
		s.signal.disconnect(slot)
		slot = next
	}

	if s.signal.publishes > 0 {
		return
	}
}

// 派发信号并忽略槽函数返回值
func (s *Signal[R]) Publish() {
	s.Collect(nil)
}

// 派发信号并把每个槽函数返回值交给 collector
//
// collector 返回 true 时停止继续派发，派发顺序为后注册先调用
func (s *Signal[R]) Collect(collector func(R) bool) {
	if s == nil || s.tail == nil {
		return
	}

	s.publishes++
	for slot := s.tail; slot != nil; slot = slot.previous {
		if !slot.connected || slot.callback == nil {
			continue
		}
		if collector != nil && collector(slot.callback()) {
			break
		}
		if collector == nil {
			slot.callback()
		}
	}
	s.publishes--

	if s.publishes == 0 && len(s.pending) > 0 {
		s.compact()
	}
}

// 释放当前槽函数连接
func (c SignalConnection[R]) Release() {
	if c.signal == nil || c.slot == nil {
		return
	}

	c.signal.release(c.slot)
}

func (s *Signal[R]) connect(callback func() R) SignalConnection[R] {
	slot := &signalSlot[R]{
		callback:  callback,
		connected: true,
		previous:  s.tail,
	}
	if s.tail != nil {
		s.tail.next = slot
	} else {
		s.head = slot
	}
	s.tail = slot

	return SignalConnection[R]{
		signal: s,
		slot:   slot,
	}
}

func (s *Signal[R]) release(slot *signalSlot[R]) {
	if !slot.connected {
		return
	}

	s.disconnect(slot)
}

func (s *Signal[R]) compact() {
	for slot := range s.pending {
		if !slot.connected {
			s.unlink(slot)
		}
	}

	s.pending = nil
}

func (s *Signal[R]) disconnect(slot *signalSlot[R]) {
	if !slot.connected {
		return
	}

	slot.connected = false
	if s.publishes == 0 {
		s.unlink(slot)
		return
	}

	if s.pending == nil {
		s.pending = make(map[*signalSlot[R]]struct{})
	}
	s.pending[slot] = struct{}{}
}

func (s *Signal[R]) unlink(slot *signalSlot[R]) {
	if slot.previous != nil {
		slot.previous.next = slot.next
	} else {
		s.head = slot.next
	}

	if slot.next != nil {
		slot.next.previous = slot.previous
	} else {
		s.tail = slot.previous
	}

	slot.previous = nil
	slot.next = nil
}
