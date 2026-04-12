package fractaltree_test

import (
	"cmp"
	"fmt"
	"sync"

	"github.com/aalhour/fractaltree"
)

// --- Constructors ---

func ExampleNew() {
	t, _ := fractaltree.New[string, int]()

	t.Put("alice", 42)
	t.Put("bob", 7)

	v, ok := t.Get("alice")
	fmt.Println(v, ok)
	fmt.Println(t.Len())

	// Output:
	// 42 true
	// 2
}

func ExampleNewWithCompare() {
	type Point struct {
		X, Y int
	}

	comparator := func(a, b Point) int {
		if c := cmp.Compare(a.X, b.X); c != 0 {
			return c
		}
		return cmp.Compare(a.Y, b.Y)
	}

	t, _ := fractaltree.NewWithCompare[Point, string](comparator)

	t.Put(Point{1, 2}, "a")
	t.Put(Point{1, 3}, "b")
	t.Put(Point{2, 1}, "c")

	for k, v := range t.All() {
		fmt.Printf("(%d,%d)=%s ", k.X, k.Y, v)
	}
	fmt.Println()

	// Output:
	// (1,2)=a (1,3)=b (2,1)=c
}

func ExampleNewDisk() {
	f := &memFlusher[string, int]{nodes: make(map[uint64][]byte)}
	t, _ := fractaltree.NewDisk[string, int](f)

	t.Put("key", 42)
	v, _ := t.Get("key")
	fmt.Println(v)
	fmt.Println(t.Close())

	// Output:
	// 42
	// <nil>
}

// --- Options ---

func ExampleWithEpsilon() {
	// Lower epsilon = larger buffers, better write amortization.
	t, _ := fractaltree.New[int, string](fractaltree.WithEpsilon(0.3))

	t.Put(1, "one")
	fmt.Println(t.Len())

	// Output:
	// 1
}

func ExampleWithBlockSize() {
	// Smaller block size = more frequent flushes and splits.
	t, _ := fractaltree.New[int, string](fractaltree.WithBlockSize(64))

	t.Put(1, "one")
	fmt.Println(t.Len())

	// Output:
	// 1
}

// --- Core operations ---

func ExampleBETree_Put() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")
	t.Put(2, "two")
	t.Put(1, "ONE") // overwrite

	v, _ := t.Get(1)
	fmt.Println(v)
	fmt.Println(t.Len())

	// Output:
	// ONE
	// 2
}

func ExampleBETree_Get() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")

	v, ok := t.Get(1)
	fmt.Println(v, ok)

	v, ok = t.Get(99)
	fmt.Println(v, ok)

	// Output:
	// one true
	//  false
}

func ExampleBETree_Delete() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")
	t.Put(2, "two")

	fmt.Println(t.Delete(1))
	fmt.Println(t.Delete(99))
	fmt.Println(t.Len())

	// Output:
	// true
	// false
	// 1
}

func ExampleBETree_DeleteRange() {
	t, _ := fractaltree.New[int, string]()

	for i := range 10 {
		t.Put(i, "v")
	}

	removed := t.DeleteRange(3, 7) // removes keys 3, 4, 5, 6
	fmt.Println("removed:", removed)
	fmt.Println("len:", t.Len())
	fmt.Println("contains 3:", t.Contains(3))
	fmt.Println("contains 7:", t.Contains(7))

	// Output:
	// removed: 4
	// len: 6
	// contains 3: false
	// contains 7: true
}

func ExampleBETree_Contains() {
	t, _ := fractaltree.New[string, int]()

	t.Put("alice", 1)

	fmt.Println(t.Contains("alice"))
	fmt.Println(t.Contains("bob"))

	// Output:
	// true
	// false
}

func ExampleBETree_Len() {
	t, _ := fractaltree.New[int, string]()

	fmt.Println(t.Len())
	t.Put(1, "one")
	t.Put(2, "two")
	fmt.Println(t.Len())

	// Output:
	// 0
	// 2
}

func ExampleBETree_Clear() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")
	t.Put(2, "two")
	t.Clear()

	fmt.Println(t.Len())
	fmt.Println(t.Contains(1))

	// Output:
	// 0
	// false
}

func ExampleBETree_Close() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")
	err := t.Close()
	fmt.Println(err)

	// Output:
	// <nil>
}

// --- Upsert operations ---

func ExampleBETree_Upsert() {
	t, _ := fractaltree.New[string, int]()

	t.Upsert("counter", fractaltree.Increment(1))
	t.Upsert("counter", fractaltree.Increment(1))
	t.Upsert("counter", fractaltree.Increment(1))

	v, _ := t.Get("counter")
	fmt.Println(v)

	// Output:
	// 3
}

func ExampleBETree_PutIfAbsent() {
	t, _ := fractaltree.New[string, string]()

	fmt.Println(t.PutIfAbsent("key", "first"))
	fmt.Println(t.PutIfAbsent("key", "second"))

	v, _ := t.Get("key")
	fmt.Println(v)

	// Output:
	// true
	// false
	// first
}

func ExampleIncrement() {
	t, _ := fractaltree.New[string, int]()

	// Increment creates an UpsertFn that adds delta to the value.
	// If the key is absent, delta becomes the initial value.
	t.Upsert("hits", fractaltree.Increment(1))
	t.Upsert("hits", fractaltree.Increment(5))

	v, _ := t.Get("hits")
	fmt.Println(v)

	// Output:
	// 6
}

