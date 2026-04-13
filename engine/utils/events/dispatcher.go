package events

import (
	"container/list"
	"reflect"
)

// 用于区分同一事件类型下的不同队列
type QueueID string

// 表示默认队列
const DefaultQueue QueueID = ""

// 是一个通用事件派发器
//
// 当前实现对齐 EnTT dispatcher 的核心语义
// - Trigger 立即同步触发
// - Enqueue 只入队
// - Update 统一派发队列中的事件
//
// 由于 Go 不支持泛型方法，像 EnTT 那样的 dispatcher sink 泛型接口
// 这里改为通过 SinkOf[T](dispatcher) 获取一个类型化 sink
// 当前默认只在主线程使用，不做并发保护
type Dispatcher struct {
	// 按事件类型和队列名保存订阅回调链表
	handlers map[eventQueueKey]*handlerList
	// 按事件类型和队列名保存等待派发的事件
	queues map[eventQueueKey][]any
}

// 用于管理某种事件类型在指定队列上的监听器
type Sink[T any] struct {
	// 绑定的事件派发器
	dispatcher *Dispatcher
	// 当前 sink 操作的队列
	queueID QueueID
}

// 表示一条可释放的监听连接
type Connection struct {
	// 连接所属的事件派发器
	dispatcher *Dispatcher
	// 直接指向监听器节点，用于 O(1) 释放
	element *list.Element
}

// 用于定位某个事件类型在某个队列上的状态
type eventQueueKey struct {
	// 事件的具体 Go 类型
	eventType reflect.Type
	// 事件所在的队列
	queueID QueueID
}

// 是一条监听器记录
//
// 当前通过 container list 管理节点，使 Connection Release 可以直接按节点释放
type handlerEntry struct {
	// 该监听器所属的事件类型和队列
	key eventQueueKey
	// 类型擦除后的监听回调
	fn func(any)
	// 该监听器当前所属的监听器列表
	list *handlerList
	// 表示监听器是否仍然有效，派发中释放时先置为 false
	connected bool
}

// 保存同一事件类型和队列下的监听器
//
// 派发过程中不直接删除节点，而是标记 dirty，等当前派发结束后统一清理
type handlerList struct {
	// 按注册顺序保存监听器
	items list.List
	// 记录当前嵌套派发层数
	dispatches int
	// 表示派发过程中出现过待清理节点
	dirty bool
}

// 创建一个空的事件派发器
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[eventQueueKey]*handlerList),
		queues:   make(map[eventQueueKey][]any),
	}
}

// 返回某种事件类型在指定队列上的 sink
func SinkOf[T any](dispatcher *Dispatcher, queueID ...QueueID) Sink[T] {
	return Sink[T]{
		dispatcher: dispatcher,
		queueID:    resolveQueueID(queueID),
	}
}

// 注册监听器，并返回一条可释放的连接
func (s Sink[T]) Connect(listener func(T)) Connection {
	if s.dispatcher == nil || listener == nil {
		return Connection{}
	}

	key := makeEventQueueKey[T](s.queueID)
	handler := &handlerEntry{
		key: key,
		fn: func(event any) {
			listener(event.(T))
		},
		connected: true,
	}
	element := s.dispatcher.appendHandler(key, handler)

	return Connection{
		dispatcher: s.dispatcher,
		element:    element,
	}
}

// 清空当前 sink 上的全部监听器
func (s Sink[T]) Disconnect() {
	if s.dispatcher == nil {
		return
	}

	s.dispatcher.disconnect(makeEventQueueKey[T](s.queueID))
}

// 断开一条连接
func (c Connection) Release() {
	if c.dispatcher == nil || c.element == nil {
		return
	}

	c.dispatcher.release(c.element)
}

// 立即触发默认队列上的事件
func (d *Dispatcher) Trigger(event any) {
	d.TriggerHint(DefaultQueue, event)
}

// 立即触发指定队列上的事件
func (d *Dispatcher) TriggerHint(queueID QueueID, event any) {
	if d == nil {
		return
	}

	key, ok := makeQueueKeyFromEvent(queueID, event)
	if !ok {
		return
	}

	d.dispatch(key, event)
}

// 把事件放入默认队列，等待后续 Update 派发
func (d *Dispatcher) Enqueue(event any) {
	d.EnqueueHint(DefaultQueue, event)
}

// 把事件放入指定队列，等待后续 Update 派发
func (d *Dispatcher) EnqueueHint(queueID QueueID, event any) {
	if d == nil {
		return
	}

	key, ok := makeQueueKeyFromEvent(queueID, event)
	if !ok {
		return
	}

	d.queues[key] = append(d.queues[key], event)
}

// 派发当前所有队列里已经存在的事件
//
// 如果回调里再次 Enqueue，新事件会留到下一次 Update
func (d *Dispatcher) Update() {
	if d == nil || len(d.queues) == 0 {
		return
	}

	pending := d.queues
	d.queues = make(map[eventQueueKey][]any)

	for key, events := range pending {
		for _, event := range events {
			d.dispatch(key, event)
		}
	}
}

// 清空当前所有队列中的待派发事件，但不影响监听器
func (d *Dispatcher) Clear() {
	if d == nil {
		return
	}

	d.queues = make(map[eventQueueKey][]any)
}

