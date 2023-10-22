package tormdb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm/tormfunc"
)

var ERROR_DB_RECORD_NOT_FOUND = errors.New("record not found")
var ERROR_DB_EXECUTOR_NOT_FOUND = errors.New("not found dbExecutor")

type DBExecutor interface {
	ExecOrQueryContext(ctx context.Context, sqls string, out interface{}) (err error)
}
type DBExecutorGetter func() (dbExecutor DBExecutor)

type DBConfig struct {
	DSN         string `json:"dsn"`
	LogLevel    string `json:"logLevel"`
	Timeout     int    `json:"timeout"`
	MaxOpen     int    `json:"maxOpen"`
	MaxIdle     int    `json:"maxIdle"`
	MaxIdleTime int    `json:"maxIdleTime"`
}

type LogName string

func (l LogName) String() string {
	return string(l)
}

type LogInfoEXECSQL struct {
	Context      context.Context
	SQL          string    `json:"sql"`
	Result       string    `json:"result"`
	Err          error     `json:"error"`
	BeginAt      time.Time `json:"beginAt"`
	EndAt        time.Time `json:"endAt"`
	Duration     string    `json:"time"`
	AffectedRows int64     `json:"affectedRows"`
	LastInsertId int64     `json:"lastInsertId"`
	Level        string    `json:"level"`
	logchan.EmptyLogInfo
}

func (l *LogInfoEXECSQL) GetName() logchan.LogName {
	return LOG_INFO_EXEC_SQL
}
func (l *LogInfoEXECSQL) Error() error {
	return l.Err
}
func (l *LogInfoEXECSQL) GetLevel() string {
	return l.Level
}
func (l *LogInfoEXECSQL) BeforeSend() {
	duration := float64(l.EndAt.Sub(l.BeginAt).Nanoseconds()) / 1e6
	l.Duration = fmt.Sprintf("%.3fms", duration)
}

const (
	LOG_INFO_EXEC_SQL LogName = "LogInfoEXECSQL"
)

// DefaultPrintLogInfoEXECSQL 默认日志打印函数
func DefaultPrintLogInfoEXECSQL(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error) {
	if typeName != LOG_INFO_EXEC_SQL {
		return
	}
	logInfoEXECSQL, ok := logInfo.(*LogInfoEXECSQL)
	if !ok {
		return
	}
	if err != nil {
		_, err1 := fmt.Fprintf(logchan.LogWriter, "%s|loginInfo:%s|error:%s\n", logchan.DefaultPrintLog(logInfoEXECSQL), logInfoEXECSQL.GetName(), err.Error())
		if err1 != nil {
			fmt.Printf("err: DefaultPrintLogInfoEXECSQL fmt.Fprintf:%s\n", err1.Error())
		}
		return
	}
	_, err1 := fmt.Fprintf(logchan.LogWriter, "%s|SQL:%+s [%s rows:%d]\n", logchan.DefaultPrintLog(logInfoEXECSQL), logInfoEXECSQL.SQL, logInfoEXECSQL.Duration, logInfoEXECSQL.AffectedRows)
	if err1 != nil {
		fmt.Printf("err: DefaultPrintLogInfoEXECSQL fmt.Fprintf:%s\n", err1.Error())
	}
}

var DriverName = "mysql"

const (
	SQL_TYPE_SELECT = "SELECT"
	SQL_TYPE_OTHER  = "OTHER"
)

// SQLType 判断 sql  属于那种类型
func SQLType(sqls string) string {
	sqlArr := strings.Split(sqls, tormfunc.EOF)
	selectLen := len(SQL_TYPE_SELECT)
	for _, sql := range sqlArr {
		if len(sql) < selectLen {
			continue
		}
		pre := sql[:selectLen]
		if strings.ToUpper(pre) == SQL_TYPE_SELECT {
			return SQL_TYPE_SELECT
		}
	}
	return SQL_TYPE_OTHER
}

var dbExecutorMap sync.Map

func RegisterDBExecutor(identify string, dbExecutor DBExecutor) (dbExecutorGetter DBExecutorGetter) {
	dbExecutorMap.Store(identify, dbExecutor)
	dbExecutorGetter = func() (dbExecutor DBExecutor) {
		dbExecutor, ok := GetDBExecutor(identify)
		if !ok {
			err := errors.WithMessagef(ERROR_DB_EXECUTOR_NOT_FOUND, "by identify:%s", identify)
			panic(err)
		}
		return dbExecutor
	}
	return
}

func GetDBExecutor(identify string) (dbExecutor DBExecutor, ok bool) {
	value, ok := dbExecutorMap.Load(identify)
	if !ok {
		return nil, false
	}
	dbExecutor, ok = value.(DBExecutor)
	return dbExecutor, ok
}
