package memoryAlloc

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	minPhysPageSize        = 4096
	maxPhysPageSize        = 512 << 10
	maxPhysHugePageSize    = pallocChunkBytes
	pagesPerReclaimerChunk = 512
	physPagesAlignedStacks = GOOS == "openbsd"
)

type mheap struct {
	lock sync.Mutex
	//arena对page的管理
	pages pageAlloc //page allocation data structure
	//sweep到多少个span,只能在STW时被修改
	sweepgen uint32 //
	//一个span的管理单元(span可以被理解为面对程序员的内存分配单元，而arena则可以理解为面向heap的管理单元)
	//不管是mcache还是mcentral还是mheap在实际的对象分配中，都利用特定spanClass的span进行分配
	//而对于arena 则有助于heap对pages进行管理以及GC
	allspans []*mspan

	//pagesInUse 记录了当前已经被分配出去的内存页数量，换句话说，它表示了当前已经被程序占用的堆内存大小。这个字段通常会被用于检查堆内存使用情况，以便进行垃圾回收或内存释放等操作。
	//
	//需要注意的是，pagesInUse 只记录了已经被分配出去的内存页数量，并不能反映当前正在使用的内存页数量，因为有些内存页可能已经被释放或空闲了。
	//如果要准确地知道当前正在使用的内存页数量，可以通过遍历整个堆内存，统计正在使用的内存页数量来获取。
	pagesInUse atomic.Uint64
	//pagesWept 记录了垃圾回收器在执行垃圾回收操作时扫描过的内存页数量。垃圾回收器会遍历整个程序中分配的内存，找出不再使用的对象并将其释放，而 pagesWept 就是用来记录垃圾回收器扫描的内存页数，以便监控垃圾回收器的性能和效率。
	//
	//需要注意的是，pagesWept 只记录被垃圾回收器扫描过的内存页数量，并不能反映当前正在使用的内存页数量。另外，pagesWept 的值在垃圾回收结束后会被清空，以便下一次垃圾回收器扫描时重新计数。
	pagesSwept atomic.Uint64
	//pagesSweptBasis 记录了上一次垃圾回收器执行垃圾回收时扫描的内存页数量。当垃圾回收器开始执行垃圾回收时，它会在 mheap 中记录当前的 pagesInUse 值，并将其保存到 pagesSweptBasis 中。
	//这样，当垃圾回收完成后，就可以计算出垃圾回收器扫描的内存页数量，即 pagesSweptBasis - mheap.pagesInUse。pagesSweptBasis - mheap.pagesInUse的值代表了在执行垃圾回收后被释放的内存页数量。这个值越大，表明垃圾回收器释放的内存越多，程序的性能和内存占用情况都有可能得到改善。这个值被用于监控垃圾回收器的性能和效率。
	//
	//需要注意的是，pagesSweptBasis 只记录上一次垃圾回收器扫描时的内存页数量，并不能反映当前正在使用的内存页数量。另外，pagesSweptBasis 的值会在垃圾回收器开始执行垃圾回收时被更新。
	pagesSweptBasis atomic.Uint64
	//golang的mheap中的sweepHeapLiveBasis是一个用于垃圾回收的参数，表示在垃圾回收过程中参考的heapLive的值。heapLive是一个表示当前堆中活跃对象大小的指标
	sweepHeapLiveBasis uint64
	//每次扫描需要处理多少个page
	sweepPagesPerByte float64
	scavengeGoal      uint64
	reclaimIndex      atomic.Uint64
	//mheap.reclaimCredit是用于表示被垃圾回收器释放的内存页数量的字段。
	//
	//具体来说，当垃圾回收器扫描到一些可以被释放的内存页时，它会将这些内存页的数量添加到mheap.reclaimCredit中。
	//然后，在下一次分配内存时，mheap会优先使用已经被标记为“可回收”的内存页，而不是从操作系统中申请新的内存页，以提高程序的性能和效率。
	//需要注意的是，mheap.reclaimCredit并不记录实际被释放的内存页数量，而是记录了垃圾回收器的“信誉值”。
	//这个“信誉值”反映了垃圾回收器在过去多次垃圾回收中释放内存的能力和效率，它会随着时间的推移逐渐减小。
	//如果垃圾回收器释放的内存页数量超过了mheap.reclaimCredit的值，那么会重新开始向操作系统申请新的内存页。
	reclaimCredit atomic.Uintptr
	//arenas 将其切分成更细小的空间(heapArena 64MB)去管理
	arenas         [1 << arenaL1Bits]*[1 << arenaL2Bits]*heapArena
	heapArenaAlloc linearAlloc
	arenaHints     *arenaHint
	arena          linearAlloc
	//所有的arena(建立索引)
	allArenas []arenaIdx
	//正在sweep的arena
	sweepArenas []arenaIdx
	curArena    struct {
		base, end uintptr
	}
	//用于内存对齐
	_ uint32
	//对于每一个spanClass都会有一个mcentral
	central [numSpanClasses]struct {
		mcentral mcentral
		pad      [cpu.CacheLinePadSize - unsafe.Sizeof(mcentral{})%cpu.CacheLinePadSize]byte
	}
	spanalloc             fixalloc
	cachealloc            fixalloc
	specialfinalizeralloc fixalloc
	specialprofilealloc   fixalloc
	specialReachableAlloc fixalloc
	speciallock           sync.Mutex
	arenaHintAlloc        fixalloc
	unused                *specialfinalizer
}

