package main

import (
	"log"
	"net/http"
	"os"

	"suanzi/api"
	"suanzi/db"
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
	http.HandleFunc("/api/count", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			api.QueryHandler(w, r)
		} else if r.Method == "POST" {
			api.CountHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/init", api.InitHandler)
	
	// 静态文件服务（使用绝对路径）
	staticDir := "/data/suanzi/src/static"
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(staticDir))))
	
	// 根路径返回介绍页面
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, staticDir+"/index.html")
		} else {
			http.NotFound(w, r)
		}
	})

	// 启动 HTTP 服务
	port := "7788"
	log.Printf("Starting Suanzi service on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}