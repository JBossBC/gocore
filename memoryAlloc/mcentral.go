package memoryAlloc

type mcentral struct {
	spanClass spanClass
	partial   [2]spanSet
	full      [2]spanSet
}

func (c *mcentral) init(spc spanClass) {
	c.spanclass = spc
	lockInit(&c.partial[0].spineLock, lockRankSpanSetSpine)
	lockInit(&c.partial[1].spineLock, lockRankSpanSetSpine)
	lockInit(&c.full[0].spineLock, lockRankSpanSetSpine)
	lockInit(&c.full[1].spineLock, lockRankSpanSetSpine)
}
func (c *mcentral) cacheSpan() *mspan {
	spanBytes := uintptr(class_to_allocnpages[c.spanclass.sizeclass()]) * _PageSize
	spanBudget := 100
	var s *mspan
	var sl sweepLocker
	//mheap_.sweepgen 主要是用于管理垃圾回收器的工作进度，记录当前已经扫描的对象数量，
	//以及哪些对象已经被标记为待回收状态，从而保证垃圾回收器的工作能够高效而稳定地进行
	sg := mheap_.sweepgen
	if s = c.partialSwept(sg).pop(); s != nil {
		goto havespan
	}
	//Sweep为未在标记阶段标记的块释放或收集终结器。
	//它清除标记位，为下一轮GC做准备。如果跨度已返回堆，则返回true。如果preserve=true，则不要将其返回到堆中，也不要在mcentral列表中重新链接；打电话的人会处理好的。
	sl = sweep.active.begin()
	if sl.valid {
		for ; spanBudget >= 0; spanBudget-- {
			s = c.partialUnswept(sg).pop()
			if s == nil {
				break
			}
			if s, ok := sl.tryAcquire(s); ok {
				// We got ownership of the span, so let's sweep it and use it.
				s.sweep(true)
				sweep.active.end(sl)
				goto havespan
			}
		}
		for ; spanBudget >= 0; spanBudget-- {
			s = c.fullUnswept(sg).pop()
			if s == nil {
				break
			}
			if s, ok := sl.tryAcquire(s); ok {
				// We got ownership of the span, so let's sweep it.
				s.sweep(true)
				// Check if there's any free space.
				freeIndex := s.nextFreeIndex()
				if freeIndex != s.nelems {
					s.freeindex = freeIndex
					sweep.active.end(sl)
					goto havespan
				}
				// Add it to the swept list, because sweeping didn't give us any free space.
				c.fullSwept(sg).push(s.mspan)
			}
			// See comment for partial unswept spans.
		}
		sweep.active.end(sl)
	}
	s = c.grow()
	if s == nil {
		return nil
	}
havespan:
	if trace.enabled && !traceDone {
		traceGCSweepDone()
	}
	n := int(s.nelems) - int(s.allocCount)
	if n == 0 || s.freeindex == s.nelems || uintptr(s.allocCount) == s.nelems {
		throw("span has no free objects")
	}
	freeByteBase := s.freeindex &^ (64 - 1)
	whichByte := freeByteBase / 8
	// Init alloc bits cache.
	s.refillAllocCache(whichByte)

	// Adjust the allocCache so that s.freeindex corresponds to the low bit in
	// s.allocCache.
	s.allocCache >>= s.freeindex % 64

	return s
}

//向heap申请span
func (c *mcentral) grow() *mspan {
	npages := uintptr(class_to_allocnpages[c.spanclass.sizeclass()])
	size := uintptr(class_to_size[c.spanclass.sizeclass()])

	s := mheap_.alloc(npages, c.spanclass)
	if s == nil {
		return nil
	}

	// Use division by multiplication and shifts to quickly compute:
	// n := (npages << _PageShift) / size
	n := s.divideByElemSize(npages << _PageShift)
	s.limit = s.base() + size*n
	heapBitsForAddr(s.base()).initSpan(s)
	return s
}
