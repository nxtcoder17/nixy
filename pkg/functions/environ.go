package functions

import (
	"fmt"
)

func ToEnviron(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, fmt.Sprintf("%s=%q", k, v))
	}
	return result
}
