package main

import (
	"Greenlight/internal/data"
	"Greenlight/internal/validator"
	"errors"
	"fmt"
	"golang.org/x/time/rate"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 只会恢复在执行recoverPanic中间件同一进程中发生的panic
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//创建一个defer function  将会在Go遇到panic展开堆栈时运行
		defer func() {
			//使用recover() function检查是否有panic
			if err := recover(); err != nil {
				//如果存在panic, 在响应上设置一个"Connection:close" header
				//它作为一个触发器，在发送完响应时自动关闭http链接
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}

		}()
		next.ServeHTTP(w, r)
	})
}

/*
为每个客户端单独设置一个速率限制器
方法：

	创建一个内存中的速率限制器map，使用每个客户端的IP地址作为映射键。当新客户端发送请求时，将其IP地址作为映射键存储
	并为其添加一个新的速率限制器。对于后续请求，我们从映射中检索该客户端的速率限制器调用allow判断是否允许访问

另外，客户端映射将无限延长，为了防止这样的事情发生，记录客户端最后一次出现的时间，定期删除很长时间没有看到的客户端
*/
func (app *application) rateLimit(next http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)
	go func() {
		//开启一个线程，每隔一分钟扫描客户端映射，删除过长时间没有出现的客户端信息
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for k, v := range clients {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(clients, k)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//只有启用速率限制时才执行检查
		if app.config.limiter.enabled {
			//提取客户端的IP地址
			port, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}
			mu.Lock()
			if _, found := clients[port]; !found {
				clients[port] = &client{limiter: rate.NewLimiter(2, 4)}
			}
			if !clients[port].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}
			mu.Unlock()
			next.ServeHTTP(w, r)
		}

	})
}

// 根据请求的token将对应用户信息存入上下文，没有
func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//在响应头中添加"Vary: Authorization" 这向任何缓存表明，响应会根据请求中的Authentication头的值变化
		w.Header().Set("Vary", "Authorization")
		authenticationHeader := r.Header.Get("Authorization")
		//如果没有token，在上下文中设置空的匿名结构体
		if authenticationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}
		headerPart := strings.Split(authenticationHeader, " ")
		if len(headerPart) != 2 || headerPart[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}
		token := headerPart[1]
		v := validator.New()
		data.ValidateTokenPlainText(v, token)
		if !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}
		//
		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
		//将提取的用户存储在上下文中
		request := app.contextSetUser(r, user)
		next.ServeHTTP(w, request)
	})
}

// 检查用户是否是匿名用户
func (app *application) requireAuthenticationUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		//如果是匿名用户，提醒他们进行身份验证
		if user.IsAnonymousUser() {
			app.authenticationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// 对用户身份的状态进行验证，查看是否激活
func (app *application) requireActivateUser(next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		//如果是实体用户，但是没有激活，提醒用户需要激活
		if !user.Activated {
			app.inactivateAccountResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
	//中间件执行顺序，先执行requireAuthenticationUser,在执行requireActivateUser，先知道User是谁，在查看是否激活
	return app.requireAuthenticationUser(fn)
}
func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		//根据userID获取该user的权限
		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		include := permissions.Include(code)
		if !include {
			app.notPermittedResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
	return app.requireActivateUser(fn)
}
