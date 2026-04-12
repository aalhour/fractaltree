package fractaltree

// Increment returns an UpsertFn that adds delta to the existing value.
// If the key does not exist, delta is used as the initial value.
func Increment[V interface{ ~int | ~int64 | ~float64 }](delta V) UpsertFn[V] {
	return func(existing *V, exists bool) V {
		if !exists {
			return delta
		}
		return *existing + delta
	}
}

// CompareAndSwap returns an UpsertFn that replaces the value with newVal
// only if the current value equals expected. If the key does not exist
// or the current value differs, the existing value is retained unchanged.
func CompareAndSwap[V comparable](expected, newVal V) UpsertFn[V] {
	return func(existing *V, exists bool) V {
		if exists && *existing == expected {
			return newVal
		}
		if exists {
			return *existing
		}
		var zero V
		return zero
	}
}
