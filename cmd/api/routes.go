package main

import (
	"expvar"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
	"net/http"
)

/*
封装路由的好处：
	可以通过初始化application实例并调用routes方法，在任何测试中访问路由
*/

func (app *application) routes() http.Handler {
	//初始化一个httprouter实例
	router := httprouter.New()
	//httprouter找不到匹配路由时，会调用NotFound Handler处理程序自动发送纯文本响应
	//使用HandlerFunc将具有特定签名的函数转变为handler类型
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	//HandlerFunc类型是一个适配器，允许将普通函数用作HTTP处理程序。如果f是一个具有适当签名的函数，HandlerFunction（f）是一个调用f的handler

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthCheckHandler)

	router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission("movies:write", app.createMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission("movies:read", app.showMovieHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission("movies:write", app.updateMovieHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission("movies:write", app.deleteMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))
	//User
	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/password", app.updateUserPasswordHandler)
	//Token
	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/activation", app.createActivateTokenHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/password-reset", app.createPasswordResetTokenHandler)

	router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())
	chain := alice.New(app.metrics, app.recoverPanic, app.enableCORS, app.rateLimit, app.authenticate)
	return chain.Then(router)
}
