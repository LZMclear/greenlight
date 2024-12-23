package main

import (
	"Greenlight/internal/data"
	"Greenlight/internal/jsonlog"
	"Greenlight/internal/mailer"
	"context"
	"database/sql"
	"flag"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"sync"
	"time"
)

const version = "1.0.0"

type config struct { //保存所有配置设置
	port int // 端口号
	env  string
	db   struct {
		dsn         string
		maxOpenConn int    //最大连接数
		maxIdleConn int    //最大空闲连接数：小于或等于maxOpenConn
		maxIdleTime string //最大空闲时间
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
}

// 定义application struct为处理程序，中间件，帮助程序保存所有的依赖项
type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup // 不需要初始化
}

func main() {
	//声明一个配置实例
	var cfg config
	flag.StringVar(&cfg.env, "env", "development", "开发|测试|生产")
	flag.IntVar(&cfg.port, "port", 4000, "端口号")

	//数据库命令行
	flag.StringVar(&cfg.db.dsn, "dsn", "root:251210@tcp(127.0.0.1:3306)/greenlight?charset=utf8mb4&parseTime=true", "连接字符串")
	flag.IntVar(&cfg.db.maxOpenConn, "maxOpenConn", 25, "sql max openConn number")
	flag.IntVar(&cfg.db.maxIdleConn, "maxIdleConn", 25, "sql maxConn free number")
	flag.StringVar(&cfg.db.maxIdleTime, "maxIdleTime", "15m", "conn max free time")
	//速率限制器配置
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	//smtp配置
	flag.StringVar(&cfg.smtp.host, "smtp-host", "localhost", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
	//  服务器和密码用来登录smtp服务器的，这里用的本地的fakeSMTP服务器，所以填不填无所谓
	flag.StringVar(&cfg.smtp.username, "smtp-username", "***", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "***", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Greenlight <no-reply@greenlight.alexedwards.net>", "SMTP sender")

	flag.Parse()
	//声明一个依赖项的实例
	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)
	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.Close()
	logger.PrintInfo("database connect pool established", nil)
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}

}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.db.dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.db.maxOpenConn) //设置最大连接数
	db.SetMaxIdleConns(cfg.db.maxIdleConn) // 设置最大空闲连接数
	//使用ParseDuration将空闲超时字符串转换为时间戳类型
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//使用Ping函数查看是否连接成功
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}
