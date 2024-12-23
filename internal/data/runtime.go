package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Runtime int32

//当Go解码一些json时，将检查目标类型是否满足json.Unmarshaler接口,如果满足这个接口，Go将调用他的UnmarshalJSON方法来确定如何将类型解码
//编码时也一样

// ErrInvalidRuntimeFormat 如果不能成功的够解码或者转变JSON字符串，返回此错误
var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

// MarshalJSON Runtime类型实现MarshalJSON方法使其满足json.Marshaler接口
// 使用值接收器，使代码更具灵活性，因为值接受者可以使用指针或者值调用，但指针接受者只能使用指针调用
func (r Runtime) MarshalJSON() ([]byte, error) {
	//以要求的格式生成一个字符串包含movie runtime
	jsonValue := fmt.Sprintf("%d mins", r)
	//对字符串使用strconv.Quote将其包含在双引号中，它需要使用双引号包含以使成为一个json字符串
	quoteJSONValue := strconv.Quote(jsonValue)
	return []byte(quoteJSONValue), nil
}

// UnmarshalJSON 为Runtime类型实现UnmarshalJSON方法，使Go在解码JSON数据时，按照方法定义的格式解码目标类型的数据
// 因为UnmarshalJSON需要更改Runtime类型
func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRuntimeFormat
	}
	parts := strings.Split(unquotedJSONValue, " ")
	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRuntimeFormat
	}
	i, err := strconv.Atoi(parts[0])
	if err != nil {
		return ErrInvalidRuntimeFormat
	}
	*r = Runtime(i)
	return nil
}
