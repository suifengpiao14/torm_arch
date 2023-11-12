package torm

import (
	"context"
	"text/template"

	"github.com/suifengpiao14/torm/tormdb"
	"github.com/suifengpiao14/torm/tormfunc"
	"github.com/suifengpiao14/torm/tormsql"
)

func RegisterSQLTpl(sqlTplIdentify string, r *template.Template, dbExectorGetter tormdb.DBExecutorGetter) {
	tormsql.RegisterSQLTpl(sqlTplIdentify, r, dbExectorGetter)
}

func GetSQLTpl(identify string) (sqlTplInstance *tormsql.SqlTplInstance, err error) {
	return tormsql.GetSQLTpl(identify)
}

// GetSQL 生成SQL(不关联DB操作)
func GetSQL(sqlTplIdentify string, tplName string, volume tormfunc.VolumeInterface) (sqls string, namedSQL string, resetedVolume tormfunc.VolumeInterface, err error) {
	sqlTplInstance, err := GetSQLTpl(sqlTplIdentify)
	if err != nil {
		return "", "", nil, err
	}
	t := sqlTplInstance.GetTemplate()
	return getSQL(t, tplName, volume)
}

func getSQL(t *template.Template, tplName string, volume tormfunc.VolumeInterface) (sqls string, namedSQL string, resetedVolume tormfunc.VolumeInterface, err error) {
	namedSQL, resetedVolume, err = tormfunc.ExecTPL(t, tplName, volume)
	if err != nil {
		return "", "", nil, err
	}
	sqls, err = tormsql.ToSQL(namedSQL, resetedVolume)
	if err != nil {
		return "", "", nil, err
	}
	return sqls, namedSQL, resetedVolume, nil
}

// ExecSQLTpl 执行模板中sql语句
func ExecSQLTpl(ctx context.Context, sqlTplIdentify string, tplName string, volume tormfunc.VolumeInterface, out interface{}) (err error) {
	sqlTplInstance, err := GetSQLTpl(sqlTplIdentify)
	if err != nil {
		return err
	}

	sqls, _, _, err := getSQL(sqlTplInstance.GetTemplate(), tplName, volume)
	if err != nil {
		return err
	}
	err = ExecSQL(ctx, sqlTplIdentify, sqls, out)
	if err != nil {
		return err
	}
	return nil
}

// ExecSQL 执行sql语句
func ExecSQL(ctx context.Context, sqlTplIdentify string, sql string, out interface{}) (err error) {
	sqlTplInstance, err := GetSQLTpl(sqlTplIdentify)
	if err != nil {
		return err
	}
	dbExecutor := sqlTplInstance.GetDBExecutor()
	if dbExecutor == nil {
		err = tormsql.ERROR_DB_EXECUTOR_REQUIRD
		return err
	}
	err = dbExecutor.ExecOrQueryContext(ctx, sql, out)
	if err != nil {
		return err
	}
	return nil
}
