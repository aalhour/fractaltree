package fractaltree

// MsgKind identifies the type of a buffered message in a B-epsilon-tree node.
type MsgKind uint8

const (
	// MsgPut inserts or overwrites a key-value pair.
	MsgPut MsgKind = iota

	// MsgDelete removes a single key.
	MsgDelete

	// MsgDeleteRange removes all keys in [Key, EndKey).
	MsgDeleteRange

	// MsgUpsert applies a read-modify-write function to a key.
	MsgUpsert
)

// UpsertFn is called during upsert resolution. If the key exists, existing
// points to the current value and exists is true. Otherwise existing is nil
// and exists is false. The returned value is stored as the new value.
type UpsertFn[V any] func(existing *V, exists bool) V

// Message represents a buffered operation in a B-epsilon-tree node.
// Messages flow from root toward leaves during flushes.
type Message[K any, V any] struct {
	Kind MsgKind
	Key  K
	// EndKey is the exclusive upper bound for MsgDeleteRange. Unused for other kinds.
	EndKey K
	// Value is the payload for MsgPut. Zero value for other kinds.
	Value V
	// Fn is the upsert function for MsgUpsert. Nil for other kinds.
	Fn UpsertFn[V]

	// counted tracks whether t.size was incremented when this MsgPut was buffered.
	// Used by applyToLeaf to correct size for buffer-duplicate overwrites.
	counted bool
}
