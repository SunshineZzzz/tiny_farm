# dispatcher

`dispatcher` 提供轻量事件分发器，用于在游戏逻辑中解耦事件生产方和监听方

当前实现参考 EnTT dispatcher 的核心使用方式：

- `Trigger` 立即同步派发事件
- `Enqueue` 只把事件放入队列
- `Update` 统一派发队列中已经存在的事件
- `SinkOf[T]` 获取某个事件类型在指定队列上的监听入口
- `SignalConnection.Release` 释放单个监听器连接
- `Sink.Disconnect` 清空某个事件类型和队列上的全部监听器
- `Signal[T]` 和 `SignalSink[T]` 提供局部信号和注册入口，对齐 EnTT `sigh` / `sink`

## 基本用法

```go
type FarmEvent struct {
	Name string
}

bus := dispatcher.NewDispatcher()

connection := dispatcher.SinkOf[FarmEvent](bus).Connect(func(event FarmEvent) {
	fmt.Println(event.Name)
})

bus.Trigger(FarmEvent{Name: "seed"})

connection.Release()
```

## 队列派发

`Enqueue` 不会立即调用监听器，事件会等到后续 `Update` 时派发：

```go
bus.Enqueue(FarmEvent{Name: "grow"})
dispatcher.Update[FarmEvent](bus)
```

`Update` 使用快照语义，只派发调用开始时已经入队的事件

如果监听器回调里再次 `Enqueue`，新事件会留到下一次 `Update`，不会在当前派发中继续扩散

## 命名队列

同一种事件类型可以放在不同队列中：

```go
const UIQueue dispatcher.QueueID = "ui"

dispatcher.SinkOf[FarmEvent](bus, UIQueue).Connect(func(event FarmEvent) {
	fmt.Println(event.Name)
})

bus.EnqueueHint(UIQueue, FarmEvent{Name: "button"})
dispatcher.Update[FarmEvent](bus, UIQueue)
```

默认队列使用 `dispatcher.DefaultQueue`

## 信号槽

`Signal[R]` 表示一组无参槽函数，返回值类型由 `R` 指定，适合表达类似 EnTT `sigh<bool()>` 的局部回调列表

`SignalSink[R]` 绑定到一个具体的 `Signal[R]`，只负责注册和断开槽函数，对齐 EnTT `sink{sigh}` 的关系

```go
var action dispatch.Signal[bool]
sink := action.Sink()

connection := sink.Connect(func() bool {
	return true
})

handled := false
action.Collect(func(result bool) bool {
	handled = result
	return result
})

connection.Release()
_ = handled
```

`Collect` 会按后注册先调用的顺序执行槽函数，并把每个返回值交给 collector

当 collector 返回 `true` 时会停止继续派发，适合输入动作这类“已处理就截断”的场景

如果不需要返回值，可以调用 `Publish`

## 当前边界

- 当前默认只在主线程使用，不做并发保护
- `nil` 事件没有具体类型，会被忽略
- Dispatcher 监听器派发顺序按注册顺序执行
- 派发过程中新增的 Dispatcher 监听器不参与当前这一轮派发
- Signal 槽函数派发顺序按后注册先执行