var mheap_ mheap

/**
  对heap划分的更小管理单元
*/
type heapArena struct {
	bitmap       [heapArenaBitmapBytes]byte
	spans        [pagesPerArena]*mspan
	pageInUse    [pagesPerArena / 8]uint8
	pageMarks    [pagesPerArena / 8]uint8
	pageSpecials [pagesPerArena / 8]uint8
	checkmarks   *checkmarksMap
	zeroedBase   uintptr
}

//Go 的内存管理器会在当前 arena 中查找空闲的内存空间用于分配。如果当前 arena 中已经没有足够的空闲内存，则会通过从操作系统申请新的 arena 来满足分配需求。
//
//arenaHint 就是在这个过程中被使用的辅助数据结构。它记录了 arena 中可能有可用内存的位置。
//具体来说，arenaHint 结构体中包含 addr 字段，表示在 arena 中下一个可能的空闲内存的地址；down 字段表示下一个空闲内存是否在 addr 下方（向下增长的情况）；next 字段则表示下一个 arenaHint 结构体，用于形成简单链表以便遍历。
//Go 的内存管理器通过遍历 arenaHint 的链表，查找空闲内存适合进行内存分配。
type arenaHint struct {
	addr uintptr
	down bool
	next *arenaHint
}

type mSpanState uint8

const (
	mSpanDead mSpanState = iota
	mSpanInUse
	mSpanManual
)

var mSpanStateNames = []string{
	"mSpanDead",
	"mSpanInUse",
	"mSpanManual",
	"mSpanFree",
}

//为mspanState提供原子性操作
type mSpanStateBox struct {
	s mSpanState
}

func (b *mSpanStateBox) set(s mSpanState) {
	atomic.Store8((*uint8)(&b.s), uint8(s))
}

func (b *mSpanStateBox) get() mSpanState {
	return mSpanState(atomic.Load8((*uint8)(&b.s)))
}

type mSpanList struct {
	first *mspan
	last  *mspan
}

type mspan struct {
	next *mspan
	prev *mspan
	list *mSpanList
	//offset
	startAddr uintptr
	//size
	npages         uintptr
	manualFreeList gclinkptr
	freeindex      uintptr
	nelems         uintptr

	//allocache 是用于确定从哪个 mspan 的本地缓存中分配内存的机制。它根据需要分配的内存大小，选择对应的 mspan 和其中的本地缓存。
	//如果所选 mspan 中的本地缓存没有可用内存，allocache 则会继续寻找其他的 mspan 直到找到可用的内存为止。
	//如果所有的 mspan 都没有可用内存，allocache 会向全局的 mheap 中申请内存，并将其分配给新的 mspan。
	allocCache  uint64
	allocBits   *gcBits
	gcmarkBits  *gcBits
	sweepgen    uint32
	divMul      uint32
	allocCount  uint16
	spanclass   spanClass
	state       mSpanStateBox
	needzero    uint8
	elemsize    uintptr
	limit       uintptr
	speciallock sync.Mutex
	specials    *special
}

func (s *mspan) base() uintptr {
	return s.startAddr
}
func (s *mspan) layout() (size, n, total uintptr) {
	total = s.npages << _PageShift
	size = s.elemsize
	if size > 0 {
		n = total / size
	}
	return
}

