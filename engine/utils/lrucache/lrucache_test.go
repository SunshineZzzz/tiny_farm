package lrucache

import (
	"testing"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache[uint32, uint32](2)

	// 添加键值对
	val1 := uint32(1)
	cache.Put(1, &val1)
	val2 := uint32(2)
	cache.Put(2, &val2)

	// 获取键1的值，期望为1
	if val := cache.Get(1); val != &val1 {
		t.Errorf("Expected value %v, but got %v", &val1, val)
	}

	// 添加一个新的键值对，容量已满，会移除键2
	val3 := uint32(3)
	cache.Put(3, &val3)

	// 获取键2的值，期望为-1（已被移除）
	if val := cache.Get(2); val != nil {
		t.Errorf("Expected value nil, but got %d", val)
	}

	// 添加一个新的键值对，会更新键1的值
	val1New := uint32(32)
	cache.Put(1, &val1New)

	// 获取键1的值，期望为10
	if val := cache.Get(1); val != &val1New {
		t.Errorf("Expected value %v, but got %d", &val1New, val)
	}

	// 获取键3的值，期望为3
	if val := cache.Get(3); val != &val3 {
		t.Errorf("Expected value %v, but got %d", &val3, val)
	}

	// 删除键1
	cache.Delete(1)

	// 获取键1的值，期望为-1（已被删除）
	if val := cache.Get(1); val != nil {
		t.Errorf("Expected value nil, but got %d", val)
	}

	// 获取缓存大小，期望为2
	if size := cache.Len(); size != 1 {
		t.Errorf("Expected size 1, but got %d", size)
	}
}

// 验证结构体可以作为缓存键
func TestLRUCacheSupportsComparableStructKey(t *testing.T) {
	type cacheKey struct {
		name string
		size int
	}
	cache := NewLRUCache[cacheKey, int](1)
	value := 1
	key := cacheKey{name: "font", size: 16}

	cache.Put(key, &value)

	if cached := cache.Get(key); cached != &value {
		t.Fatalf("expected cached value %v, got %v", &value, cached)
	}
}

// 验证调整容量会立即淘汰最久未使用条目
func TestLRUCacheSetCapacityTrimsOldEntries(t *testing.T) {
	cache := NewLRUCache[int, int](3)
	first := 1
	second := 2
	third := 3
	cache.Put(1, &first)
	cache.Put(2, &second)
	cache.Put(3, &third)
	cache.Get(1)

	cache.SetCapacity(2)

	if cache.Get(2) != nil {
		t.Fatal("expected least recently used entry to be removed")
	}
	if cache.Len() != 2 {
		t.Fatalf("expected size 2, got %d", cache.Len())
	}
}

// 验证按条件删除和清空缓存
func TestLRUCacheDeleteFuncAndClear(t *testing.T) {
	cache := NewLRUCache[int, int](3)
	first := 1
	second := 2
	third := 3
	cache.Put(1, &first)
	cache.Put(2, &second)
	cache.Put(3, &third)

	cache.DeleteFunc(func(key int, _ *int) bool {
		return key%2 == 1
	})

	if cache.Get(1) != nil || cache.Get(3) != nil || cache.Get(2) != &second {
		t.Fatal("expected only the even key to remain")
	}
	cache.Clear()
	if cache.Len() != 0 {
		t.Fatalf("expected empty cache, got size %d", cache.Len())
	}
}

// 验证零容量缓存不会保留条目
func TestLRUCacheZeroCapacityDoesNotStoreEntries(t *testing.T) {
	cache := NewLRUCache[int, int](0)
	value := 1

	cache.Put(1, &value)

	if cache.Len() != 0 || cache.Get(1) != nil {
		t.Fatal("expected zero-capacity cache to remain empty")
	}
}
