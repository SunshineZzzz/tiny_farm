# events

`events` 提供一个轻量事件派发器，用于在游戏逻辑中解耦事件生产方和监听方。

当前实现参考 EnTT dispatcher 的核心使用方式：

- `Trigger` 立即同步派发事件。
- `Enqueue` 只把事件放入队列。
- `Update` 统一派发队列中已经存在的事件。
- `SinkOf[T]` 获取某个事件类型在指定队列上的监听入口。
- `Connection.Release` 释放单个监听器连接。
- `Sink.Disconnect` 清空某个事件类型和队列上的全部监听器。

## 基本用法

```go
type FarmEvent struct {
	Name string
}

dispatcher := events.NewDispatcher()

connection := events.SinkOf[FarmEvent](dispatcher).Connect(func(event FarmEvent) {
	fmt.Println(event.Name)
})

dispatcher.Trigger(FarmEvent{Name: "seed"})

connection.Release()
```

## 队列派发

`Enqueue` 不会立即调用监听器，事件会等到后续 `Update` 时派发：

```go
dispatcher.Enqueue(FarmEvent{Name: "grow"})
events.Update[FarmEvent](dispatcher)
```

`Update` 使用快照语义：只派发调用开始时已经入队的事件。

如果监听器回调里再次 `Enqueue`，新事件会留到下一次 `Update`，不会在当前这次派发中继续扩散。这样可以避免单帧内事件链无限增长。

## 命名队列

同一种事件类型可以放在不同队列中：

```go
const UIQueue events.QueueID = "ui"

events.SinkOf[FarmEvent](dispatcher, UIQueue).Connect(func(event FarmEvent) {
	fmt.Println(event.Name)
})

dispatcher.EnqueueHint(UIQueue, FarmEvent{Name: "button"})
events.Update[FarmEvent](dispatcher, UIQueue)
```

默认队列使用 `events.DefaultQueue`。

## 监听器释放

`Connection.Release` 用于释放单个监听器：

```go
connection := events.SinkOf[FarmEvent](dispatcher).Connect(listener)
connection.Release()
```

当前监听器内部使用 `container/list` 保存，`Release` 可以通过连接持有的节点直接释放，避免按监听器列表线性查找。

派发过程中释放监听器时，会先标记为断开，等当前派发结束后再清理节点，避免修改正在遍历的列表。

## 当前边界

- 当前默认只在主线程使用，不做并发保护。
- `nil` 事件没有具体类型，会被忽略。
- 派发顺序当前按监听器注册顺序执行。
- 派发过程中新增的监听器不会参与当前这一轮派发。
