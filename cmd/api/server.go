package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	srv := http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		//服务器可以编写自己的日志消息,实现Write方法变成io.Write接口可以将自定义日志传递进去
		//ErrorLog: log.New(logger, "", 0),
	}

	shutdownErr := make(chan error)
	//开启一个后台进程 捕获关闭信号，关闭所有连接和后台进程，实现优雅的关机
	go func() {
		//创建一个通道保存os.Signal值
		/*
			在这里使用缓冲通道，如果使用非缓冲通道，quit 通道 在信号发送的时刻没有准备去接收，会错过信号
		*/
		quit := make(chan os.Signal, 1)
		//使用signal.Notify监听SIGINT and SIGTERM信号并将他们传入quit通道。其他信号不会被捕获并保留他们默认的行为。
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		//从通道中读取值，这个操作将被阻塞直到通道接收一个值
		s := <-quit
		app.logger.PrintInfo("shutting down server", map[string]string{
			"signal": s.String(),
		})
		//创建一个5秒的上下文
		ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelFunc()
		//服务调用shutdown，如果成功关闭将返回nil，或者返回错误（当关闭一个监听器时遇到问题，或者在到达截止日期时没有完成）
		/*
			工作原理是首先关闭所有打开的侦听器，然后关闭所有空闲连接，然后无限期地等待连接返回空闲状态，然后关闭。
		*/

		/*
			1. 关闭连接时没有返回错误，说明连接全部关闭，但是还有后台进程，告诉正在执行关闭后台进程，需要等待wg归零。
			2. 关闭连接出现错误。将err存入通道，放行主程序
		*/

		//shutdown返回的是一个error类型的变量
		err := srv.Shutdown(ctx)
		//通道中存入错误，主程序取消阻塞，继续执行
		if err != nil {
			shutdownErr <- err
		}
		//正在完成后台进程
		app.logger.PrintInfo("completing background tasks", map[string]string{
			"addr": srv.Addr,
		})
		app.wg.Wait()
		//后台进程全部关闭后，放行主程序
		shutdownErr <- nil
	}()

	app.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})
	//调用shutdown会造成ListenAndServer立刻返回一个http.ErrServerClosed错误。如果是这个错误，表明我们的程序优雅的关闭了
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	//阻塞  关闭后才能继续执行
	err = <-shutdownErr
	if err != nil {
		return err
	}
	app.logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})
	return nil
}
