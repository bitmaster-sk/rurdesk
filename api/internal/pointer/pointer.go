package pointer

func Equal[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func Value[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}
