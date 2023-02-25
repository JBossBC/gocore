package golangUtil

// FIFO cache obsolescence policy
type fifo struct {
	slice      []any
	head, tail int
}

var globalFIFO *fifo = &fifo{head: 0, tail: 0}

func (f *fifo) Enqueue(data any) {
	f.slice[f.tail] = data
	if f.tail+1 == f.head {
		expansionCap(f)
	}
	f.tail++
}

func (f *fifo) Dequeue() any {
	result := f.slice[f.head]
	f.head++
	return result
}
func expansionCap(f *fifo) {
	replace := make([]any, len(f.slice)*2)
	i := f.head
	var start int
	for ; i+1 != f.tail; i = (i + 1) % len(f.slice) {
		replace[start] = f.slice[i]
		start++
	}

	f.head = 0
	f.tail = start
}
