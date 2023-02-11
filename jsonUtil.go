package golangUtil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

const MaxStorageBytes int64 = 1024
const MaxTolerateCacheFailedRate = 0.4

type nocopy uintptr
type jsonEncoder struct {
	cache             map[uintptr]*[]byte
	obsolete          heap
	cacheAccessTimes  map[uintptr]int64
	capacity          int64
	nocopy            nocopy
	rw                sync.RWMutex
	cacheInValidTimes int
	//the field to order the cache invalid info
	cacheSuccessNums int64
	cacheNums        int64
}

func init() {
	JsonEncoder = &jsonEncoder{
		cache:    make(map[uintptr]*[]byte),
		capacity: 0,
		obsolete: heap{
			heapArr: make([]int, 100),
			value:   make([]uintptr, 100),
			length:  0,
		},
		rw:               sync.RWMutex{},
		cacheAccessTimes: make(map[uintptr]int64),
	}
}

var (
	JsonEncoder *jsonEncoder
)

func (e *jsonEncoder) store(address uintptr, data *[]byte) {
	e.check()
	e.rw.Lock()
	defer e.rw.Unlock()
	if _, ok := e.cache[address]; ok {
		return
	}
	var cacheSuccessNums = e.cacheSuccessNums
	var cacheNums = e.cacheNums
	if e.capacity+int64(len(*data)) > MaxStorageBytes {
		if int64(e.obsolete.peekMax())+MaxStorageBytes-e.capacity < int64(len(*data)) && e.cacheInValidTimes < 10 {
			e.cacheInValidTimes++
			return
		} else if e.cacheInValidTimes >= 10 {
			value := e.obsolete.peekMax()
			if int64(len(*e.cache[value])) > MaxStorageBytes/4 {
				delete(e.cache, value)
				e.obsolete.delete(e.obsolete.length)
				delete(e.cacheAccessTimes, value)
			} else if float64(cacheSuccessNums/cacheNums) < MaxTolerateCacheFailedRate {
				var sum = int64(e.obsolete.length)
				for i := 0; i > e.obsolete.length; i++ {
					var temp = e.obsolete.value[e.obsolete.length]
					//can't give up the hot data
					if e.cacheAccessTimes[temp] > cacheSuccessNums/sum*2 {
						continue
					}
					delete(e.cache, temp)
					delete(e.cacheAccessTimes, temp)
					e.obsolete.delete(i)
				}
				e.cacheNums = 0
				e.cacheSuccessNums = 0
			}
			e.cacheInValidTimes = 0
		} else if int64(e.obsolete.peekMax())+MaxStorageBytes-e.capacity >= int64(len(*data)) {
			if e.cacheAccessTimes[e.obsolete.peekMax()] <= cacheSuccessNums/int64(len(e.cache))*2 {
				var temp = e.obsolete.peekMax()
				delete(e.cache, temp)
				delete(e.cacheAccessTimes, temp)
				e.obsolete.delete(1)
			}
		}
	}
	atomic.AddInt64(&e.capacity, int64(len(*data)))
	e.obsolete.insert(len(*data), address)
	e.cacheAccessTimes[address] = 0
	e.cache[address] = data
}
func (e *jsonEncoder) load(address uintptr) (*[]byte, bool) {
	e.check()
	e.rw.RLock()
	defer e.rw.RUnlock()
	atomic.AddInt64(&e.cacheNums, 1)
	value, ok := e.cache[address]
	if ok {
		times := e.cacheAccessTimes[address]
		atomic.AddInt64(&times, 1)
		atomic.AddInt64(&e.cacheSuccessNums, 1)
	}
	return value, ok
}
func (e *jsonEncoder) check() {
	if uintptr(e.nocopy) != uintptr(unsafe.Pointer(e)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(&e.nocopy), 0, uintptr(unsafe.Pointer(e))) &&
		uintptr(e.nocopy) != uintptr(unsafe.Pointer(e)) {
		panic(any("object has copyed"))
	}
}

func Marshal(value any) ([]byte, error) {
	var address uintptr
	var load *[]byte
	var ok bool
	if value, ok := value.(unsafe.Pointer); ok {
		address = uintptr(value)
	} else {
		address = uintptr(unsafe.Pointer(&value))
	}
	load, ok = JsonEncoder.load(address)
	if ok {
		return *load, nil
	}
	sb := strings.Builder{}
	bytes, err := marshal(reflect.ValueOf(value))
	sb.WriteString("{")
	sb.WriteString(string(bytes))
	sb.WriteString("}")
	bytes = []byte(sb.String())
	if err != nil {
		return nil, err
	}
	JsonEncoder.store(address, &bytes)
	return bytes, nil
}

