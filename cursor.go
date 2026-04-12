package fractaltree

// Cursor provides manual bidirectional iteration over a BETree.
// It operates on a snapshot taken at creation time; subsequent tree
// mutations are not visible. A Cursor must be closed when no longer needed.
type Cursor[K any, V any] struct {
	pairs []kvPair[K, V]
	pos   int
	valid bool
	cmp   func(K, K) int
}

// Next advances the cursor to the next key-value pair.
// Returns true if the cursor is positioned at a valid entry after advancing.
func (c *Cursor[K, V]) Next() bool {
	if !c.valid {
		// First call to Next positions at the beginning.
		if len(c.pairs) > 0 {
			c.pos = 0
			c.valid = true
			return true
		}
		return false
	}
	c.pos++
	if c.pos >= len(c.pairs) {
		c.valid = false
		return false
	}
	return true
}

// Prev moves the cursor to the previous key-value pair.
// Returns true if the cursor is positioned at a valid entry after moving.
func (c *Cursor[K, V]) Prev() bool {
	if !c.valid {
		// First call to Prev positions at the end.
		if len(c.pairs) > 0 {
			c.pos = len(c.pairs) - 1
			c.valid = true
			return true
		}
		return false
	}
	c.pos--
	if c.pos < 0 {
		c.valid = false
		return false
	}
	return true
}

// Seek positions the cursor at the first key >= the given key.
// Returns true if such a key exists.
func (c *Cursor[K, V]) Seek(key K) bool {
	lo, hi := 0, len(c.pairs)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if c.cmp(c.pairs[mid].key, key) < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= len(c.pairs) {
		c.valid = false
		return false
	}
	c.pos = lo
	c.valid = true
	return true
}

// Key returns the key at the cursor's current position.
// The caller must check Valid before calling Key.
func (c *Cursor[K, V]) Key() K {
	return c.pairs[c.pos].key
}

// Value returns the value at the cursor's current position.
// The caller must check Valid before calling Value.
func (c *Cursor[K, V]) Value() V {
	return c.pairs[c.pos].value
}

// Valid reports whether the cursor is positioned at a valid entry.
func (c *Cursor[K, V]) Valid() bool {
	return c.valid
}

// Close releases resources held by the cursor.
// It is safe to call Close multiple times.
func (c *Cursor[K, V]) Close() {
	c.valid = false
	c.pairs = nil
}
