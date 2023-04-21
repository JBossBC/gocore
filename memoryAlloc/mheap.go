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
	pages    pageAlloc //page allocation data structure
	sweepgen uint32    //
	//一个span的管理单元
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
	pagesSweptBasis    atomic.Uint64
	sweepHeapLiveBasis uint64
	sweepPagesPerByte  float64
	scavengeGoal       uint64
	reclaimIndex       atomic.Uint64
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
	allArenas      []arenaIdx
	sweepArenas    []arenaIdx
	curArena       struct {
		base, end uintptr
	}
	_       uint32
	central [numSpanClasses]struct {
		mcentral mcentral
		pad      [cpu.CacheLinePadSize - unsafe.Sizeof(mcentral{})%cpu.CacheLinePadSize]byte
	}
}

var mheap_ mheap

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
	next           *mspan
	prev           *mspan
	list           *mSpanList
	startAddr      uintptr
	npages         uintptr
	manualFreeList gclinkptr
	freeindex      uintptr
	nelems         uintptr
	allocCache     uint64
	allocBits      *gcBits
	gcmarkBits     *gcBits
	sweepgen       uint32
	divMul         uint32
	allocCount     uint16
	spanclass      spanClass
	state          mSpanStateBox
	needzero       uint8
	elemsize       uintptr
	limit          uintptr
	speciallock    sync.Mutex
	specials       *special
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