func ExampleCompareAndSwap() {
	t, _ := fractaltree.New[string, string]()

	t.Put("status", "pending")

	// CAS only updates if the current value matches expected.
	t.Upsert("status", fractaltree.CompareAndSwap("pending", "active"))
	v, _ := t.Get("status")
	fmt.Println(v)

	// Mismatch: value stays unchanged.
	t.Upsert("status", fractaltree.CompareAndSwap("pending", "closed"))
	v, _ = t.Get("status")
	fmt.Println(v)

	// Output:
	// active
	// active
}

// --- Iterators ---

func ExampleBETree_All() {
	t, _ := fractaltree.New[int, string]()

	t.Put(3, "three")
	t.Put(1, "one")
	t.Put(2, "two")

	for k, v := range t.All() {
		fmt.Println(k, v)
	}

	// Output:
	// 1 one
	// 2 two
	// 3 three
}

func ExampleBETree_Ascend() {
	t, _ := fractaltree.New[int, string]()

	t.Put(3, "three")
	t.Put(1, "one")
	t.Put(2, "two")

	for k, v := range t.Ascend() {
		fmt.Println(k, v)
	}

	// Output:
	// 1 one
	// 2 two
	// 3 three
}

func ExampleBETree_Descend() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")
	t.Put(2, "two")
	t.Put(3, "three")

	for k, v := range t.Descend() {
		fmt.Println(k, v)
	}

	// Output:
	// 3 three
	// 2 two
	// 1 one
}

func ExampleBETree_Range() {
	t, _ := fractaltree.New[int, string]()

	for i := range 10 {
		t.Put(i, fmt.Sprintf("val%d", i))
	}

	// Range [3, 6) yields keys 3, 4, 5 in ascending order.
	for k, v := range t.Range(3, 6) {
		fmt.Println(k, v)
	}

	// Output:
	// 3 val3
	// 4 val4
	// 5 val5
}

func ExampleBETree_DescendRange() {
	t, _ := fractaltree.New[int, string]()

	for i := range 10 {
		t.Put(i, fmt.Sprintf("val%d", i))
	}

	// DescendRange(hi=7, lo=3) yields keys in (3, 7] = {7, 6, 5, 4} descending.
	for k, v := range t.DescendRange(7, 3) {
		fmt.Println(k, v)
	}

	// Output:
	// 7 val7
	// 6 val6
	// 5 val5
	// 4 val4
}

// --- Cursor ---

func ExampleBETree_Cursor() {
	t, _ := fractaltree.New[int, string]()

	t.Put(10, "ten")
	t.Put(20, "twenty")
	t.Put(30, "thirty")

	c := t.Cursor()
	defer c.Close()

	c.Seek(15) // positions at first key >= 15
	fmt.Println(c.Key(), c.Value())

	c.Next()
	fmt.Println(c.Key(), c.Value())

	// Output:
	// 20 twenty
	// 30 thirty
}

func ExampleCursor_Seek() {
	t, _ := fractaltree.New[int, string]()

	t.Put(10, "ten")
	t.Put(20, "twenty")
	t.Put(30, "thirty")

	c := t.Cursor()
	defer c.Close()

	// Seek positions at the first key >= the target.
	fmt.Println(c.Seek(20))
	fmt.Println(c.Key(), c.Value())

	// Non-existent key: positions at next greater.
	fmt.Println(c.Seek(15))
	fmt.Println(c.Key(), c.Value())

	// Beyond max: returns false.
	fmt.Println(c.Seek(100))

	// Output:
	// true
	// 20 twenty
	// true
	// 20 twenty
	// false
}

func ExampleCursor_Next() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")
	t.Put(2, "two")
	t.Put(3, "three")

	c := t.Cursor()
	defer c.Close()

	// Next advances forward. First call positions at the beginning.
	for c.Next() {
		fmt.Println(c.Key(), c.Value())
	}

	// Output:
	// 1 one
	// 2 two
	// 3 three
}

func ExampleCursor_Prev() {
	t, _ := fractaltree.New[int, string]()

	t.Put(1, "one")
	t.Put(2, "two")
	t.Put(3, "three")

	c := t.Cursor()
	defer c.Close()

	// Prev moves backward. First call positions at the end.
	for c.Prev() {
		fmt.Println(c.Key(), c.Value())
	}

	// Output:
	// 3 three
	// 2 two
	// 1 one
}

// --- Codec ---

func ExampleGobCodec() {
	c := fractaltree.GobCodec[string]{}

	data, _ := c.Encode("hello")
	v, _ := c.Decode(data)
	fmt.Println(v)
	fmt.Println(len(data), "bytes")

	// Output:
	// hello
	// 9 bytes
}

// --- memFlusher for DiskBETree examples ---

type memFlusher[K, V any] struct {
	mu    sync.Mutex
	nodes map[uint64][]byte
}

func (m *memFlusher[K, V]) WriteNode(id uint64, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[id] = append([]byte(nil), data...)
	return nil
}

func (m *memFlusher[K, V]) ReadNode(id uint64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %d not found", id)
	}
	return data, nil
}

func (m *memFlusher[K, V]) Sync() error  { return nil }
func (m *memFlusher[K, V]) Close() error { return nil }
