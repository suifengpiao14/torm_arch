package tormdb

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm/pkg"
	"golang.org/x/sync/singleflight"
)

const (
	max_retry_times = 2
)

type ExecutorGorm struct {
	dbConfig   DBConfig
	sshConfig  *SSHConfig
	_db        *gorm.DB
	once       sync.Once
	retryTimes int
}

func NewExecutorGormGetter(cfg DBConfig, sshCfg *SSHConfig) (dbExecutorGetter DBExecutorGetter) {
	e := &ExecutorGorm{
		dbConfig:  cfg,
		sshConfig: sshCfg,
	}
	return func() (dbExecutor DBExecutor) {
		return e
	}
}

func (e *ExecutorGorm) GetDB() (db *gorm.DB) {
	var err error
	for {
		if e.retryTimes > max_retry_times {
			break
		}
		e.retryTimes++
		err = e.connect()
		if err == nil {
			break
		}
		e.once = sync.Once{} // 重新初始化

	}
	if err != nil {
		panic(err)
	}
	e.retryTimes = 0
	return e._db
}

func (e *ExecutorGorm) connect() (err error) {
	e.once.Do(func() {
		cfg := e.dbConfig
		var gormConnect interface{} = cfg.DSN
		if e.sshConfig != nil {
			gormConnect, err = Tunnel(*e.sshConfig, e.dbConfig.DSN)
			if err != nil {
				return
			}
		}
		var db *gorm.DB
		db, err = gorm.Open(DriverName, gormConnect)
		if err != nil {
			return
		}
		sqlDB := db.DB()
		sqlDB.SetMaxOpenConns(cfg.MaxOpen)
		sqlDB.SetMaxIdleConns(cfg.MaxIdle)
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.MaxIdleTime) * time.Minute)
		e._db = db
	})
	if err != nil {
		return err
	}
	if e._db == nil {
		err = errors.New("db is nil")
		return err
	}
	err = e._db.DB().Ping() // 增加ping实现重链
	if err != nil {
		return err
	}

	return nil
}

func (e *ExecutorGorm) Identify() string {
	return "dbExecutorGorm"
}

func (e *ExecutorGorm) ExecOrQueryContext(ctx context.Context, sqls string, out interface{}) (err error) {
	return execOrQueryContextUseGorm(ctx, e.GetDB(), sqls, out)
}

var execOrQueryContextUseGormSingleflight = new(singleflight.Group)

func execOrQueryContextUseGorm(ctx context.Context, gormDB *gorm.DB, sqls string, out interface{}) (err error) {
	sqlLogInfo := &LogInfoEXECSQL{}
	defer func() {
		sqlLogInfo.Err = err
		if out != nil {
			jsonByte, _ := json.Marshal(out)
			outStr := string(jsonByte)
			sqlLogInfo.Result = outStr
		}
		logchan.SendLogInfo(sqlLogInfo)
	}()
	sqls = pkg.StandardizeSpaces(pkg.TrimSpaces(sqls)) // 格式化sql语句
	sqlLogInfo.SQL = sqls
	sqlType := SQLType(sqls)
	rv := reflect.Indirect(reflect.ValueOf(out))
	kind := reflect.Invalid
	if out != nil {
		kind = rv.Type().Kind()
		if !rv.CanSet() {
			err = errors.Errorf("execOrQueryContextUseGorm arg out must CanSet")
			return err
		}
	}
	if sqlType != SQL_TYPE_SELECT {
		sqlLogInfo.BeginAt = time.Now().Local()
		res, err := gormDB.DB().ExecContext(ctx, sqls)
		if err != nil {
			return err
		}
		sqlLogInfo.EndAt = time.Now().Local()
		rowsAffected, _ := res.RowsAffected()
		sqlLogInfo.AffectedRows = rowsAffected
		switch kind {
		case reflect.Int, reflect.Int64:
			rv.SetInt(rowsAffected)

			lastInsertId, _ := res.LastInsertId()
			sqlLogInfo.LastInsertId = lastInsertId
			if lastInsertId > 0 {
				switch kind {
				case reflect.Int, reflect.Int64:
					rv.SetInt(lastInsertId)

				}
				return nil
			}

		}
		return nil
	}
	v, err, _ := execOrQueryContextUseGormSingleflight.Do(sqls, func() (interface{}, error) {
		result := gormDB.Raw(sqls)
		if result.Error != nil {
			return nil, result.Error
		}
		sqlLogInfo.BeginAt = time.Now().Local()
		rv := reflect.Indirect(reflect.ValueOf(out))
		switch rv.Type().Kind() {
		case reflect.Float64, reflect.Int, reflect.Int64:
			err = result.Count(out).Error
		default:
			err = result.Scan(out).Error
		}
		sqlLogInfo.EndAt = time.Now().Local()
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) { // 替换错误类型，屏蔽内部引用
			err = ERROR_DB_RECORD_NOT_FOUND
		}
		return out, err
	})
	if err != nil {
		return err
	}
	rv.Set(reflect.Indirect(reflect.ValueOf(v))) // 给其它的请求赋值

	switch kind {
	case reflect.Array, reflect.Slice:
		sqlLogInfo.AffectedRows = int64(rv.Len())
	case reflect.Struct:
		sqlLogInfo.AffectedRows = 1
	}

	return nil
}
