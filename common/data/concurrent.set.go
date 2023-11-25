package data

import "sync"

type ConcurrentSet[T comparable] struct {
	lock *sync.RWMutex
	set  map[T]bool
}

func CreateConcurrentSet[T comparable]() ConcurrentSet[T] {
	return ConcurrentSet[T]{
		lock: &sync.RWMutex{},
		set:  make(map[T]bool),
	}
}

func (s ConcurrentSet[T]) Add(key T) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.set[key] = true
}

func (s ConcurrentSet[T]) AddUniq(key T) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.set[key] {
		return false
	}

	s.set[key] = true

	return true
}

func (s ConcurrentSet[T]) Remove(key T) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.set, key)
}

func (s ConcurrentSet[T]) Has(key T) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.set[key]
}
