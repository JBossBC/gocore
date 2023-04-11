package sync

import (
	"runtime/race"
	"sync/atomic"
	"unsafe"
)

func throw(string)

type Mutex struct {
	state int32
	sema  uint32
}
type Locker interface {
	Lock()
	Unlock()
}

const (
	// zero value represent the mutex unlocked
	mutexLocked = 1 << iota
	//mutexWoken 就是用来记录是否需要通知其他 goroutine 的标记位。
	//具体来说，当一个 goroutine 成功获取到锁时，如果有其他 goroutine 在等待该锁，则会将 mutexWoken 标记位置为 1，表示需要通知其他 goroutine 重新获取锁。
	//这样，等待队列中的 goroutine 在等待一定时间后就会被唤醒，然后重新尝试获取锁。
	mutexWoken
	mutexStarving
	//Mutex 互斥锁类型使用一个 int32 类型的字段 state 来表示锁的状态。
	//其中，最低位是用来标记锁是否被占用，高位则表示当前等待获取该锁的 goroutine 的数量。
	//具体来说，通过右移操作可以在高位腾出空间存储等待 goroutine 的数量，
	//mutexWaiterShift 就是为了进行右移操作而定义的一个常量，表示需要右移多少位来存储等待 goroutine 的数量。
	mutexWaiterShift      = iota
	starvationThresholdNs = 1e6
)

func (m *Mutex) Lock() {
	//CAS
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		if race.Enabled {
			race.Acquire(unsafe.Pointer(m))
		}
		return
	}
	m.lockSlow()
}

