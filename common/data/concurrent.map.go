package data

import "sync"

type ConcurrentMap[K comparable, V any] struct {
	m    map[K]V
	lock *sync.RWMutex
}

func CreateConcurrentMap[K comparable, V any]() ConcurrentMap[K, V] {
	return ConcurrentMap[K, V]{
		m:    map[K]V{},
		lock: &sync.RWMutex{},
	}
}

func (m ConcurrentMap[K, V]) Set(k K, v V) ConcurrentMap[K, V] {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.m[k] = v

	return m
}

func (m ConcurrentMap[K, V]) Delete(k K) ConcurrentMap[K, V] {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.m, k)

	return m
}

func (m ConcurrentMap[K, V]) Get(k K, d V) V {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if v, ok := m.m[k]; ok {
		return v
	}

	return d
}

func (m ConcurrentMap[K, V]) Update(k K, update func(v V) V) ConcurrentMap[K, V] {
	m.lock.Lock()
	defer m.lock.Unlock()

	if v, ok := m.m[k]; ok {
		m.m[k] = update(v)
	}

	return m
}

func (m ConcurrentMap[K, V]) ForEach(traverse func(k K, v V)) ConcurrentMap[K, V] {
	m.lock.RLock()
	defer m.lock.RUnlock()

	for k, v := range m.m {
		traverse(k, v)
	}

	return m
}

func (m ConcurrentMap[K, V]) Values() []V {
	m.lock.RLock()
	defer m.lock.RUnlock()

	res := make([]V, 0, len(m.m))

	for _, v := range m.m {
		res = append(res, v)
	}

	return res
}