func marshal(value reflect.Value) (result []byte, err error) {
	defer func() {
		if panicErr := recover(); panicErr != any(nil) {
			result = nil
			fmt.Println(panicErr)
			err = fmt.Errorf("params analy error")
		}
	}()
	result = make([]byte, 0, value.Type().Size()*2)
	switch value.Kind() {
	case reflect.String:
		result = append(result, []byte(value.String())...)
	case reflect.Int, reflect.Int64, reflect.Int8, reflect.Int16, reflect.Int32:
		result = append(result, []byte(strconv.FormatInt(value.Int(), 10))...)
	case reflect.Slice:
		var tempArr = make([]byte, 0, value.Len()*value.Type().Elem().Align())
		for i := 0; i < value.Len(); i++ {
			temp, err := marshal(value.Index(i))
			if err != nil {
				return nil, err
			}
			if i != value.Len()-1 {
				temp = append(temp, ',')
			}
			tempArr = append(tempArr, temp...)
		}
		result = append(result, combineToJson(value.Type().Name(), string(tempArr), value.Kind())...)
	case reflect.Struct:
		var number = value.NumField()
		var temp = make([]byte, 0, value.Type().Size())
		for i := 0; i < number; i++ {
			var fieldValue = value.Field(i)
			fieldName := value.Type().Field(i).Name
			bytes, err := marshal(fieldValue)
			if err != nil {
				return nil, err
			}
			fieldType := fieldValue.Kind()
			if fieldType == reflect.Struct {
				temp = append(temp, bytes...)
				if i != number-1 {
					temp = append(temp, ',')
				}
				continue
			}
			if fieldType == reflect.Map {
				fieldType = reflect.Int
			}
			temp = append(temp, combineToJson(fieldName, string(bytes), fieldType)...)
			if i != number-1 {
				temp = append(temp, ',')
			}
		}
		result = append(result, combineToJson(value.Type().Name(), string(temp), value.Kind())...)
	case reflect.Bool:
		result = append(result, strconv.FormatBool(value.Bool())...)
	case reflect.Pointer, reflect.Interface, reflect.Uintptr:
		elem := value.Elem()
		temp, err := marshal(elem)
		if err != nil {
			return nil, err
		}
		result = append(result, temp...)
	case reflect.Float32, reflect.Float64:
		result = append(result, strconv.FormatFloat(value.Float(), 'E', -1, 32)...)
	case reflect.Complex64, reflect.Complex128:
		result = append(result, strconv.FormatComplex(value.Complex(), 'E', -1, 32)...)
	//maybe can improve
	case reflect.Map:
		keys := value.MapKeys()
		if len(keys) <= 0 {
			return
		}
		var tempArr = make([]byte, 0, 2*len(keys)*(keys[0].Type().Align())*value.MapIndex(keys[0]).Type().Align())
		for i := 0; i < len(keys); i++ {
			var keyValue = value.MapIndex(keys[i])
			keyBytes, err := marshal(keys[i])
			if err != nil {
				return nil, err
			}
			valueBytes, err := marshal(keyValue)
			if err != nil {
				return nil, err
			}
			tempArr = append(tempArr, combineToJson(string(keyBytes), string(valueBytes), reflect.String)...)
		}
		result = append(result, combineToJson("", string(tempArr), reflect.Map)...)
	}
	return result, nil
}

func combineToJson(key string, value string, valueType reflect.Kind) []byte {
	sb := strings.Builder{}
	sb.WriteString("\"")
	sb.WriteString(key)
	sb.WriteString("\": ")
	switch valueType {
	case reflect.String:
		sb.WriteString("\"")
		sb.WriteString(value)
		sb.WriteString("\"")
	case reflect.Int, reflect.Int64, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Bool:
		sb.WriteString(value)
	case reflect.Slice, reflect.Array:
		sb.WriteString("[")
		sb.WriteString(value)
		sb.WriteString("]")
	case reflect.Struct, reflect.Interface:
		sb.WriteString("{")
		sb.WriteString(value)
		sb.WriteString("}")
	case reflect.Map:
		sb = strings.Builder{}
		sb.WriteString("{")
		sb.WriteString(value)
		sb.WriteString("}")
	}
	sb.WriteString("\n")
	return []byte(sb.String())
}
