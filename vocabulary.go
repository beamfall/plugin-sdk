package sdk

func copySlice[T any](in []T) []T {
	return append([]T(nil), in...)
}
