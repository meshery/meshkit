package store

import "sync"

type GenerticThreadSafeStore[K any] struct {
	data map[string]K
	mx   sync.Mutex
}

func NewGenericThreadSafeStore[K any]() *GenerticThreadSafeStore[K] {
	return &GenerticThreadSafeStore[K]{
		data: make(map[string]K),
	}
}

func (s *GenerticThreadSafeStore[K]) Set(key string, val K) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.data[key] = val
}

func (s *GenerticThreadSafeStore[K]) Get(key string) (K, bool) {
	s.mx.Lock()
	defer s.mx.Unlock()

	value, ok := s.data[key]
	return value, ok
}

func (s *GenerticThreadSafeStore[K]) Delete(key string) {
	s.mx.Lock()
	defer s.mx.Unlock()
	delete(s.data, key)
}


func (s *GenerticThreadSafeStore[K]) GetAllPairs() map[string]K {
	values := make(map[string]K, 0)

	s.mx.Lock()
	defer s.mx.Unlock()

	for key, val := range s.data {
		values[key] = val
	}
	return values
}
