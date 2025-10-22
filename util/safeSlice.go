package util

import (
	"sync"
)

type TsSlice[T any] struct {
	data []T
	mx   *sync.RWMutex
}

func New[T any](size int) *TsSlice[T] {
	return &TsSlice[T]{
		data: make([]T, 0, size),
		mx:   &sync.RWMutex{},
	}
}

func (s *TsSlice[T]) Append(val T) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.data = append(s.data, val)
}

func (s *TsSlice[T]) Subslice(from, to int) *TsSlice[T] {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return &TsSlice[T]{
		data: s.data[from:to],
		mx:   s.mx,
	}
}

func (s *TsSlice[T]) Get(idx int) T {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return s.data[idx]
}

func (s *TsSlice[T]) Delete(idx int) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if idx == len(s.data)-1 {
		s.data = s.data[:len(s.data)-1]
	} else {
		s.data = append(s.data[0:idx], s.data[idx+1:]...)
	}
}

func (s *TsSlice[T]) Length() int {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return len(s.data)
}
