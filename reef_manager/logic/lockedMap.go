package logic

import "sync"

type LockedValue[T any] struct {
	Lock *sync.RWMutex
	Data *T
}

func NewLockedValue[T any](v T) LockedValue[T] {
	return LockedValue[T]{
		Lock: &sync.RWMutex{},
		Data: &v,
	}
}

type LockedMap[K comparable, V any] struct {
	Map  map[K]V
	Lock sync.RWMutex
}

func newLockedMap[K comparable, V any]() LockedMap[K, V] {
	return LockedMap[K, V]{
		Map:  make(map[K]V),
		Lock: sync.RWMutex{},
	}
}

func (m *LockedMap[K, V]) Clear() {
	m.Lock.Lock()
	clear(m.Map)
	m.Lock.Unlock()
}

func (m *LockedMap[K, V]) Insert(k K, v V) {
	m.Lock.Lock()
	m.Map[k] = v
	m.Lock.Unlock()
}

func (m *LockedMap[K, V]) Get(k K) (V, bool) {
	m.Lock.RLock()
	v, found := m.Map[k]
	m.Lock.RUnlock()
	return v, found
}

func (m *LockedMap[K, V]) Delete(k K) (v V, found bool) {
	m.Lock.Lock()
	value, found := m.Map[k]
	delete(m.Map, k)
	m.Lock.Unlock()

	return value, found
}

func (m *LockedMap[K, V]) DeleteNoLock(k K) (v V, found bool) {
	value, found := m.Map[k]
	delete(m.Map, k)

	return value, found
}
