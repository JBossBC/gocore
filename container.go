package golangUtil

type heap struct {
	heapArr []int
	value   []uintptr
	length  int
}

func (h *heap) down(index int) {
	var result = index
	if index*2 < h.length && h.heapArr[result] < h.heapArr[index*2] {
		result = index * 2
	}
	if index*2+1 < h.length && h.heapArr[result] < h.heapArr[index*2+1] {
		result = index*2 + 1
	}
	if result != index {
		h.swap(index, result)
		h.down(result)
	}
}
func (h *heap) up(index int) {
	for index/2 > 0 {
		if h.heapArr[index] > h.heapArr[index/2] {
			h.swap(index, index/2)
			index = index / 2
		} else {
			break
		}
	}
}
func (h *heap) peekMin() uintptr {
	return h.value[h.length-1]
}
func (h *heap) insert(key int, value uintptr) {
	h.length++
	if h.length > cap(h.heapArr) {
		h.heapArr = append(h.heapArr, make([]int, 100)...)
		h.value = append(h.value, make([]uintptr, 100)...)
	}
	h.heapArr[h.length] = key
	h.value[h.length] = value
	h.down(h.length)
	h.up(h.length)
}
func (h *heap) delete(index int) {
	h.swap(index, h.length)
	h.length--
	h.down(index)
	h.up(index)
}
func (h *heap) peekMax() uintptr {
	if h.length <= 0 {
		return uintptr(0)
	}
	return h.value[1]
}
func (h *heap) swap(heapPre int, heapLast int) {
	h.heapArr[heapPre] = h.heapArr[heapPre] ^ h.heapArr[heapLast]
	h.heapArr[heapLast] = h.heapArr[heapPre] ^ h.heapArr[heapLast]
	h.heapArr[heapPre] = h.heapArr[heapPre] ^ h.heapArr[heapLast]
	h.value[heapPre] = h.value[heapPre] ^ h.value[heapLast]
	h.value[heapLast] = h.value[heapPre] ^ h.value[heapLast]
	h.value[heapPre] = h.value[heapPre] ^ h.value[heapLast]
}
