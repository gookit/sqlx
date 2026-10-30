# sqlx

[![Go Reference](https://pkg.go.dev/badge/github.com/gookit/sqlx.svg)](https://pkg.go.dev/github.com/gookit/sqlx)
[![Go version](https://img.shields.io/github/go-mod/go-version/gookit/sqlx?style=flat-square)](go.mod)
[![Actions Status](https://github.com/gookit/sqlx/actions/workflows/go.yml/badge.svg)](https://github.com/gookit/sqlx/actions)
[![License](https://img.shields.io/badge/license-MIT-red.svg)](LICENSE)

`sqlx` 在 Go 标准库 `database/sql` 之上提供结构体扫描与命名参数。它是
[jmoiron/sqlx](https://github.com/jmoiron/sqlx) 的维护分支（MIT 许可，原始版权保留在
[LICENSE](LICENSE) 中），导入路径为 `github.com/gookit/sqlx`。

> [English](README.md)

`sqlx` 的 `sql.DB`、`sql.Tx`、`sql.Stmt`、`sql.Conn` 包装类型不改变底层接口，是标准库接口的
超集，已有的 `database/sql` 代码可以照常使用。

## 特性

- 把一行扫描进结构体（支持内嵌结构体）、map 或切片
- 结构体 / map 的命名参数，以及切片的批量插入
- `Get`、`Select`、`StructScan`、`SliceScan`、`MapScan` 等便捷方法
- `*sqlx.DB`、`*sqlx.Tx`、`*sqlx.Conn` 都提供对应方法，并有 `...Context` 版本
- 自定义字段映射（`NameMapper`、`db` tag），以及忽略多余列的 unsafe 模式
- 测试套件不依赖 cgo：使用纯 Go 的 `modernc.org/sqlite` 驱动

## 安装

```shell
go get github.com/gookit/sqlx
```

## 快速开始

```go
package main

import (
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gookit/sqlx"
)

type Person struct {
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
	Email     string
}

func main() {
	// Connect 会 ping 数据库；需要 sql.Open 的语义请用 sqlx.Open
	db, err := sqlx.Connect("mysql", "user:pass@tcp(127.0.0.1:3306)/db?parseTime=true")
	if err != nil {
		log.Fatal(err) // 错误消息里的密码会被打码
	}
	defer db.Close()

	db.MustExec("CREATE TABLE IF NOT EXISTS person (first_name text, last_name text, email text)")

	// 事务 + 命名参数
	tx := db.MustBegin()
	tx.NamedExec("INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)",
		&Person{"Jason", "Moiron", "jmoiron@jmoiron.net"})
	tx.Commit()

	// 多行结果扫描进切片
	people := []Person{}
	if err := db.Select(&people, "SELECT * FROM person ORDER BY first_name"); err != nil {
		log.Fatal(err)
	}

	// 单行结果扫描进结构体；无结果时返回 sql.ErrNoRows
	jason := Person{}
	if err := db.Get(&jason, "SELECT * FROM person WHERE first_name = ?", "Jason"); err != nil {
		log.Fatal(err)
	}

	// 批量插入：结构体切片或 map 切片
	_, err = db.NamedExec(
		"INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)",
		[]Person{
			{"Ardie", "Savea", "asavea@ab.co.nz"},
			{"Ngani", "Laumape", "nlaumape@ab.co.nz"},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	// 逐行扫描进可复用的结构体
	rows, err := db.Queryx("SELECT * FROM person")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		p := Person{}
		if err := rows.StructScan(&p); err != nil {
			log.Fatal(err)
		}
	}
}
```

## 驱动与占位符类型

| 占位符类型 | 驱动名 |
| --- | --- |
| `QUESTION` (`?`) | `mysql`、`sqlite`、`sqlite3`、`nrmysql`、`nrsqlite3` |
| `DOLLAR` (`$1`) | `postgres`、`pgx`、`pgx/v4`、`pgx/v5`、`pq-timeouts`、`cloudsqlpostgres`、`ql`、`nrpostgres`、`cockroach` |
| `NAMED` (`:arg1`) | `oci8`、`ora`、`goracle`、`godror` |
| `AT` (`@p1`) | `sqlserver`、`azuresql` |

- `sqlx.BindType(driverName)` 取驱动的占位符类型。
- `sqlx.BindDriver(driverName, bindType)` 注册上表之外的驱动。
- `db.Rebind(query)` 把 `?` 查询转换成该驱动的占位符。
- `sqlx.In(query, args...)` 把切片展开成对应数量的 `?` 占位符。它**始终只认 `?`**：
  PostgreSQL 下请写 `IN (?)`，再对结果调用 `Rebind`。

## 与上游的差异

- 模块路径为 `github.com/gookit/sqlx`，API 与 `jmoiron/sqlx` 保持兼容。
- 纯 Go 的 `modernc.org/sqlite` 驱动（驱动名 `sqlite`）映射到 `QUESTION`，并用于测试
  套件，因此测试不需要 cgo；`mattn/go-sqlite3` 已不再是依赖。
- 已整合的上游修复：`Rebind` 忽略字符串与注释里的 `?`（#768）、`Conn` 补齐 `DB`/`Tx`
  的命名查询方法（#780）、支持命名参数后紧跟 PostgreSQL `::` 类型转换（#1004）、
  `Get` 返回 `sql.ErrNoRows` 时不再改动目标结构（#1005）、连接错误里的密码打码
  （#1006），以及 #757、#798、#805、#811、#846、#858、#938、#953、#962、#979、#993、
  #1003。
- 全面使用 `any` 替代 `interface{}`；要求 Go 1.24 及以上。
- CI 只保留 GitHub Actions，旧的 CircleCI 配置已删除。

## 注意事项

- `Row.Scan` 不接受 `*sql.RawBytes`，与 `database/sql` 的 `QueryRow` 一致。
- 多语句 `Exec` 的行为因驱动而异：`pq` 会全部执行，sqlite 不会。
- `SELECT *` 遇到可空列时，目标结构体里需要 `sql.Null*`（或指针）字段。
- `sqlx.Open`/`Connect` 会把畸形 DSN 中的密码在错误消息里打码。

## 开发

```shell
make test-race   # go test -race -count=1 ./...
make lint        # go vet ./... && staticcheck -checks=all ./...
make vuln-check  # govulncheck ./...
```

CI 在 Go 1.24.x 与 1.26.x 上跑 `go test ./...` 与 `go vet ./...`（带 PostgreSQL 服务）。
推送 `v*` tag 会触发 `Tag-release` 工作流，用
[chlog](https://github.com/gookit/gitw) 从提交记录生成发布说明并创建 Release。

## 许可证

MIT，详见 [LICENSE](LICENSE)。