//recordspan 函数是用于记录 mspan 中已经被分配的内存块数量的函数。它主要是在 mheap 中使用的，mheap 是 Go 运行时系统中用来管理堆内存的核心数据结构之一。
//
//具体来说，recordspan 函数会更新 mspan 的相关信息，如已分配的内存块数量，以及 mspan 在 heap 中的起始和终止地址等。这些信息可以帮助 mheap 在分配新的内存块时快速找到可用的空间。

func revordspan(vh unsafe.Pointer, p unsafe.Pointer) {
	h := (*mheap)(vh)
	s := (*span)(p)
	assertLockHeld(&h.lock)
	if len(h.allspans) >= cap(h.arenaHintAlloc) {
		n := 64 * 1024 / goarch.PtrSize
		if n < cap(h.allspans)*3/2 {
			n = cap(h.allspans) * 3 / 2
		}
		var new []*mspan
		sp := (*slice)(unsafe.Pointer(&new))
		sp.array = sysAlloc(uintptr(n)*goarch.PtrSize, &memstats.other_sys)
		if sp.array == nil {
			throw("runtime: cannot allocate memory")
		}
		sp.len = len(h.allspans)
		sp.cap = n
		if len(h.allspans) > 0 {
			copy(new, h.allspans)
		}
		oldAllspans := h.allspans
		*(*notInHeapSlice)(unsafe.Pointer(&h.allspans)) = *(*notInHeapSlice)(unsafe.Pointer(&new))
		if len(oldAllspans) != 0 {
			sysFree(unsafe.Pointer(&oldAllspans[0]), uintptr(cap(oldAllspans))*unsafe.Sizeof(oldAllspans[0]), &memstats.other_sys)
		}
	}
	h.allspans = h.allspans[:len(h.allspans)+1]
	h.allspans[len(h.allspans)-1] = s
}

type spanClass uint8

const (
	numSpanClasses = _NumSizeClasses << 1
	tinySpanClass  = spanClass(tinySizeClass<<1 | 1)
)

func makeSpanClass(sizeclass uint8, noscan bool) spanClass {
	return spanClass(sizeclass<<1) | spanClass(bool2int(noscan))
}

func (sc spanClass) sizeclass() int8 {
	return int8(sc >> 1)
}

func (sc spanClass) noscan() bool {
	return sc&1 != 0
}
func arenaIndex(p uintptr) arenaIdx {
	return arenaIdx((p - arenaBaseOffset) / heapArenaBytes)
}

func arenaBase(i arenaIdx) uintptr {
	return uintptr(i)*heapArenaBytes + arenaBaseOffset
}

type arenaIdx uint

func (i arenaIdx) l1() uint {
	if arenaL1Bits == 0 {
		// Let the compiler optimize this away if there's no
		// L1 map.
		return 0
	} else {
		return uint(i) >> arenaL1Shift
	}
}

func (i arenaIdx) l2() uint {
	if arenaL1Bits == 0 {
		return uint(i)
	} else {
		return uint(i) & (1<<arenaL2Bits - 1)
	}
}
func inheap(b uintptr) bool {
	return spanOfHeap(b) != nil
}

func inHeapOrStack(b uintptr) bool {
	s := spanOf(b)
	if s == nil || b < s.base() {
		return false
	}
	switch s.state.get() {
	case mSpanInUse, mSpanManual:
		return b < s.limit
	default:
		return false
	}
}

func (h *mheap) alloc(npages uintptr, spanclass spanClass) *mspan {
	var s *mspan
	//系统栈是与用户栈不同的一种栈，用于执行一些必须保证不被抢占、不会增长用户栈且可能需要切换 goroutine 的任务。为了避免系统栈上的信息被垃圾回收器扫描和回收，Golang 采用了一些特殊的机制来管理系统栈的生命周期。
	//
	//其中，systemstack 就是一个运行时函数，它可以暂时切换到系统栈，从而执行那些必须保证不可被抢占的任务。以systemstack为代表的这些函数，常常是一些跨平台调用操作系统 API 或执行底层内存分配等操作的场景下会用到。
	systemstack(func() {
		if !isSweepDone() {
			h.reclaim(npages)
		}
		s = h.allocSpan(npages, spanAllocHeap, spanclass)
	})
	return s
}

//TODO cant
func (m *mheap) allocSpan(npages uintptr, typ spanAllocType, spanclass spanClass) (s *mspan) {
	gp := getg()
	base, scav := uintptr(0), uintptr(0)
	growth := uintptr(0)
	needPhysPageAlign := physPageAlignedStacks && typ == spanAllocStack && pageSize < physPageSize
	pp := gp.m.p.ptr()
}
