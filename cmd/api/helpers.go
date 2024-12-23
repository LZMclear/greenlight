package main

import (
	"Greenlight/internal/validator"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (app *application) readIDParam(r *http.Request) (int, error) {
	//当httprouter解析一个请求时，会将URL参数存储在请求上下文中。
	params := httprouter.ParamsFromContext(r.Context()) //提取参数的名称和值到一个切片中
	id, err := strconv.Atoi(params.ByName("id"))
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

func (app *application) writeJSON(w http.ResponseWriter, status int, header http.Header, data envelope) error {
	//将空白添加到json编码
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}
	//遍历header map，向 http.ResponseWriter header map添加每个Header，Go在遍历一个空的map不会抛出error
	for key, value := range header {
		w.Header()[key] = value
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

/*
	注意：
		1. json.Decoder正常情况解码时，如果客户端的json数据包含Go对象没有的字段，解码时会自动忽略。
		在解码前调用DisallowUnknownFields()，这意味着如果从客户端发送的json数据中包含没有Go对象映射的字段时，调用Decoder将会返回错误，而不是忽略字段
		2. 如果json数据中没有Go对象中的某个字段时，会默认赋其零值。
*/

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	//没有上限意味着对任意希望对我们的 API 执行拒绝服务攻击的恶意客户端是一个很好的攻击对象
	//限制请求体的大小为1MB
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	//初始化json.Decoder 在解码前调用DisallowUnknownFields()，这意味着如果从客户端发送的json数据中包含没有Go对象映射的字段时，
	//调用Decoder将会返回错误，而不是忽略字段
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&dst)
	if err != nil {
		//判断错误类型
		var syntaxError *json.SyntaxError                  //语法错误
		var unmarshalTypeError *json.UnmarshalTypeError    //json值不适合目标Go对象
		var invalidUnmarshalError *json.UnmarshalTypeError //解码目标无效（通常是因为传入的Decoder不是指针类型）
		switch {
		//使用errors.As检查err是否有*json.SyntaxError类型，如果有，返回一个简单的英文错误消息，其中包含了问题的位置
		case errors.As(err, &syntaxError):
			fmt.Sprintf("body contains badly-formed JSON (at character %d)", syntaxError.Offset) //
		//有些情况Decoder可能返回一个io.ErrUnexpectEOF错误
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		// 这种情况通常发生在json值对Go对象来说是错误的类型
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		//如果json数据包含没有Go对象映射的字段，Decoder会返回一个错误信息"json: unknown field "<name>""
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)
		//请求体为空
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		//向Decoder传递一个非空的指针
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}
	//再次调用Decoder函数，传递一个匿名结构体的空指针类型，如果request body只包含一个json数据，则会返回io.EOF错误
	//如果返回其他错误，则说明还有多余的数据
	err = dec.Decode(&struct {
	}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

// 定义一个封装类型，用来封装API响应
type envelope map[string]interface{}

func (app *application) readString(values url.Values, key string, defaultValue string) string {
	value := values.Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func (app *application) readCSV(values url.Values, key string, defaultValue []string) []string {
	csv := values.Get(key)
	if csv == "" {
		return defaultValue
	}
	return strings.Split(csv, ",")
}
func (app *application) readInt(values url.Values, key string, defaultValue int, v *validator.Validator) int {
	value := values.Get(key)
	if value == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		v.AddError(key, "must be a integer value")
	}
	return i
}

func (app *application) background(fn func()) {
	//开启一个后台进程
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		//使用延迟函数捕获可能出现的panic
		defer func() {
			if err := recover(); err != nil {
				app.logger.PrintError(fmt.Errorf("%s", err), nil)
			}
		}()
		fn()
	}()
}
