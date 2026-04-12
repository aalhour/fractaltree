package fractaltree_test

import (
	"cmp"
	"fmt"

	"github.com/aalhour/fractaltree"
)

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

func ExampleBETree_Range() {
	t, _ := fractaltree.New[int, string]()

	for i := range 10 {
		t.Put(i, fmt.Sprintf("val%d", i))
	}

	for k, v := range t.Range(3, 6) {
		fmt.Println(k, v)
	}

	// Output:
	// 3 val3
	// 4 val4
	// 5 val5
}

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
