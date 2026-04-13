package events

import "testing"

type testEventA struct {
	Value int
}

type testEventB struct {
	Name string
}

// 验证 Connect 注册的监听器会被 Trigger 立即同步调用
func TestSinkConnectAndTrigger(t *testing.T) {
	dispatcher := NewDispatcher()
	called := false
	value := 0

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		called = true
		value = event.Value
	})

	dispatcher.Trigger(testEventA{Value: 3})

	if !called {
		t.Fatal("expected trigger to dispatch immediately")
	}

	if value != 3 {
		t.Fatalf("expected value 3, got %d", value)
	}
}

// 验证 Enqueue 只入队，直到类型化 Update 才派发
func TestEnqueueWaitsForUpdate(t *testing.T) {
	dispatcher := NewDispatcher()
	called := false

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		called = true
	})

	dispatcher.Enqueue(testEventA{Value: 1})
	if called {
		t.Fatal("expected enqueue to wait for update")
	}

	Update[testEventA](dispatcher)
	if !called {
		t.Fatal("expected typed update to dispatch queued event")
	}
}

// 验证 Dispatcher Update 会派发当前所有事件队列
func TestDispatcherUpdateDispatchesAllQueuedEvents(t *testing.T) {
	dispatcher := NewDispatcher()
	var received []string

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, "a")
	})
	SinkOf[testEventB](dispatcher).Connect(func(event testEventB) {
		received = append(received, "b")
	})

	dispatcher.Enqueue(testEventA{Value: 1})
	dispatcher.Enqueue(testEventB{Name: "worker"})
	dispatcher.Update()

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
}

// 验证同一事件队列内的事件按入队顺序派发
func TestDispatcherUpdateKeepsOrderInSameQueue(t *testing.T) {
	dispatcher := NewDispatcher()
	var received []int

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, event.Value)
	})

	dispatcher.Enqueue(testEventA{Value: 1})
	dispatcher.Enqueue(testEventA{Value: 2})
	dispatcher.Enqueue(testEventA{Value: 3})
	Update[testEventA](dispatcher)

	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}

	for index, value := range []int{1, 2, 3} {
		if received[index] != value {
			t.Fatalf("expected order %v, got %v", []int{1, 2, 3}, received)
		}
	}
}

// 验证派发回调中新入队的事件留到下一次 Update
func TestEnqueueDuringUpdateWaitsForNextUpdate(t *testing.T) {
	dispatcher := NewDispatcher()
	var received []int

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, event.Value)
		if event.Value == 1 {
			dispatcher.Enqueue(testEventA{Value: 2})
		}
	})

	dispatcher.Enqueue(testEventA{Value: 1})
	Update[testEventA](dispatcher)

	if len(received) != 1 {
		t.Fatalf("expected only the first event in current update, got %v", received)
	}

	Update[testEventA](dispatcher)

	if len(received) != 2 {
		t.Fatalf("expected queued event on second update, got %v", received)
	}
}

// 验证 Release 后对应监听器不再接收事件
func TestConnectionReleaseStopsDispatch(t *testing.T) {
	dispatcher := NewDispatcher()
	called := false

	connection := SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		called = true
	})
	connection.Release()

	dispatcher.Trigger(testEventA{Value: 1})
	if called {
		t.Fatal("expected released connection not to be called")
	}
}

// 验证同一 sink 上的监听器按注册顺序执行
func TestListenersRunInConnectOrder(t *testing.T) {
	dispatcher := NewDispatcher()
	var received []int

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, 1)
	})
	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, 2)
	})
	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, 3)
	})

	dispatcher.Trigger(testEventA{Value: 1})

	if len(received) != 3 {
		t.Fatalf("expected 3 listeners to run, got %v", received)
	}

	for index, value := range []int{1, 2, 3} {
		if received[index] != value {
			t.Fatalf("expected listener order %v, got %v", []int{1, 2, 3}, received)
		}
	}
}

// 验证派发中释放监听器不会破坏本轮遍历
func TestReleaseDuringDispatchSkipsReleasedListener(t *testing.T) {
	dispatcher := NewDispatcher()
	var received []int
	var second Connection

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, 1)
		second.Release()
	})
	second = SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, 2)
	})
	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		received = append(received, 3)
	})

	dispatcher.Trigger(testEventA{Value: 1})

	if len(received) != 2 || received[0] != 1 || received[1] != 3 {
		t.Fatalf("expected released listener to be skipped without stopping dispatch, got %v", received)
	}
}

// 验证派发中 Disconnect 会跳过本轮剩余监听器
func TestDisconnectDuringDispatchSkipsRemainingListeners(t *testing.T) {
	dispatcher := NewDispatcher()
	var received []int
	sink := SinkOf[testEventA](dispatcher)

	sink.Connect(func(event testEventA) {
		received = append(received, 1)
		sink.Disconnect()
	})
	sink.Connect(func(event testEventA) {
		received = append(received, 2)
	})
	sink.Connect(func(event testEventA) {
		received = append(received, 3)
	})

	dispatcher.Trigger(testEventA{Value: 1})
	dispatcher.Trigger(testEventA{Value: 2})

	if len(received) != 1 || received[0] != 1 {
		t.Fatalf("expected disconnect during dispatch to clear current and later listeners, got %v", received)
	}
}

// 验证 Disconnect 会清空当前 sink 上的全部监听器
func TestSinkDisconnectClearsListeners(t *testing.T) {
	dispatcher := NewDispatcher()
	called := false
	sink := SinkOf[testEventA](dispatcher)

	sink.Connect(func(event testEventA) {
		called = true
	})
	sink.Disconnect()

	dispatcher.Trigger(testEventA{Value: 1})
	if called {
		t.Fatal("expected sink listeners to be disconnected")
	}
}

// 验证同一事件类型下不同队列互不影响
func TestNamedQueuesAreIndependent(t *testing.T) {
	dispatcher := NewDispatcher()
	defaultCount := 0
	customCount := 0

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		defaultCount += event.Value
	})
	SinkOf[testEventA](dispatcher, QueueID("custom")).Connect(func(event testEventA) {
		customCount += event.Value
	})

	dispatcher.Enqueue(testEventA{Value: 1})
	dispatcher.EnqueueHint(QueueID("custom"), testEventA{Value: 2})

	Update[testEventA](dispatcher)

	if defaultCount != 1 {
		t.Fatalf("expected default queue count 1, got %d", defaultCount)
	}

	if customCount != 0 {
		t.Fatalf("expected custom queue not dispatched yet, got %d", customCount)
	}

	Update[testEventA](dispatcher, QueueID("custom"))

	if customCount != 2 {
		t.Fatalf("expected custom queue count 2, got %d", customCount)
	}
}

// 验证 ClearType 只丢弃指定类型和队列中的待派发事件
func TestClearTypeDropsQueuedEvents(t *testing.T) {
	dispatcher := NewDispatcher()
	called := false

	SinkOf[testEventA](dispatcher).Connect(func(event testEventA) {
		called = true
	})

	dispatcher.Enqueue(testEventA{Value: 1})
	ClearType[testEventA](dispatcher)
	Update[testEventA](dispatcher)

	if called {
		t.Fatal("expected cleared queued event not to be dispatched")
	}
}