// 派发某种事件类型在默认队列上的事件
func Update[T any](dispatcher *Dispatcher, queueID ...QueueID) {
	if dispatcher == nil {
		return
	}

	key := makeEventQueueKey[T](resolveQueueID(queueID))
	events := dispatcher.queues[key]
	if len(events) == 0 {
		return
	}

	delete(dispatcher.queues, key)

	for _, event := range events {
		dispatcher.dispatch(key, event)
	}
}

// 清空某种事件类型在指定队列上的待派发事件
func ClearType[T any](dispatcher *Dispatcher, queueID ...QueueID) {
	if dispatcher == nil {
		return
	}

	delete(dispatcher.queues, makeEventQueueKey[T](resolveQueueID(queueID)))
}

// 派发指定队列中的单个事件
//
// 当前会固定本轮派发开始时的尾节点，派发过程中新增的监听器不会参与本轮
func (d *Dispatcher) dispatch(key eventQueueKey, event any) {
	handlers := d.handlers[key]
	if handlers == nil {
		return
	}

	handlers.dispatches++
	// 锁定循环开始时的最后一个元素，保证本轮派发只针对派发开始前就已经存在的监听器
	tail := handlers.items.Back()
	for element := handlers.items.Front(); element != nil; element = element.Next() {
		handler := element.Value.(*handlerEntry)
		if handler.connected {
			handler.fn(event)
		}
		if element == tail {
			break
		}
	}
	handlers.dispatches--

	if handlers.dispatches == 0 && handlers.dirty {
		d.compactHandlers(key, handlers)
	}
}

// 把监听器追加到指定事件队列的末尾
//
// 返回的 list Element 会被 Connection 保存，用于后续 O(1) 释放
func (d *Dispatcher) appendHandler(key eventQueueKey, handler *handlerEntry) *list.Element {
	handlers := d.handlers[key]
	if handlers == nil {
		handlers = &handlerList{}
		d.handlers[key] = handlers
	}

	handler.list = handlers
	return handlers.items.PushBack(handler)
}

// 释放指定监听器节点
//
// 如果当前正在派发，则只标记断开，避免修改正在遍历的链表
func (d *Dispatcher) release(element *list.Element) {
	handler := element.Value.(*handlerEntry)
	if !handler.connected {
		return
	}

	handlers := handler.list
	if handlers == nil {
		handler.connected = false
		return
	}

	handler.connected = false
	if handlers.dispatches > 0 {
		handlers.dirty = true
		return
	}

	d.unlinkHandler(handler.key, handlers, element, handler)
}

// 断开指定事件队列上的全部监听器
//
// 如果当前正在派发，则保留列表到派发结束后再清理
func (d *Dispatcher) disconnect(key eventQueueKey) {
	handlers := d.handlers[key]
	if handlers == nil {
		return
	}

	for element := handlers.items.Front(); element != nil; element = element.Next() {
		handler := element.Value.(*handlerEntry)
		handler.connected = false
	}

	if handlers.dispatches > 0 {
		handlers.dirty = true
		delete(d.handlers, key)
		return
	}

	// 遍历删除可以切断外部 Connection 对整条监听器链表的引用
	// 删除后未释放的 Connection 只会保留一个很小的 handlerEntry
	for element := handlers.items.Front(); element != nil; {
		next := element.Next()
		handler := element.Value.(*handlerEntry)
		handler.list = nil
		handlers.items.Remove(element)
		element = next
	}

	delete(d.handlers, key)
}

// 清理派发过程中已经标记断开的监听器
//
// 当前只在最外层派发结束后调用，避免嵌套派发时破坏遍历状态
func (d *Dispatcher) compactHandlers(key eventQueueKey, handlers *handlerList) {
	for element := handlers.items.Front(); element != nil; {
		next := element.Next()
		handler := element.Value.(*handlerEntry)
		if !handler.connected {
			d.unlinkHandler(key, handlers, element, handler)
		}
		element = next
	}

	handlers.dirty = false
	if handlers.items.Len() == 0 && d.handlers[key] == handlers {
		delete(d.handlers, key)
	}
}

// 从列表中移除单个监听器节点
//
// 当列表被清空时，同步删除 dispatcher 上对应的 map 入口
func (d *Dispatcher) unlinkHandler(key eventQueueKey, handlers *handlerList, element *list.Element, handler *handlerEntry) {
	handler.list = nil
	handlers.items.Remove(element)

	if handlers.items.Len() == 0 && d.handlers[key] == handlers {
		delete(d.handlers, key)
	}
}

// 根据泛型事件类型和队列生成查找 key
func makeEventQueueKey[T any](queueID QueueID) eventQueueKey {
	return eventQueueKey{
		eventType: reflect.TypeFor[T](),
		queueID:   queueID,
	}
}

// 根据运行时事件值和队列生成查找 key
//
// 空事件没有具体类型，当前直接返回 false 并跳过处理
func makeQueueKeyFromEvent(queueID QueueID, event any) (eventQueueKey, bool) {
	eventType := reflect.TypeOf(event)
	if eventType == nil {
		return eventQueueKey{}, false
	}

	return eventQueueKey{
		eventType: eventType,
		queueID:   queueID,
	}, true
}

// 解析可选队列参数
//
// 当前只读取第一个参数，未传入时使用默认队列
func resolveQueueID(queueID []QueueID) QueueID {
	if len(queueID) == 0 {
		return DefaultQueue
	}

	return queueID[0]
}
