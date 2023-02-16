package golangUtil

import (
	"sync"
	"sync/atomic"
	"time"
)

const SmallWindows int64 = 1 << 5
const TimeShift = 1e9
const WindowsSize int64 = 3 * TimeShift
const MaxRequestPerWindows = WindowsSize / TimeShift * 1000

type slideWindowsLimiter struct {
	permitsPerWindows    int64
	windows              map[int64]int64
	totalCount           int64
	lock                 sync.Mutex
	once                 sync.Once
	timestamp            int64
	smallWindowsDistance int64
	windowsSize          int64
	clearFlag            int32
	subWindowsSize       int64
	cond                 *sync.Cond
	options              rateLimiterOptions
}
type rateLimiterOptions func(limiter *slideWindowsLimiter)

func WithMaxPassingPerWindows(numbers int64) rateLimiterOptions {
	return func(limiter *slideWindowsLimiter) {
		limiter.permitsPerWindows = numbers
	}
}

// WithSubWindowsNumber function represent  you can set the sub windows number to control better the  flow overflow in a very short time,but this operation can increase your time which handler the outdated windows
func WithSubWindowsNumber(numbers int64) rateLimiterOptions {
	return func(limiter *slideWindowsLimiter) {
		limiter.subWindowsSize = numbers
		limiter.smallWindowsDistance = limiter.windowsSize / limiter.subWindowsSize
	}
}

// WithWindowsSize function represent you can set windows size to promise the actual max Request numbers can‘t exceed the max request which you set in the period of time
func WithWindowsSize(times time.Duration) rateLimiterOptions {
	return func(limiter *slideWindowsLimiter) {
		limiter.windowsSize = times.Nanoseconds()
		limiter.smallWindowsDistance = limiter.windowsSize / limiter.subWindowsSize
	}
}
func init() {
	slideLimiter = initDefaultLimiter()
	deferCreateWindows(slideLimiter)
}

func GetRateLimiterMiddleware(options ...rateLimiterOptions) (result *slideWindowsLimiter) {
	result = initDefaultLimiter()
	for i := 0; i < len(options); i++ {
		options[i](result)
	}
	deferCreateWindows(result)
	return result
}
func deferCreateWindows(limiter *slideWindowsLimiter) {
	limiter.once.Do(func() {
		var i int64 = 0
		for ; i < limiter.subWindowsSize; i++ {
			limiter.windows[i] = 0
		}
	})
}

func initDefaultLimiter() (result *slideWindowsLimiter) {
	result = &slideWindowsLimiter{
		permitsPerWindows: MaxRequestPerWindows,
		// windows length is  prime number may be can defeat conflict better
		windows:              make(map[int64]int64, SmallWindows+3),
		timestamp:            time.Now().UnixNano(),
		lock:                 sync.Mutex{},
		smallWindowsDistance: WindowsSize / SmallWindows,
		windowsSize:          WindowsSize,
		subWindowsSize:       SmallWindows,
	}
	result.cond = sync.NewCond(&result.lock)
	return result
}

var slideLimiter *slideWindowsLimiter

func TryAcquire() bool {
	return slideLimiter.TryAcquire()
}
func (s *slideWindowsLimiter) TryAcquire() bool {
	var diff int64
	s.lock.Lock()
	for atomic.LoadInt32(&s.clearFlag) != 0 {
		s.cond.Wait()
	}
	diff = time.Now().UnixNano() - s.timestamp
	var index = diff / s.smallWindowsDistance
	if diff <= s.windowsSize {
		if s.totalCount < s.permitsPerWindows {
			s.totalCount++
			s.windows[index]++
			s.lock.Unlock()
			return true
		} else {
			s.lock.Unlock()
			return false
		}
	} else {
		if atomic.CompareAndSwapInt32(&s.clearFlag, 0, 1) {
			go func() {
				s.lock.Lock()
				defer s.lock.Unlock()
				s.timestamp += diff
				var i int64 = 0
				var invalidWindows = index % s.subWindowsSize
				for ; i <= invalidWindows; i++ {
					s.totalCount -= s.windows[i]
					s.windows[i] = 0
				}
				atomic.CompareAndSwapInt32(&s.clearFlag, 1, 0)
				s.cond.Broadcast()
			}()
		}
		s.lock.Unlock()
	}
	return s.TryAcquire()
}
