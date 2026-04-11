package main

import (
	"context"
	"fmt"
	"net/http"
	"nexai-backend/cmd/server/bootstrap"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

func main() {
	bootstrap.InitViper()
	bootstrap.InitValidate()
	tpCancel := bootstrap.InitOTEL()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		tpCancel(ctx)
	}()

	app := InitApp()

	// 初始化 HTTP Server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: app.engine,
	}

	// 1. 在 goroutine 中启动服务器
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 只有非关闭引起的错误才 panic
			panic(err)
		}
	}()

	// 2. 监听中断信号 (Ctrl+C, kill 等)
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞直到接收到信号
	<-quit
	fmt.Println("Shutting down server...")

	// 3. 创建一个 5 秒超时的 Context，给正在处理的请求一点时间收尾
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Server forced to shutdown:", err.Error())
	}

	fmt.Println("Server exiting")
}
