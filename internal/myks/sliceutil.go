package myks

func appendIfNotExists(slice []string, element string) ([]string, bool) {
	for _, item := range slice {
		if item == element {
			return slice, false
		}
	}

	return append(slice, element), true
}

func filterSlice[T any](slice []T, filterFunc func(v T) bool) []T {
	var result []T
	for _, v := range slice {
		if filterFunc(v) {
			result = append(result, v)
		}
	}
	return result
}

// concatenate merges multiple slices of the same type into a new slice.
func concatenate[T any](slices ...[]T) []T {
	result := []T{}
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}

func unique[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	result := []T{}
	for _, item := range slice {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// mapToSlice converts a map of strings to a slice of "key=value" strings.
func mapToSlice(env map[string]string) []string {
	var envSlice []string
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}
	return envSlice
}
