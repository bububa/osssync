package pkg

import "sync"

type Map[K comparable, V any] struct {
	mp *sync.Map
}

func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{mp: &sync.Map{}}
}

/*
以下的函数是按照 sync.Map 的所有函数 100% 或者说 1:1 封装的，只是多个泛型，让参数和返回值都有类型。
*/

func (m *Map[K, V]) CompareAndDelete(key K, value V) bool {
	return m.mp.CompareAndDelete(key, value)
}

func (m *Map[K, V]) CompareAndSwap(key K, old, new V) bool {
	return m.mp.CompareAndSwap(key, old, new)
}

func (m *Map[K, V]) Delete(key K) {
	m.mp.Delete(key)
}

func (m *Map[K, V]) Clear() {
	m.mp.Clear()
}

func (m *Map[K, V]) Load(key K) (res V, ok bool) {
	value, ok := m.mp.Load(key)
	if !ok {
		return res, false
	}
	return value.(V), true
}

func (m *Map[K, V]) LoadAndDelete(key K) (res V, ok bool) {
	value, ok := m.mp.LoadAndDelete(key)
	if !ok {
		return res, false
	}
	return value.(V), true
}

func (m *Map[K, V]) LoadOrStore(key K, new V) (V, bool) {
	value, ok := m.mp.LoadOrStore(key, new)
	if !ok {
		return value.(V), false
	}
	return value.(V), true
}

func (m *Map[K, V]) Range(run func(key K, value V) bool) {
	m.mp.Range(func(k, v any) bool {
		return run(k.(K), v.(V))
	})
}

func (m *Map[K, V]) Store(key K, value V) {
	m.mp.Store(key, value)
}

func (m *Map[K, V]) Swap(key K, value V) (pre V, ok bool) {
	previous, ok := m.mp.Swap(key, value)
	if !ok {
		return pre, false
	}
	return previous.(V), true
}

func (m *Map[K, V]) Count() (size int) {
	m.Range(func(k K, v V) bool {
		size++
		return true
	})
	return size
}

func (m *Map[K, V]) Keys() (keys []K) {
	m.Range(func(k K, v V) bool {
		keys = append(keys, k)
		return true
	})
	return keys
}

func (m *Map[K, V]) Values() (values []V) {
	m.Range(func(k K, v V) bool {
		values = append(values, v)
		return true
	})
	return values
}

func (m *Map[K, V]) GetMap() map[K]V {
	res := map[K]V{}
	m.Range(func(k K, v V) bool {
		res[k] = v
		return true
	})
	return res
}

func (m *Map[K, V]) SetMap(mp map[K]V) {
	for k, v := range mp {
		m.Store(k, v)
	}
}

func (m *Map[K, V]) SetSyncMap(mp *Map[K, V]) {
	mp.Range(func(k K, v V) bool {
		m.Store(k, v)
		return true
	})
}

func (m *Map[K, V]) SetSyncMaps(mps ...*Map[K, V]) {
	for _, mp := range mps {
		m.SetSyncMap(mp)
	}
}