func (m *Mutex) TryLock() bool {
	old := m.state
	if old&(mutexLocked|mutexStarving) != 0 {
		return false
	}
	if !atomic.CompareAndSwapInt32(&m.state, old, old|mutexLocked) {
		return false
	}
	if race.Enabled {
		race.Acquire(unsafe.Pointer(m))
	}
	return true
}
func (m *Mutex) lockSlow() {
	//wait 开始的时间,用于切换mutex state ,(starving mode)
	var waitStartTime int64
	//饥饿状态
	starving := false
	// mutex 基于常规模式的goroutinue 有两种唤醒方式，一种为FIFO规则唤醒的方式，另一种为正在自旋的goroutinue抢占式的唤醒方式
	awoke := false
	//自旋次数
	iter := 0
	// old state
	old := m.state
	for {
		//runtime_canSpin() 函数接受一个整数参数 iter，代表自旋的次数。
		//函数中判断了多个条件，包括当前的 GOMAXPROCS 值是否大于 1，当前系统是否是支持并发的，以及自旋次数是否超过了一定的阈值。
		//通过这些条件的判断，可以确定是否需要进行自旋操作，或者直接将协程挂起等待资源。

		// mutex 被处于锁住状态并且mutex没有处于饥饿状态,那么就在一定条件下开启自旋
		if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
			if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
				atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
				awoke = true
			}
			//在 runtime_doSpin 函数中，会根据当前 CPU 的计时器（TSC, Time Stamp Counter）来实现自旋等待。具体来说，如果当前的 TSC 值减去上次获取锁时的 TSC 值小于一个阈值 spinThreshold，那么就继续执行自旋操作；否则就停止自旋并返回 false，让 goroutine 进入阻塞状态。
			//
			//需要注意的是，自旋次数过多或自旋时间过长，可能会导致 CPU 繁忙，从而影响系统的正常运行。
			//因此，在实现使用自旋操作的同步原语时，需要根据应用场景和硬件环境等因素来调整自旋步长和自旋阈值等参数。
			runtime_doSpin()
			iter++
			old = m.state
			continue
		}
		//进入下步的可能:
		// ①、mutex没有处于锁住状态
		//②、mutex处于饥饿状态
		// ③、mutex 不能满足自旋的条件
		// mutex 期待的状态
		new := old
		//如果mutex 没有处于饥饿状态，那么将mutex的状态变为 locked
		if old&mutexStarving == 0 {
			new |= mutexLocked
		}
		//如果mutex 处于锁住状态或者处于饥饿状态
		if old&(mutexLocked|mutexStarving) != 0 {
			new += 1 << mutexWaiterShift
		}
		// mutex 处于  locked状态并且当前goroutine处于饥饿状态
		if starving && old&mutexLocked != 0 {
			new |= mutexStarving
		}
		//如果 当前goroutine 状态为 woken,
		if awoke {
			if new&mutexWoken == 0 {
				throw("sync: inconsistent mutex state")
			}
			new &^= mutexWoken
		}
		// 期待的状态是否能够进行CAS
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			//如果old 没有处于locked或者starving状态，直接获取锁
			if old&(mutexLocked|mutexStarving) == 0 {
				break
			}
			//下面是饥饿状态的基于信号量的拿锁,等待状态越长放在前面
			queueLifo := waitStartTime != 0
			if waitStartTime == 0 {
				waitStartTime = runtime_nanotime()
			}
			//SemacquireMutex 是 golang runtime 包中的一个方法，用于获取互斥锁（mutex）。
			//在实现过程中，该方法使用了操作系统提供的原子操作实现了一个基于信号量的等待队列机制。
			//具体来说，当一个 goroutine 尝试获取锁时，如果锁已经被其他 goroutine 获取，则会将自己加入到等待队列中，并阻塞当前 goroutine 线程，直到锁被释放后重新激活该 goroutine。

			//runtime_SemacquireMutex 的使用方法类似于 Semacquire 函数，但是用来阻塞互斥对象的。具体来说，该函数包含三个参数：一个是锁或信号量的地址，另一个是一个 bool 类型的参数 lifo，用于控制是否将等待该资源的 goroutine 放到等待队列的队首，以及一个 int 类型的 skipframes 参数，表示需要跳过的堆栈帧数，一般可以设置为默认值 1。
			//在 runtime_SemacquireMutex 函数中，会不断尝试获得锁或信号量，如果无法获得，则会陷入休眠等待信号量释放。
			//当前协程可以获得信号量后，从 runtime_SemacquireMutex 函数中返回。
			runtime_SemacquireMutex(&m.sema, queueLifo, 1)
			starving = starving || runtime_nanotime()-waitStartTime > starvationThresholdNs
			old = m.state
			if old&mutexStarving != 0 {
				if old&(mutexLocked|mutexWoken) != 0 || old>>mutexWaiterShift == 0 {
					throw("sync: inconsistent mutex state")
				}
				delta := int32(mutexLocked - 1<<mutexWaiterShift)
				if !starving || old>>mutexWaiterShift == 1 {
					delta -= mutexStarving
				}
				atomic.AddInt32(&m.state, delta)
				break
			}
			awoke = true
			iter = 0
		} else {
			old = m.state
		}
	}
	//race 包是 Golang 中的一个内置包，用于数据竞争检测。当多个 Goroutine 同时访问同一片内存区域，且其中至少有一个 Goroutine 会对该内存区域进行写入操作时，就会产生数据竞争。这种情况下，无法保证程序的正常行为，并且可能会产生意料之外的结果或者崩溃。
	//
	//race 包提供了一组函数，可以在代码中手动标识共享内存区域的读取和写入操作，从而帮助用户找出并修复潜在的数据竞争问题。其主要方法有：
	//
	//Acquire 和 Release：用于标注对互斥锁、读写锁等锁类型的获取和释放操作。
	//Read 和 Write：用于标注共享内存区域的读取和写入操作。
	//Begin 和 End：用于标注一个临界区的开始和结束，临界区内的所有共享内存操作都需要被标记。
	//需要注意的是，使用 race 包进行代码验证可能会对程序性能产生一定的影响，并且在开启了 -race 编译选项后，编译器会插入额外的代码来捕获并检测竞争条件，因此编译后的二进制文件可能会变得更大。
	if race.Enabled {
		reace.Acquire(unsafe.Pointer(m))
	}
}

func (m *Mutex) Unlock() {
	if race.Enabled {
		_ = m.state
		race.Release(unsafe.Pointer(m))
	}
	new := atomic.AddInt32(&m.state, -mutexLocked)
	if new != 0 {
		m.unlockSlow(new)
	}
}

func (m *Mutex) unlockSlow(new int32) {
	if (new+mutexLocked)&mutexLocked == 0 {
		throw("sync: unlock of unlocked mutex")
	}
	if new&mutexStarving == 0 {
		old := new
		for {
			if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
				return
			}
			new = (old - 1<<mutexWaiterShift) | mutexWoken
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				//runtime_Semrelease 还接收两个参数：一个是信号量或互斥锁的地址，另一个是一个 bool 类型的参数 handoff，用于控制唤醒等待队列中的 goroutine 时是否将当前 goroutine 放到等待队列的队首。
				//skipframes 参数表示需要跳过的堆栈帧数，一般可以设置为默认值 1。
				runtime_Semrelease(&m.sema, false, 1)
				return
			}
			old = m.state
		}
	} else {
		runtime_Semrelease(&m.sema, true, 1)
	}
}
