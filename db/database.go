package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() {
	// 确保 data 目录存在
	dataDir := filepath.Join("..", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("无法创建 data 目录:", err)
	}

	dbPath := filepath.Join(dataDir, "suanzi.db")
	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("无法打开数据库:", err)
	}

	// 设置连接池
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(5)

	// 创建表
	createTables()
}

func createTables() {
	sitesTable := `
	CREATE TABLE IF NOT EXISTS sites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	countersTable := `
	CREATE TABLE IF NOT EXISTS counters (
		site_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		path TEXT DEFAULT '',
		count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (site_id, type, path),
		FOREIGN KEY (site_id) REFERENCES sites(id)
	);`

	uvRecordsTable := `
	CREATE TABLE IF NOT EXISTS uv_records (
		site_id INTEGER NOT NULL,
		uuid TEXT NOT NULL,
		date DATE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (site_id, uuid, date),
		FOREIGN KEY (site_id) REFERENCES sites(id)
	);`

	tables := []string{sitesTable, countersTable, uvRecordsTable}
	for _, tableSQL := range tables {
		_, err := DB.Exec(tableSQL)
		if err != nil {
			log.Fatal("创建表失败:", err)
		}
	}

	fmt.Println("数据库表创建成功")
}