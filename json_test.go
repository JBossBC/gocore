package golangUtil

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type testStruct struct {
	name    string
	value   string
	mapping int
	err     error
}

func TestMarshal(t *testing.T) {
	const testTimes int = 100
	for i := 0; i <= testTimes; i++ {
		var temp = testStruct{name: "xiyang", value: "xiyangValue", mapping: rand.Int(), err: fmt.Errorf("hello error")}
		bytes, err := Marshal(temp)
		if err != nil {
			return
		}
		fmt.Println(string(bytes))
		if !json.Valid(bytes) {
			fmt.Println("error .....")
			return
		}
	}
	fmt.Println(JsonEncoder.cacheSuccessNums)

}
func TestGolangUtilMarshal(t *testing.T) {
	const testTimes int = 10000
	group := sync.WaitGroup{}
	group.Add(testTimes)
	now := time.Now()
	for i := 0; i < testTimes; i++ {
		go func() {
			defer group.Done()
			_, err := Marshal(&testStruct{name: "xiyanggou", value: "xiyangValuesagou", mapping: rand.Int(), err: fmt.Errorf("hello error")})
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}()
	}
	group.Wait()
	since := time.Since(now)
	fmt.Println(since)
	fmt.Println(JsonEncoder.cacheSuccessNums)
	fmt.Println(len(JsonEncoder.cache))
}
func TestJsonMarshal(t *testing.T) {
	const testTimes int = 10000
	group := sync.WaitGroup{}
	group.Add(testTimes)
	now := time.Now()
	for i := 0; i < testTimes; i++ {
		go func() {
			defer group.Done()
			_, err := json.Marshal(&testStruct{name: "xiyang", value: "xiyangValue", mapping: rand.Int(), err: fmt.Errorf("hello error")})
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}()
	}
	group.Wait()
	since := time.Since(now)
	fmt.Println(since)

}
func BenchmarkGolangUtilMarshal(b *testing.B) {

	for i := 0; i < b.N; i++ {
		_, err := Marshal(&testStruct{name: "xiyang", value: "xiyangValue", mapping: rand.Int() * 20, err: fmt.Errorf("hello error")})
		if err != nil {
			fmt.Println(err.Error())
			return
		}

	}
}
func BenchmarkJSONMarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(&testStruct{name: "xiyang", value: "xiyangValue", mapping: rand.Int() * 20, err: fmt.Errorf("hello error")})
		if err != nil {
			return
		}
	}
}
