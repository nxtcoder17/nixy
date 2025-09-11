package set

import (
	"cmp"
	"slices"
)

type Set[T cmp.Ordered] struct {
	items map[T]struct{}
}

func (s *Set[T]) Add(item T) {
	if s.items == nil {
		s.items = make(map[T]struct{}, 1)
	}

	if _, ok := s.items[item]; !ok {
		s.items[item] = struct{}{}
	}
}

func (s *Set[T]) ToSortedList() []T {
	result := make([]T, 0, len(s.items))
	for item := range s.items {
		result = append(result, item)
	}
	slices.SortFunc(result, func(a, b T) int {
		return cmp.Compare(a, b)
	})

	return result
}
