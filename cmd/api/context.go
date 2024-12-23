package main

import (
	"Greenlight/internal/data"
	"context"
	"net/http"
)

type contextKey string

// 将字符串"user"转变为contextKey类型，在后面使用这个常量作为设置和获取用户信息的键
const userContextKey = contextKey("user")

// 返回请求的新副本
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func (app *application) contextGetUser(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User) //后面是类型断言，取出来的都是interface类型，需要转变为User数据类型
	if !ok {
		panic("missing user value in request context")
	}
	return user
}
