package data

import "sync"

type ConcurrentPrefixTree[T any] struct {
	children map[rune]*ConcurrentPrefixTree[T]
	payload  *T
	m        *sync.RWMutex
}

func CreatePrefixTree[T any]() *ConcurrentPrefixTree[T] {
	return &ConcurrentPrefixTree[T]{
		m:        &sync.RWMutex{},
		children: map[rune]*ConcurrentPrefixTree[T]{},
	}
}

func (t *ConcurrentPrefixTree[T]) Add(str string, value *T) *ConcurrentPrefixTree[T] {
	t.m.Lock()
	defer t.m.Unlock()

	node := t
	for _, r := range str {
		node = node.getOrCreate(r)
	}

	node.payload = value

	return t
}

func (t *ConcurrentPrefixTree[T]) GetLastPayload(str string) (*T, string) {
	t.m.RLock()
	defer t.m.RUnlock()

	node := t
	var payload *T
	match := ""
	for _, r := range str {
		ok := false
		if node, ok = node.children[r]; !ok {
			break
		}

		match += string(r)

		if node.payload != nil {
			payload = node.payload
		}
	}

	return payload, match
}

func (t *ConcurrentPrefixTree[T]) Remove(str string) *ConcurrentPrefixTree[T] {
	t.m.Lock()
	defer t.m.Unlock()

	remove(t, str)
	return t
}

func remove[T any](node *ConcurrentPrefixTree[T], str string) {
	if str == "" {
		node.payload = nil
		return
	}

	r := rune(str[0])
	next := node.children[r]
	if next == nil {
		return
	}

	remove(next, str[1:])
	if len(next.children) == 0 && next.payload == nil {
		delete(node.children, r)
	}
}

func (t *ConcurrentPrefixTree[T]) getOrCreate(r rune) *ConcurrentPrefixTree[T] {
	if node, ok := t.children[r]; ok {
		return node
	}

	node := CreatePrefixTree[T]()
	t.children[r] = node

	return node
}
