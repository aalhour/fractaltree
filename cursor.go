package fractaltree

// Cursor provides manual bidirectional iteration over a BETree.
// A Cursor must be closed when no longer needed.
type Cursor[K any, V any] struct {
	valid bool
}

// Next advances the cursor to the next key-value pair.
// Returns false if the cursor has moved past the last entry.
func (c *Cursor[K, V]) Next() bool {
	// TODO: implement
	return false
}

// Prev moves the cursor to the previous key-value pair.
// Returns false if the cursor has moved before the first entry.
func (c *Cursor[K, V]) Prev() bool {
	// TODO: implement
	return false
}

// Seek positions the cursor at the first key >= the given key.
// Returns false if no such key exists.
func (c *Cursor[K, V]) Seek(_ K) bool {
	// TODO: implement
	return false
}

// Key returns the key at the cursor's current position.
// The caller must check Valid before calling Key.
func (c *Cursor[K, V]) Key() K {
	var zero K
	return zero
}

// Value returns the value at the cursor's current position.
// The caller must check Valid before calling Value.
func (c *Cursor[K, V]) Value() V {
	var zero V
	return zero
}

// Valid reports whether the cursor is positioned at a valid entry.
func (c *Cursor[K, V]) Valid() bool {
	return c.valid
}

// Close releases resources held by the cursor.
// It is safe to call Close multiple times.
func (c *Cursor[K, V]) Close() {
	c.valid = false
}
