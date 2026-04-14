package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "/codespace/developers/linnan/claudeProjects/cronicle-next/cronicle.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 查看表结构
	fmt.Println("=== Nodes 表结构 ===")
	rows, err := db.Query("PRAGMA table_info(nodes)")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
		fmt.Printf("%s: %s (nullable: %v)\n", name, ctype, notnull == 0)
	}

	// 查看数据
	fmt.Println("\n=== 节点数据 ===")
	dataRows, err := db.Query("SELECT id, hostname, ip, tags, status, pid FROM nodes")
	if err != nil {
		log.Fatal(err)
	}
	defer dataRows.Close()

	count := 0
	for dataRows.Next() {
		var id, hostname, ip, tags, status string
		var pid sql.NullInt64
		dataRows.Scan(&id, &hostname, &ip, &tags, &status, &pid)
		fmt.Printf("ID: %s, Host: %s, IP: %s, Tags: %s, Status: %s, PID: %v\n",
			id, hostname, ip, tags, status, pid.Int64)
		count++
	}
	fmt.Printf("总共 %d 条记录\n", count)
}
