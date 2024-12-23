package validator

import "regexp"

// EmailRX 声明一个正则表达式用于检测邮件格式是否正确
var EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// Validator 定义一个Validator类型用于存储验证错误
type Validator struct {
	Errors map[string]string
}

// New 创建一个Validator实例
func New() *Validator {
	return &Validator{Errors: map[string]string{}}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError 向map中添加一个错误信息
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// Check adds an error message to the map only if a validation check is not 'ok'.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// In 如果一个指定的值存在于一个列表中返回真
func In(value string, list ...string) bool {
	for i := range list {
		if value == list[i] {
			return true
		}
	}
	return false
}

// Matches 如果一个字符串的值和一个指定的regexp 模式匹配返回true
func Matches(values string, rx *regexp.Regexp) bool {
	return rx.MatchString(values)
}

// Unique returns true if all string values in a slice are unique.
// 通过创建一个map，遍历值作为键存进去，如果遇到相同的值会存储在一个键中，出现重复的值时会导致长度不一致
func Unique(values []string) bool {
	uniqueValues := make(map[string]bool)
	for _, value := range values {
		uniqueValues[value] = true
	}
	return len(values) == len(uniqueValues)
}
