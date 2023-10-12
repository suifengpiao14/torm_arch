package tormdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm/pkg"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/singleflight"
)

type ExecutorSQL struct {
	config DBConfig
	_db    *sql.DB
	once   sync.Once
}

func NewExecutorSQLGetter(cfg DBConfig) (dbExecutorGetter DBExecutorGetter) {
	e := &ExecutorSQL{
		config: cfg,
	}
	return func() (dbExecutor DBExecutor) {
		return e
	}
}

func (e *ExecutorSQL) GetDB() (db *sql.DB) {
	e.once.Do(func() {
		cfg := e.config
		db, err := sql.Open(DriverName, e.config.DSN)
		if err != nil {
			if errors.Is(err, &net.OpError{}) {
				err = nil
				time.Sleep(100 * time.Millisecond)
				db, err = sql.Open(DriverName, cfg.DSN)
			}
		}
		if err != nil {
			panic(err)
		}
		sqlDB := db
		sqlDB.SetMaxOpenConns(cfg.MaxOpen)
		sqlDB.SetMaxIdleConns(cfg.MaxIdle)
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.MaxIdleTime) * time.Minute)
		e._db = db
	})
	return e._db
}

func (e *ExecutorSQL) Identify() string {
	return "dbExecutorSQL"
}

func (e *ExecutorSQL) ExecOrQueryContext(ctx context.Context, sqls string, out interface{}) (err error) {
	str, err := execOrQueryContext(ctx, e.GetDB(), sqls)
	if err != nil {
		return err
	}
	if str == "" {
		return
	}

	rt := reflect.Indirect(reflect.ValueOf(out)).Type()
	switch rt.Kind() {
	case reflect.Map, reflect.Struct:
		result := gjson.Parse(str)
		if result.IsArray() {
			str = result.Get("@this.0").String()
		}
	}
	err = json.Unmarshal([]byte(str), out)
	if err != nil {
		return err
	}
	return nil

}

var execOrQueryContextSingleflight = new(singleflight.Group)

func execOrQueryContext(ctx context.Context, sqlDB *sql.DB, sqls string) (out string, err error) {
	sqlLogInfo := &LogInfoEXECSQL{}
	defer func() {
		sqlLogInfo.Err = err
		logchan.SendLogInfo(sqlLogInfo)
	}()
	sqls = pkg.StandardizeSpaces(pkg.TrimSpaces(sqls)) // 格式化sql语句
	sqlLogInfo.SQL = sqls
	sqlType := SQLType(sqls)
	if sqlType != SQL_TYPE_SELECT {
		sqlLogInfo.BeginAt = time.Now().Local()
		res, err := sqlDB.ExecContext(ctx, sqls)
		if err != nil {
			return "", err
		}
		sqlLogInfo.EndAt = time.Now().Local()
		sqlLogInfo.AffectedRows, _ = res.RowsAffected()
		lastInsertId, _ := res.LastInsertId()
		if lastInsertId > 0 {
			return strconv.FormatInt(lastInsertId, 10), nil
		}
		rowsAffected, _ := res.RowsAffected()
		return strconv.FormatInt(rowsAffected, 10), nil
	}
	v, err, _ := execOrQueryContextSingleflight.Do(sqls, func() (interface{}, error) {
		sqlLogInfo.BeginAt = time.Now().Local()
		rows, err := sqlDB.QueryContext(ctx, sqls)
		sqlLogInfo.EndAt = time.Now().Local()
		if err != nil {
			return "", err
		}
		defer func() {
			err := rows.Close()
			if err != nil {
				panic(err)
			}
		}()
		allResult := make([][]map[string]string, 0)
		rowsAffected := 0
		for {
			records := make([]map[string]string, 0)
			for rows.Next() {
				rowsAffected++
				var record = make(map[string]interface{})
				var recordStr = make(map[string]string)
				err := MapScan(rows, record)
				if err != nil {
					return "", err
				}
				for k, v := range record {
					if v == nil {
						recordStr[k] = ""
					} else {
						recordStr[k] = fmt.Sprintf("%s", v)
					}
				}
				records = append(records, recordStr)
			}
			allResult = append(allResult, records)
			if !rows.NextResultSet() {
				break
			}
		}
		sqlLogInfo.AffectedRows = int64(rowsAffected)
		if len(allResult) == 1 { // allResult 初始值为[[]],至少有一个元素
			result := allResult[0]
			if len(result) == 0 { // 结果为空，返回空字符串
				return "", nil
			}
			if len(result) == 1 && len(result[0]) == 1 {
				row := result[0]
				for _, val := range row {
					return val, nil // 只有一个值时，直接返回值本身
				}
			}
			jsonByte, err := json.Marshal(result)
			if err != nil {
				return "", err
			}
			out = string(jsonByte)
			sqlLogInfo.Result = out
			return out, nil
		}

		jsonByte, err := json.Marshal(allResult)
		if err != nil {
			return "", err
		}
		out = string(jsonByte)
		sqlLogInfo.Result = out

		return out, nil
	})
	if err != nil {
		return "", nil
	}
	out = v.(string)
	return out, nil
}

// MapScan copy sqlx
func MapScan(r *sql.Rows, dest map[string]interface{}) error {
	// ignore r.started, since we needn't use reflect for anything.
	columns, err := r.Columns()
	if err != nil {
		return err
	}

	values := make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}

	err = r.Scan(values...)
	if err != nil {
		return err
	}

	for i, column := range columns {
		dest[column] = *(values[i].(*interface{}))
	}

	return r.Err()
}
