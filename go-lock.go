package golangUtils

import (
	"context"
	"crypto/sha1"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

func init() {
	singleConfig = new(config)
	singleConfig.cancelTime = defaultCancelTime
	singleConfig.expiresTime = defaultExpiresTime
	singleConfig.maxOffsetTime = defaultMaxOffsetTime
	singleConfig.reties = defaultReties
	nodeID, err := getMachineID()
	if err != nil {
		panic(fmt.Sprintf("init the mutex unique nodeId false:%s", err.Error()))
	}
	singleConfig.nodeID = nodeID
}

const defaultCancelTime = 1 * time.Second

const defaultExpiresTime = 3 * time.Second

const defaultMaxOffsetTime = 10 * time.Millisecond
const defaultReties = 2

type Mutex struct {
	// auto delay
	delayDone chan struct{}
	name      string
	config    *config
	ending    chan error
}

type config struct {
	cancelTime    time.Duration
	maxOffsetTime time.Duration
	expiresTime   time.Duration
	reties        int
	delegate      *redis.Client
	nodeID        string
}

type ConfigOption func(*config)

// WithCancelTime a time for redis conn need spend the max time
func WithCancelTime(cancelTime time.Duration) ConfigOption {
	return func(c *config) {
		c.cancelTime = cancelTime
	}
}

// WithExpiresTime   Duration of lock
func WithExpiresTime(expireTime time.Duration) ConfigOption {
	return func(c *config) {
		c.expiresTime = expireTime
	}
}

// WithMaxOffsetTime be set to priority about the max gradient times for the interval of requesting lock
func WithMaxOffsetTime(maxOffsetTime time.Duration) ConfigOption {
	return func(c *config) {
		c.maxOffsetTime = maxOffsetTime
	}
}

// WithReties be set to priority about  the gradient decreases for the interval of requesting lock,After how many repetitions
func WithReties(reties int) ConfigOption {
	return func(c *config) {
		c.reties = reties
	}
}

// WithStorageClient the must be init
func WithStorageClient(client *redis.Client) ConfigOption {
	return func(c *config) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := client.Ping(ctx).Err()
		if err != nil {
			panic(fmt.Sprintf("connect the redis error:%s", err.Error()))
		}
		c.delegate = client
	}
}

// AssemblyMutex the mutex config init
func AssemblyMutex(options ...ConfigOption) {
	once.Do(func() {
		for _, value := range options {
			value(singleConfig)
		}
	})
}

// must init before use
var (
	singleConfig *config
	once         sync.Once
)

func NewMutex(name string) *Mutex {
	if singleConfig.delegate == nil {
		panic("Execute the AssemblyMutex Method Must include claim WithStorageClient  before initing the lock")
	}
	mutex := new(Mutex)
	mutex.config = singleConfig
	mutex.name = name
	mutex.delayDone = make(chan struct{})
	mutex.ending = make(chan error)
	return mutex
}

func (mutex *Mutex) Lock() {
	timeOffset := mutex.config.maxOffsetTime
	retryTimes := 0
	for {
		ok := mutex.TryLock()
		if ok {
			//delay
			go func() {
				for {
					select {
					case <-mutex.delayDone:
						return
					default:
						//TODO resolve the relay error should do
						mutex.delay()
						// if err != nil {
						// log.Println(err)
						// 	mutex.ending <- err
						// 	return
						// }
						time.Sleep(mutex.config.expiresTime / 5)
					}
				}
			}()
			return
		}
		time.Sleep(timeOffset)
		retryTimes++
		if mutex.config.reties <= retryTimes {
			timeOffset /= 2
			retryTimes = 0
		}
	}
}

func (mutex *Mutex) TryLock() bool {
	ok, err := mutex.acquire()
	if err != nil || !ok {
		return false
	}
	return true
}

func (mutex *Mutex) acquire() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mutex.config.cancelTime)
	defer cancel()
	cmd := mutex.config.delegate.SetNX(ctx, mutex.name, mutex.config.nodeID, mutex.config.expiresTime)
	return cmd.Val(), cmd.Err()
}

// -1 resprent the delay error
const delayScript = `
   if redis.call('GET', KEYS[1]) == ARGV[1] then
          return redis.call('PEXPIRE',KEYS[1],ARGV[2])
   else
	      return -2
   end	  	   
`
const delayCacheKey = "delay"

func (mutex *Mutex) delay() error {
	ctx, cancel := context.WithTimeout(context.TODO(), mutex.config.cancelTime)
	defer cancel()
	if _, ok := cacheHash[delayCacheKey]; !ok {
		cmd := mutex.config.delegate.Eval(ctx, delayScript, []string{mutex.name}, mutex.config.nodeID, strconv.FormatInt(int64(mutex.config.expiresTime.Milliseconds()), 10))
		err := cmd.Err()
		if err != nil {
			return err
		}
		status, err := cmd.Int()
		if err != nil {
			return err
		}
		if status < 0 {
			return fmt.Errorf("%s key delay error:%d", mutex.name, status)
		}
		hash := sha1.Sum([]byte(delayScript))
		cacheHash[delayCacheKey] = fmt.Sprintf("%x", hash[:])
	}
	cmd := mutex.config.delegate.EvalSha(ctx, cacheHash[delayCacheKey], []string{mutex.name}, mutex.config.nodeID, strconv.FormatInt(int64(mutex.config.expiresTime.Milliseconds()), 10))
	err := cmd.Err()
	if err != nil {
		return err
	}
	status, err := cmd.Int()
	if err != nil {
		return err
	}
	if status < 0 {
		return fmt.Errorf("%s key delay error:%d", mutex.name, status)
	}
	return err
}

func (mutex *Mutex) UnLock() {
	if len(mutex.delayDone) > 0 {
		log.Println("delay error for mutex")
		return
	}
	mutex.delayDone <- struct{}{}
	mutex.release()

}

const releaseScript = `
   if redis.call('GET',KEYS[1])==ARGV[1] then
           return redis.call('DEL',KEYS[1])
   end	   
`

var cacheHash = make(map[string]string)

const releaseCacheKey = "release"

// const deleteCacheKey = "delete"

// if the release failed , the system cant loss any resource
func (mutex *Mutex) release() {
	ctx, cancel := context.WithTimeout(context.TODO(), mutex.config.cancelTime)
	defer cancel()
	if _, ok := cacheHash[releaseCacheKey]; !ok {
		mutex.config.delegate.Eval(ctx, releaseScript, []string{mutex.name}, mutex.config.nodeID)
		hash := sha1.Sum([]byte(releaseScript))
		cacheHash[releaseCacheKey] = fmt.Sprintf("%x", hash[:])
	}
	mutex.config.delegate.EvalSha(ctx, cacheHash[releaseCacheKey], []string{mutex.name}, mutex.config.nodeID)
}
func getMachineID() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {

		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				return "", err
			}

			// 查找第一个有效的MAC地址
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					mac := iface.HardwareAddr.String()
					if mac != "" {
						return mac, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("无法获取机器的唯一标识")
}
