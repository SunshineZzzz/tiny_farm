package lrucache

import (
	"container/list"
	"sync"
)

// 保存固定容量的最近最少使用缓存
type LRUCache[KT comparable, VT any] struct {
	// 读写互斥锁
	mu sync.RWMutex
	// 容量
	capacity int
	// key<->*list.Element
	cache map[KT]*list.Element
	// 双向链表，元素*entry
	list *list.List
}

// 保存缓存键值和对应链表节点内容
type entry[KT comparable, VT any] struct {
	// 键
	key KT
	// 值
	value *VT
}

// 创建指定容量的最近最少使用缓存
//
// 容量小于 0 时按 0 处理，0 表示不保留缓存条目
func NewLRUCache[KT comparable, VT any](capacity int) *LRUCache[KT, VT] {
	if capacity < 0 {
		capacity = 0
	}
	return &LRUCache[KT, VT]{
		capacity: capacity,
		cache:    make(map[KT]*list.Element),
		list:     list.New(),
	}
}

// 返回当前缓存条目数
func (lru *LRUCache[KT, VT]) Len() int {
	if lru == nil {
		return 0
	}
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return lru.list.Len()
}

// 查询指定键并把命中的条目移动到最近使用位置
func (lru *LRUCache[KT, VT]) Get(key KT) *VT {
	if lru == nil {
		return nil
	}
	lru.mu.Lock()
	defer lru.mu.Unlock()
	if elem, ok := lru.cache[key]; ok {
		lru.list.MoveToFront(elem)
		return elem.Value.(*entry[KT, VT]).value
	}
	return nil
}

// 写入指定键值并在超出容量时淘汰最久未使用条目
func (lru *LRUCache[KT, VT]) Put(key KT, value *VT) {
	if lru == nil {
		return
	}
	lru.mu.Lock()
	defer lru.mu.Unlock()
	if lru.capacity <= 0 {
		return
	}
	if elem, ok := lru.cache[key]; ok {
		lru.list.MoveToFront(elem)
		elem.Value.(*entry[KT, VT]).value = value
	} else {
		if lru.list.Len() >= lru.capacity {
			back := lru.list.Back()
			delete(lru.cache, back.Value.(*entry[KT, VT]).key)
			lru.list.Remove(back)
		}

		newEntry := &entry[KT, VT]{key, value}
		newElem := lru.list.PushFront(newEntry)
		lru.cache[key] = newElem
	}
}

// 删除指定键对应的缓存条目
func (lru *LRUCache[KT, VT]) Delete(key KT) {
	if lru == nil {
		return
	}
	lru.mu.Lock()
	defer lru.mu.Unlock()
	if elem, ok := lru.cache[key]; ok {
		delete(lru.cache, key)
		lru.list.Remove(elem)
	}
}

// 删除满足条件的全部缓存条目
func (lru *LRUCache[KT, VT]) DeleteFunc(match func(KT, *VT) bool) {
	if lru == nil || match == nil {
		return
	}
	lru.mu.Lock()
	defer lru.mu.Unlock()
	for elem := lru.list.Front(); elem != nil; {
		next := elem.Next()
		item := elem.Value.(*entry[KT, VT])
		if match(item.key, item.value) {
			delete(lru.cache, item.key)
			lru.list.Remove(elem)
		}
		elem = next
	}
}

// 清空全部缓存条目
func (lru *LRUCache[KT, VT]) Clear() {
	if lru == nil {
		return
	}
	lru.mu.Lock()
	defer lru.mu.Unlock()
	clear(lru.cache)
	lru.list.Init()
}

// 设置缓存容量并立即淘汰超出容量的旧条目
//
// 容量小于 0 时按 0 处理
func (lru *LRUCache[KT, VT]) SetCapacity(capacity int) {
	if lru == nil {
		return
	}
	if capacity < 0 {
		capacity = 0
	}
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.capacity = capacity
	for lru.list.Len() > lru.capacity {
		back := lru.list.Back()
		item := back.Value.(*entry[KT, VT])
		delete(lru.cache, item.key)
		lru.list.Remove(back)
	}
}
