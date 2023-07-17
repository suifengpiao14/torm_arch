package tormsql

import (
	"reflect"
	"sync"
	"text/template"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm/pkg"
	"github.com/suifengpiao14/torm/tormdb"
	"github.com/suifengpiao14/torm/tormfunc"
	gormLogger "gorm.io/gorm/logger"
)

const (
	LOG_INFO_SQL LogName = "LogInfoSQL"
)

var sqlTemplateMap sync.Map

type SqlTplInstance struct {
	sqlTplIdentify   string
	dbExecutorGetter tormdb.DBExecutorGetter
	tpl              *template.Template
	once             sync.Once
}

var ERROR_SQL_TEMPLATE_NOT_FOUND_DB = errors.New("sqlTplInstance.dbInstance is nil")
var ERROR_SQL_TEMPLATE_NOT_FOUND_TEMPLATE = errors.New("sqlTplInstance.dbInstance is nil")
var ERROR_DB_EXECUTOR_GETTER_REQUIRD = errors.Errorf("dbExecutorGetter required")
var ERROR_DB_EXECUTOR_REQUIRD = errors.Errorf("dbExecutor required")

func (ins *SqlTplInstance) GetDBExecutor() (dbExecutor tormdb.DBExecutor) {
	if ins.dbExecutorGetter == nil {
		return nil
	}
	return ins.dbExecutorGetter()
}

func (ins *SqlTplInstance) GetTemplate() (r *template.Template) {
	return ins.tpl
}

func RegisterSQLTpl(sqlTplIdentify string, r *template.Template, dbExecutorGetter tormdb.DBExecutorGetter) (err error) {
	if r == nil {
		err = errors.Errorf("RegisterSQLTpl arg r required,got nil")
		return err
	}
	instance := SqlTplInstance{
		sqlTplIdentify:   sqlTplIdentify,
		tpl:              r,
		dbExecutorGetter: dbExecutorGetter,
		once:             sync.Once{},
	}
	sqlTemplateMap.Store(sqlTplIdentify, &instance)
	return nil
}

func GetSQLTpl(identify string) (sqlTplInstance *SqlTplInstance, err error) {
	val, ok := sqlTemplateMap.Load(identify)
	if !ok {
		err = errors.Errorf("not found db by identify:%s,use RegisterSQLTpl to set", identify)
		return nil, err
	}
	p, ok := val.(*SqlTplInstance)
	if !ok {
		err = errors.Errorf("required:%v,got:%v", &SqlTplInstance{}, val)
		return nil, err
	}
	return p, nil
}

type LogName string

func (l LogName) String() string {
	return string(l)
}

type LogInfoToSQL struct {
	SQL       string                 `json:"sql"`
	Named     string                 `json:"named"`
	NamedData map[string]interface{} `json:"namedData"`
	Data      interface{}            `json:"data"`
	Err       error                  `json:"error"`
	Level     string                 `json:"level"`
}

func (l LogInfoToSQL) GetName() logchan.LogName {
	return LOG_INFO_SQL
}
func (l LogInfoToSQL) Error() error {
	return l.Err
}
func (l LogInfoToSQL) GetLevel() string {
	return l.Level
}

// ToSQL 将字符串、数据整合为sql
func ToSQL(namedSql string, data interface{}) (sql string, err error) {
	namedSql = pkg.StandardizeSpaces(pkg.TrimSpaces(namedSql)) // 格式化sql语句
	logInfo := LogInfoToSQL{
		Named: namedSql,
		Data:  data,
		Err:   err,
	}

	defer func() {
		logInfo.SQL = sql
		logInfo.Err = err
		logchan.SendLogInfo(logInfo)
	}()
	namedData, err := getNamedData(data)
	if err != nil {
		return "", err
	}
	logInfo.NamedData = namedData
	statment, arguments, err := sqlx.Named(namedSql, namedData)
	if err != nil {
		err = errors.WithStack(err)
		return "", err
	}
	sql = gormLogger.ExplainSQL(statment, nil, `'`, arguments...)
	return sql, nil
}

func getNamedData(data interface{}) (out map[string]interface{}, err error) {
	out = make(map[string]interface{})
	if data == nil {
		return
	}
	dataI, ok := data.(*interface{})
	if ok {
		data = *dataI
	}
	mapOut, ok := data.(map[string]interface{})
	if ok {
		out = mapOut
		return
	}
	mapOutRef, ok := data.(*map[string]interface{})
	if ok {
		out = *mapOutRef
		return
	}
	if mapOut, ok := data.(tormfunc.VolumeMap); ok {
		out = mapOut
		return
	}
	if mapOutRef, ok := data.(*tormfunc.VolumeMap); ok {
		out = *mapOutRef
		return
	}

	v := reflect.Indirect(reflect.ValueOf(data))

	if v.Kind() != reflect.Struct {
		return
	}
	vt := v.Type()
	// 提取结构体field字段
	fieldNum := v.NumField()
	for i := 0; i < fieldNum; i++ {
		fv := v.Field(i)
		ft := fv.Type()
		fname := vt.Field(i).Name
		if fv.Kind() == reflect.Ptr {
			fv = fv.Elem()
			ft = fv.Type()
		}
		ftk := ft.Kind()
		switch ftk {
		case reflect.Int:
			out[fname] = fv.Int()
		case reflect.Int64:
			out[fname] = int64(fv.Int())
		case reflect.Float64:
			out[fname] = fv.Float()
		case reflect.String:
			out[fname] = fv.String()
		case reflect.Struct, reflect.Map:
			subOut, err := getNamedData(fv.Interface())
			if err != nil {
				return out, err
			}
			for k, v := range subOut {
				if _, ok := out[k]; !ok {
					out[k] = v
				}
			}

		default:
			out[fname] = fv.Interface()
		}
	}
	return
}
