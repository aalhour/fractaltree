package fractaltree

import (
	"cmp"
	"fmt"
	"iter"
)

// Flusher defines the storage backend for a DiskBETree. Implementations
// handle persisting and retrieving serialized node data.
type Flusher[K, V any] interface {
	// WriteNode persists the serialized node data under the given ID.
	WriteNode(id uint64, data []byte) error

	// ReadNode retrieves the serialized node data for the given ID.
	ReadNode(id uint64) ([]byte, error)

	// Sync ensures all written data is durable.
	Sync() error

	// Close releases any resources held by the flusher.
	Close() error
}

// DiskOption configures a DiskBETree.
type DiskOption[K, V any] func(*diskOptions[K, V])

type diskOptions[K, V any] struct {
	treeOpts []Option
	keyCodec Codec[K]
	valCodec Codec[V]
}

// WithKeyCodec sets the codec used to serialize keys for disk storage.
// Defaults to GobCodec if not set.
func WithKeyCodec[K, V any](c Codec[K]) DiskOption[K, V] {
	return func(o *diskOptions[K, V]) {
		o.keyCodec = c
	}
}

// WithValueCodec sets the codec used to serialize values for disk storage.
// Defaults to GobCodec if not set.
func WithValueCodec[K, V any](c Codec[V]) DiskOption[K, V] {
	return func(o *diskOptions[K, V]) {
		o.valCodec = c
	}
}

// WithDiskEpsilon sets the epsilon parameter for the underlying tree.
func WithDiskEpsilon[K, V any](eps float64) DiskOption[K, V] {
	return func(o *diskOptions[K, V]) {
		o.treeOpts = append(o.treeOpts, WithEpsilon(eps))
	}
}

// WithDiskBlockSize sets the block size for the underlying tree.
func WithDiskBlockSize[K, V any](size int) DiskOption[K, V] {
	return func(o *diskOptions[K, V]) {
		o.treeOpts = append(o.treeOpts, WithBlockSize(size))
	}
}

func resolveDiskOptions[K, V any](opts []DiskOption[K, V]) diskOptions[K, V] {
	o := diskOptions[K, V]{
		keyCodec: GobCodec[K]{},
		valCodec: GobCodec[V]{},
	}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// DiskBETree wraps a BETree with a Flusher for disk persistence.
// All tree operations are delegated to the in-memory BETree. The Flusher
// is called during Close to sync data. Full lazy-loading and flush-time
// persistence are extension points for future implementation.
type DiskBETree[K any, V any] struct {
	tree     *BETree[K, V]
	flusher  Flusher[K, V]
	keyCodec Codec[K]
	valCodec Codec[V]
}

// NewDisk creates a DiskBETree for keys that satisfy [cmp.Ordered].
func NewDisk[K cmp.Ordered, V any](
	f Flusher[K, V],
	opts ...DiskOption[K, V],
) (*DiskBETree[K, V], error) {
	return NewDiskWithCompare(cmp.Compare, f, opts...)
}

// NewDiskWithCompare creates a DiskBETree with a user-supplied comparator.
func NewDiskWithCompare[K any, V any](
	compare func(K, K) int,
	f Flusher[K, V],
	opts ...DiskOption[K, V],
) (*DiskBETree[K, V], error) {
	o := resolveDiskOptions(opts)
	tree, err := NewWithCompare[K, V](compare, o.treeOpts...)
	if err != nil {
		return nil, err
	}
	return &DiskBETree[K, V]{
		tree:     tree,
		flusher:  f,
		keyCodec: o.keyCodec,
		valCodec: o.valCodec,
	}, nil
}

// Put delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Put(key K, value V) { d.tree.Put(key, value) }

// Get delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Get(key K) (V, bool) { return d.tree.Get(key) }

// Delete delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Delete(key K) bool { return d.tree.Delete(key) }

// DeleteRange delegates to the underlying BETree.
func (d *DiskBETree[K, V]) DeleteRange(lo, hi K) int { return d.tree.DeleteRange(lo, hi) }

// Contains delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Contains(key K) bool { return d.tree.Contains(key) }

// Len delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Len() int { return d.tree.Len() }

// Clear delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Clear() { d.tree.Clear() }

// Upsert delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Upsert(key K, fn UpsertFn[V]) { d.tree.Upsert(key, fn) }

// PutIfAbsent delegates to the underlying BETree.
func (d *DiskBETree[K, V]) PutIfAbsent(key K, v V) bool { return d.tree.PutIfAbsent(key, v) }

// All delegates to the underlying BETree.
func (d *DiskBETree[K, V]) All() iter.Seq2[K, V] { return d.tree.All() }

// Ascend delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Ascend() iter.Seq2[K, V] { return d.tree.Ascend() }

// Descend delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Descend() iter.Seq2[K, V] { return d.tree.Descend() }

// Range delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Range(lo, hi K) iter.Seq2[K, V] { return d.tree.Range(lo, hi) }

// DescendRange delegates to the underlying BETree.
func (d *DiskBETree[K, V]) DescendRange(hi, lo K) iter.Seq2[K, V] {
	return d.tree.DescendRange(hi, lo)
}

// Cursor delegates to the underlying BETree.
func (d *DiskBETree[K, V]) Cursor() *Cursor[K, V] { return d.tree.Cursor() }

// Close syncs and closes the flusher, then marks the tree as closed.
func (d *DiskBETree[K, V]) Close() error {
	if err := d.flusher.Sync(); err != nil {
		return fmt.Errorf("flusher sync: %w", err)
	}
	if err := d.flusher.Close(); err != nil {
		return fmt.Errorf("flusher close: %w", err)
	}
	return d.tree.Close()
}
