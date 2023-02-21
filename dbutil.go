package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql" // init()
)

// dbpool 定义全局的数据库连接池

var dbPool *sql.DB

func initDB(dbAddr string) (dbPool *sql.DB, err error) {
	//根据数据库驱动定义数据库类型
	dbPool, err = sql.Open("mysql", dbAddr)
	if err != nil {
		return
	}
	err = dbPool.Ping() //测试链接数据库
	if err != nil {
		return
	}
	dbPool.SetMaxIdleConns(500)
	return
}
