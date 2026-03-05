package main

import (
	"log"
	"net/http"
	"os"

	"./api"
	"./db"
)

func main() {
	// 创建必要的目录
	os.MkdirAll("../data", 0755)
	os.MkdirAll("../logs", 0755)
	os.MkdirAll("../static", 0755)

	// 初始化数据库
	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 注册路由
	http.HandleFunc("/api/count", api.CountHandler)
	http.HandleFunc("/api/init", api.InitHandler)
	
	// 静态文件服务
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// 介绍页面（后续实现）
			w.Write([]byte("Suanzi Counter Service - 欢迎使用不蒜子复刻版"))
		} else {
			http.NotFound(w, r)
		}
	})

	// 启动 HTTP 服务
	port := "7788"
	log.Printf("Starting Suanzi service on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}