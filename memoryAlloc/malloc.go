package memoryAlloc

import "unsafe"

var (
	_TinySize      = 16
	_TinySizeClass = int8(2)
)

type linearAlloc struct {
	next   uintptr // next free byte
	mapped uintptr // one byte past end of mapped space
	end    uintptr // end of reserved space

	mapMemory bool // transition memory from Reserved to Ready if true
}

func newobject(typ *_type) unsafe.Pointer {
	return mallocgc(typ.size, typ, true)
}

func mallocgc(size uintptr, typ *_type, needzero bool) unsafe.Pointer {
	//前置处理
	//小对象的处理
	if size <= maxSmallSize {
		//是否是noscan(指针),是否是微小对象
		if noscan && size < maxTinySize {
			off := c.tinyoffset
			//内存对齐
			if size&7 == 0 {
				off = alignUp(off, 8)
			} else if goarch.PtrSize == 4 && size == 12 {
				off = alignUp(off, 8)
			} else if size&3 == 0 {
				off = alignUp(off, 4)
			} else if size&1 == 0 {
				off = alignUp(off, 2)
			}
			if off+size <= maxTinySize && c.tiny != 0 {
				x = unsafe.Pointer(c.tiny + off)
				c.tinyoffset = off + size
				//分配了多少个对象
				c.tinyAllocs++
				mp.mallocing = 0
				releasem(mp)
				return x
			}
			span = c.alloc[tinySpanClass]
			//在mcache中寻找空闲的微小对象空间
			v := nextFreeFast(span)
			if v == 0 {
				//向mcentral中寻找微小的内存空间
				v, span, shouldhelpgc = c.nextFree(tinySpanClass)
			}
			//获取mspan中的一个tinySpanClass 空间
			x = unsafe.Pointer(v)
			(*[2]uint64)(x)[0] = 0
			(*[2]uint64)(x)[1] = 0
			if !raceenabled && (size < c.tinyoffset || c.tiny == 0) {
				c.tiny = uintptr(x)
				c.tinyoffset = size
			}
			size = maxTinySize
		} else {
			var sizeclass uint8
			if size <= smallSizeMax-8 {
				sizeclass = size_to_class8[divRoundUp(size, smallSizeDiv)]
			} else {
				sizeclass = size_to_class128[divRoundUp(size-smallSizeMax, largeSizeDiv)]
			}
			size = uintptr(class_to_size[sizeclass])
			spc := makeSpanClass(sizeclass, noscan)
			span := c.alloc[spc]
			v := nextFreeFast(span)
			if v == 0 {
				v, span, shouldhelpgc = c.nextFree(spc)
			}
			x = unsafe.Pointer(v)
			if needzero && span.needzero != 0 {
				memclrNoHeapPointers(unsafe.Pointer(v), size)
			}
		}
	} else {
		shouldhelpgc = true
		//大对象的分配
		span = c.allocLarge(size, noscan)
		span.freeindex = 1
		span.allocCount = 1
		size = span.elemsize
		x = unsafe.Pointer(span.base())
		if needzero && span.needzero != 0 {
			if noscan {
				delayedZeroing = true
			} else {
				memclrNoHeapPointers(x, size)
			}
		}
	}
	var scanSize uintptr

	if !noscan {
		heapBitsSetType(uintptr(x), size, dataSize, typ)
		if dataSize > typ.size {
			if typ.ptrdata != 0 {
				scanSize = dataSize - typ.size + typ.ptrdata
			}
		} else {
			scanSize = typ.ptrdata
		}
		c.scanAlloc += scanSize
	}
	publicationBarrier()
	return x
}
func (c *mcache) nextFree(spc spanClass) (v *gclinkptr, s *mspan, shouldhelpgc bool) {
	s = c.alloc[spc]
	shouldhelpgc = false
	freeIndex := s.nextFreeIndex()
	if freeIndex == s.nelems {
		//span is full
		if uintptr(s.allocCount) != s.nelems {
			println("runtime: s.allocCount=", s.allocCount, "s.nelems=", s.nelems)
			throw("s.allocCount != s.nelems && freeIndex == s.nelems")
		}
		//向central申请空余span(central调用cacheSpan())
		c.refill(spc)
		shouldhelpgc = true
		s = c.alloc[spc]
		freeIndex = s.nextFreeIndex()
	}
	if freeIndex >= s.nelems {
		throw("freeIndex is not valid")
	}

	//gclinkptr是golang中的一个指针类型，它被用来封装某些指针，以保证这些指针所引用的对象不会被垃圾回收器扫描和回收
	v = gclinkptr(freeIndex*s.elemsize + s.base())
	s.allocCount++
	if uintptr(s.allocCount) > s.nelems {
		println("s.allocCount=", s.allocCount, "s.nelems=", s.nelems)
		throw("s.allocCount > s.nelems")
	}
	return
}

func nextFreeFast(s *mspan) gclinkptr {
	theBit := sys.Ctz64(s.allocCache) // Is there a free object in the allocCache?
	if theBit < 64 {
		result := s.freeindex + uintptr(theBit)
		if result < s.nelems {
			freeidx := result + 1
			if freeidx%64 == 0 && freeidx != s.nelems {
				return 0
			}
			s.allocCache >>= uint(theBit + 1)
			s.freeindex = freeidx
			s.allocCount++
			return gclinkptr(result*s.elemsize + s.base())
		}
	}
	return 0
}
