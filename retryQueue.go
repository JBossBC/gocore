package golangUtil


import (
"container/heap"
"sync"
"sync/atomic"
"time"
"unsafe"
)

const HighPriority uint8 = 255

const MiddlerPriority uint8 = 128

const LowPriority uint8 = 0

var queue *retryQueue

const defaultDelayTimePerTimes = 200 * time.Millisecond

//auto begin the retryQueue
//func init() {
//	queue = NewRetryQuery(defaultDelayTimePerTimes)
//	go queue.Run()
//}
//
//func AddTask(function func() error, priority uint8) {
//	queue.AddTask(function, priority)
//}

const defaultConcurrencyNumber = 5
const defaultMaxRetryTimesPerTime = 3
const defaultMaxSleepTime = 3 * time.Second

type retryQueue struct {
	//arena atomic operation
	rw                   sync.RWMutex
	arena                taskArena
	signal               chan int
	work                 []worker
	nocopy               nocopy
	maxRetryTimesPerTime int32
	workerNumber         int32
	maxSleepTime         time.Duration
	//let query sleep
	remained chan struct{}
	loaf     bool
}
type Option func(retryQuery *retryQueue)

func WithConcurrencyNumber(number int32) Option {
	return func(retryQuery *retryQueue) {
		retryQuery.workerNumber = number
	}
}
func WithMaxSleepTime(time time.Duration) Option {
	return func(retryQuery *retryQueue) {
		retryQuery.maxSleepTime = time
	}
}
func WithMaxRetryTimesPerTime(number int32) Option {
	return func(retryQuery *retryQueue) {
		retryQuery.maxRetryTimesPerTime = number
	}
}

func NewRetryQuery(retryDelay time.Duration, options ...Option) *retryQueue {
	res := &retryQueue{
		workerNumber:         defaultConcurrencyNumber,
		maxRetryTimesPerTime: defaultMaxRetryTimesPerTime,
		arena:                make(taskArena, 0),
		rw:                   sync.RWMutex{},
		remained:             make(chan struct{}, 1),
		maxSleepTime:         defaultMaxSleepTime,
		loaf:                 true,
	}
	res.check()
	for i := 0; i < len(options); i++ {
		options[i](res)
	}
	res.signal = make(chan int, res.workerNumber)
	res.work = make([]worker, res.workerNumber)
	for i := 0; i < len(res.work); i++ {
		res.work[i].retryDelay = retryDelay
		res.work[i].query = res
		res.work[i].retryTimes = res.maxRetryTimesPerTime
		res.work[i].index = int32(i)
		res.signal <- i
	}
	return res
}

type worker struct {
	index      int32
	query      *retryQueue
	task       *task
	retryTimes int32
	retryDelay time.Duration
}

func (worker *worker) addTask(task *task) {
	worker.task = task
}

func (work *worker) work() {
	// request work
	var curRetryTimes = 0
	if work.task == nil {
		panic("concurrent RetryQuery panic")
	}
	for {
		err := work.task.exec()
		if err == nil {
			break
		} else {
			if curRetryTimes+1 >= int(work.retryTimes) {
				work.query.rw.Lock()
				heap.Push(&work.query.arena, work.task)
				work.query.rw.Unlock()
				break
			}
		}
		time.Sleep(work.retryDelay)
		curRetryTimes++
	}
	work.query.signal <- int(work.index)
}

type taskArena []*task

type task struct {
	exec     func() error
	priority uint8
}

func (query *retryQueue) check() {
	if !atomic.CompareAndSwapUintptr((*uintptr)(&query.nocopy), uintptr(0), uintptr(unsafe.Pointer(query))) && unsafe.Pointer(query.nocopy) != unsafe.Pointer(query) {
		panic("task copy")
	}
}

type nocopy uintptr

func (arena taskArena) Less(i, j int) bool {
	var iPriority = arena[i]
	var jPriority = arena[j]
	var iValue int
	var jValue int
	if iPriority != nil {
		iValue = int(iPriority.priority)
	}
	if jPriority != nil {
		jValue = int(jPriority.priority)
	}
	return iValue >= jValue
}
func (arena taskArena) Len() int {
	return len(arena)
}
func (arena taskArena) Swap(i, j int) {
	arena[i], arena[j] = arena[j], arena[i]
}

func (arena *taskArena) Push(value interface{}) {
	*arena = append(*arena, value.(*task))
}

func (arena *taskArena) Pop() interface{} {
	old := *arena
	n := len(old)
	x := old[n-1]
	*arena = old[0 : n-1]
	return x
}
func (query *retryQueue) AddTask(function func() error, priority uint8) {
	query.check()
	query.rw.Lock()
	defer query.rw.Unlock()
	task := task{
		exec:     function,
		priority: priority,
	}
	heap.Push(&query.arena, &task)
	query.loaf = false
}

func (query *retryQueue) Run() {
	var baseWaitTime = 1 * time.Millisecond
	var nowWaitTime = baseWaitTime
	var times = 1
	for {
		select {
		case <-query.remained:
			if query.loaf {
				nowWaitTime = baseWaitTime * (1 << times)
				if nowWaitTime > query.maxSleepTime {
					nowWaitTime = query.maxSleepTime
				}
				times++
				query.remained <- struct{}{}
			} else {
				nowWaitTime = baseWaitTime
				times = 1
			}
			time.Sleep(nowWaitTime)
		case workIndex := <-query.signal:
			query.rw.Lock()
			var cur *task
			flag := len(query.arena) <= 0
			if !flag {
				cur = query.arena.Pop().(*task)
			}
			query.rw.Unlock()
			if flag {
				query.loaf = true
				if len(query.remained) == 0 {
					query.remained <- struct{}{}
				}
				//return the idle work
				query.signal <- workIndex
				continue
			} else {
				query.work[workIndex].addTask(cur)
				go query.work[workIndex].work()
			}

		}
	}
}

