package functions

import (
	"maps"
)

func MapMerge[K comparable, V any](m ...map[K]V) map[K]V {
	result := make(map[K]V)
	for i := range m {
		maps.Copy(result, m[i])
	}

	return result
}
