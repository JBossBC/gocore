package memoryAlloc

type mcache struct {
	nextSample uintptr // trigger heap sample after allocating this many bytes
	scanAlloc  uintptr // bytes of scannable heap allocated

	tiny       uintptr
	tinyoffset uintptr
	tinyAllocs uintptr

	alloc [numSpanClasses]*mspan // spans to allocate from, indexed by spanClass
	//事实上，在 Go 语言中，对于小尺寸的内存块（小于等于 32KB），具体的分配方式是由 mcache 和 stackfreelist 管理的。每个线程都有自己的本地缓存 mcache，其中包含了由 stackfreelist 维护的多个堆栈内存块。
	//
	//stackfreelist 中的每个元素包括一个指向下一个空闲内存块的指针，以及内存块的大小等信息。当线程需要从本地缓存中获取内存时，它会先在 stackfreelist 中寻找合适大小的内存块。
	//如果没有找到，则会从全局的 runtime.stackpool 中获取内存，并将其切分成合适大小的内存块，然后添加到 stackfreelist 中。
	stackcache [_NumStackOrders]stackfreelist

	flushGen uint32
}
