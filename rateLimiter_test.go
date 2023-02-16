package golangUtil

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSlideWindows(t *testing.T) {
	println(TryAcquire())
}
func BenchmarkTryAcquire(b *testing.B) {
	group := sync.WaitGroup{}
	group.Add(b.N)
	rate := GetRateLimiterMiddleware(WithWindowsSize(200000000 * time.Nanosecond))
	var totalResult int64
	for i := 0; i < b.N; i++ {
		go func() {
			defer group.Done()
			result := rate.TryAcquire()
			if result {
				atomic.AddInt64(&totalResult, 1)
			}
		}()
	}
	group.Wait()
	fmt.Println(totalResult)
}

func TestConcurrencyTryAcquire(t *testing.T) {
	const times = 10000
	group := sync.WaitGroup{}
	group.Add(times)

	var smoothTimes time.Duration = 30 * time.Millisecond
	var resultInt int64
	var cycle int64 = 0
	for i := 0; i < times; i++ {
		go func() {
			defer group.Done()
			time.Sleep(smoothTimes)
			if cycle >= 100 && smoothTimes < 3*time.Second {
				smoothTimes += smoothTimes * 10
				cycle = 0
			} else {
				atomic.AddInt64(&cycle, 1)
			}
			result := TryAcquire()
			if slideLimiter.totalCount > slideLimiter.permitsPerWindows {
				panic(any("slide windows invalid"))
			}
			if result {
				atomic.AddInt64(&resultInt, 1)
			}

		}()
	}
	group.Wait()
	println(resultInt)
	for key, value := range slideLimiter.windows {
		println(key, ":", value)
	}
}
