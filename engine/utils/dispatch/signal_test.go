package dispatch

import "testing"

// 验证 Signal.Sink 返回的入口会绑定到当前信号
func TestSignalSinkBindsToSignal(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	var received []int

	sink.Connect(func() int {
		return 1
	})

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return false
	})

	if len(received) != 1 || received[0] != 1 {
		t.Fatalf("expected sink callback to be collected from bound signal, got %v", received)
	}
}

// 验证 SignalSinkOf 会绑定到传入的指定信号
func TestSignalSinkOfBindsSpecifiedSignal(t *testing.T) {
	var first Signal[int]
	var second Signal[int]
	firstSink := SignalSinkOf(&first)
	secondSink := SignalSinkOf(&second)
	var firstReceived []int
	var secondReceived []int

	firstSink.Connect(func() int {
		return 1
	})
	secondSink.Connect(func() int {
		return 2
	})

	first.Collect(func(value int) bool {
		firstReceived = append(firstReceived, value)
		return false
	})
	second.Collect(func(value int) bool {
		secondReceived = append(secondReceived, value)
		return false
	})

	if len(firstReceived) != 1 || firstReceived[0] != 1 {
		t.Fatalf("expected first signal to collect only first sink callback, got %v", firstReceived)
	}

	if len(secondReceived) != 1 || secondReceived[0] != 2 {
		t.Fatalf("expected second signal to collect only second sink callback, got %v", secondReceived)
	}
}

// 验证槽函数按后注册先调用的顺序收集返回值
func TestSignalCollectRunsLastConnectedFirst(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	var received []int

	sink.Connect(func() int {
		return 1
	})
	sink.Connect(func() int {
		return 2
	})
	sink.Connect(func() int {
		return 3
	})

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return false
	})

	expected := []int{3, 2, 1}
	if len(received) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, received)
	}

	for index, value := range expected {
		if received[index] != value {
			t.Fatalf("expected %v, got %v", expected, received)
		}
	}
}

// 验证 collector 返回 true 时会停止继续派发
func TestSignalCollectStopsWhenCollectorReturnsTrue(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	var received []int

	sink.Connect(func() int {
		return 1
	})
	sink.Connect(func() int {
		return 2
	})
	sink.Connect(func() int {
		return 3
	})

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return value == 2
	})

	expected := []int{3, 2}
	if len(received) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, received)
	}

	for index, value := range expected {
		if received[index] != value {
			t.Fatalf("expected %v, got %v", expected, received)
		}
	}
}

// 验证释放连接后对应槽函数不再参与派发
func TestSignalConnectionReleaseSkipsCallback(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	var received []int

	sink.Connect(func() int {
		return 1
	})
	connection := sink.Connect(func() int {
		return 2
	})
	connection.Release()

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return false
	})

	if len(received) != 1 || received[0] != 1 {
		t.Fatalf("expected released callback to be skipped, got %v", received)
	}
}

// 验证断开 sink 会清空当前信号上的全部槽函数
func TestSignalSinkDisconnectClearsCallbacks(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	called := false

	sink.Connect(func() int {
		called = true
		return 1
	})
	sink.Disconnect()
	signal.Publish()

	if called {
		t.Fatal("expected disconnected sink callback not to run")
	}
}

// 验证派发过程中释放尚未执行的槽函数会跳过该槽函数
func TestSignalReleaseDuringCollectSkipsReleasedCallback(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	var received []int
	var first SignalConnection[int]

	first = sink.Connect(func() int {
		return 1
	})
	sink.Connect(func() int {
		first.Release()
		return 2
	})
	sink.Connect(func() int {
		return 3
	})

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return false
	})

	expected := []int{3, 2}
	if len(received) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, received)
	}

	for index, value := range expected {
		if received[index] != value {
			t.Fatalf("expected %v, got %v", expected, received)
		}
	}
}

// 验证派发过程中断开 sink 会跳过本轮剩余槽函数
func TestSignalDisconnectDuringCollectSkipsRemainingCallbacks(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	var received []int

	sink.Connect(func() int {
		return 1
	})
	sink.Connect(func() int {
		sink.Disconnect()
		return 2
	})
	sink.Connect(func() int {
		return 3
	})

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return false
	})
	signal.Publish()

	expected := []int{3, 2}
	if len(received) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, received)
	}

	for index, value := range expected {
		if received[index] != value {
			t.Fatalf("expected %v, got %v", expected, received)
		}
	}
}

// 验证空 sink 的连接、释放和断开操作都是安全的
func TestSignalNilSinkIsSafe(t *testing.T) {
	var sink SignalSink[int]

	connection := sink.Connect(func() int {
		return 1
	})
	connection.Release()
	sink.Disconnect()
}

// 验证 nil 信号接收者不会触发 collector
func TestSignalNilReceiverCollectIsSafe(t *testing.T) {
	var signal *Signal[int]
	called := false

	signal.Collect(func(value int) bool {
		called = true
		return false
	})
	signal.Publish()

	if called {
		t.Fatal("expected nil signal not to collect callbacks")
	}
}

// 验证 nil 槽函数不会注册到信号中
func TestSignalNilCallbackIsIgnored(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	var received []int

	sink.Connect(nil)
	sink.Connect(func() int {
		return 1
	})

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return false
	})

	if len(received) != 1 || received[0] != 1 {
		t.Fatalf("expected nil callback to be ignored, got %v", received)
	}
}

// 验证批量释放槽函数后仍保留未释放槽函数的派发行为
func TestSignalReleaseManyCallbacksKeepsRemainingCallbacks(t *testing.T) {
	var signal Signal[int]
	sink := signal.Sink()
	connections := make([]SignalConnection[int], 0, 8)
	var received []int

	for value := 0; value < 8; value++ {
		value := value
		connection := sink.Connect(func() int {
			return value
		})
		connections = append(connections, connection)
	}

	for index, connection := range connections {
		if index%2 == 0 {
			connection.Release()
		}
	}

	signal.Collect(func(value int) bool {
		received = append(received, value)
		return false
	})

	expected := []int{7, 5, 3, 1}
	if len(received) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, received)
	}

	for index, value := range expected {
		if received[index] != value {
			t.Fatalf("expected %v, got %v", expected, received)
		}
	}
}
