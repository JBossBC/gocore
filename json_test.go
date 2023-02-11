package golangUtil

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
)

type testStruct struct {
	name    string
	value   string
	mapping int
	err     error
}

func TestMarshal(t *testing.T) {
	const testTimes int = 1
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
func BenchmarkGolangUtilMarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Marshal(&testStruct{name: "xiyang", value: "xiyangValue", mapping: rand.Int(), err: fmt.Errorf("hello error")})
		if err != nil {
			return
		}
	}
}
func BenchmarkJSONMarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(&testStruct{name: "xiyang", value: "xiyangValue", mapping: rand.Int(), err: fmt.Errorf("hello error")})
		if err != nil {
			return
		}
	}
}
