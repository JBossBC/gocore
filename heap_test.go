package golangUtil

import (
	"fmt"
	"testing"
)

func TestHeap(t *testing.T) {
	fmt.Println("hello")
	h := &heap{
		heapArr: make([]int, 100),
		value:   make([]uintptr, 100),
		length:  1,
	}
	h.insert(100, uintptr(1))
	h.insert(101, uintptr(2))
	h.insert(102, uintptr(3))
	h.insert(103, uintptr(4))
	h.insert(104, uintptr(5))
	h.insert(105, uintptr(6))
	h.insert(106, uintptr(7))
	println(h.peekMax())
	h.delete(h.length)
	h.delete(1)
	h.delete(1)
	h.delete(1)
	h.delete(1)
	h.delete(1)
	h.delete(1)

	println(h.peekMax())
}
