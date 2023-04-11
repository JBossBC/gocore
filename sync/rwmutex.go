package sync

import (
	"runtime/race"
	"sync/atomic"
	"unsafe"
)

type RWMutex struct {
	w           Mutex
	writerSem   uint32
	readerSem   uint32
	readerCount int32
	readerWait  int32
}

const rwmutexMaxReader = 1 << 30

func (rw *RWMutex) RLock() {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	if atomic.AddInt32(&rw.readerCount, 1) < 0 {
		runtime_SemacquireMutex(&rw.readerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
	}
}

func (rw *RWMutex) TryRLock() bool {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	for {
		c := atomic.LoadInt32(&rw.readerCount)
		if c < 0 {
			if race.Enabled {
				race.Enable()
			}
			return false
		}
		if atomic.CompareAndSwapInt32(&rw.readerCount, c, c+1) {
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(&rw.readerSem))
			}
			return true
		}
	}
}

func (rw *RWMutex) RUnlock() {
	if race.Enabled {
		_ = rw.w.state
		race.ReleaseMerge(unsafe.Pointer(&rw.writerSem))
		race.Disable()
	}
	if r := atomic.AddInt32(&rw.readerCount, -1); r < 0 {
		rw.rUnlockSlow(r)
	}
	if race.Enabled {
		race.Enable()
	}
}
func (rw *RWMutex) rUnlockSlow(r int32) {
	if r+1 == 0 || r+1 == -rwmutexMaxReader {
		race.Enable()
		throw("sync: RUnlock of unlocked RWMutex")
	}
	if atomic.AddInt32(&rw.readerWait, -1) == 0 {
		runtime_Semrelease(&rw.writerSem, false, 1)
	}
}
func (rw *RWMutex) Lock() {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	rw.w.Lock()
	r := atomic.AddInt32(&rw.readerCount, -rwmutexMaxReader) + rwmutexMaxReader
	if r != 0 && atomic.AddInt32(&rw.readerWait, r) != 0 {
		runtime_SemacquireMutex(&rw.writerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
}
func (rw *RWMutex) TryLock() bool {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	if !rw.w.TryLock() {
		if race.Enabled {
			race.Enable()
		}
		return false
	}
	if atomic.CompareAndSwapInt32(&rw.readerCount, 0, -rwmutexMaxReader) {
		rw.w.Unlock()
		if race.Enabled {
			race.Enable()
		}
		return false
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
	return true
}
