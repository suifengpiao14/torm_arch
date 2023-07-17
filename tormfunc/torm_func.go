package tormfunc

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rs/xid"
	"github.com/suifengpiao14/funcs"
)

const IN_INDEX = "__inIndex"

var tormfuncMapSQL = template.FuncMap{
	"zeroTime":      ZeroTime,
	"currentTime":   CurrentTime,
	"permanentTime": PermanentTime,
	"contains":      strings.Contains,
	"newPreComma":   NewPreComma,
	"in":            In,
	"toCamel":       funcs.ToCamel,
	"toLowerCamel":  funcs.ToLowerCamel,
	"snakeCase":     funcs.SnakeCase,
	//"joinAll":           JoinAll,
	"md5lower":        MD5LOWER,
	"fen2yuan":        Fen2yuan,
	"timestampSecond": TimestampSecond,
	"xid":             Xid,
	"noEmpty":         NoEmpty,
	"insert":          Insert,
	//"jsonCompact":       JsonCompact,
	//"standardizeSpaces": util.StandardizeSpaces,
}

func ZeroTime(volume VolumeInterface) (string, error) {
	named := "ZeroTime"
	placeholder := ":" + named
	value := "0000-00-00 00:00:00"
	volume.SetValue(named, value)
	return placeholder, nil
}

func CurrentTime(volume VolumeInterface) (string, error) {
	named := "CurrentTime"
	placeholder := ":" + named
	value := time.Now().Format("2006-01-02 15:04:05")
	volume.SetValue(named, value)
	return placeholder, nil
}

func PermanentTime(volume VolumeInterface) (string, error) {
	named := "PermanentTime"
	placeholder := ":" + named
	value := "3000-12-31 23:59:59"
	volume.SetValue(named, value)
	return placeholder, nil
}

func MD5LOWER(s ...string) string {
	allStr := strings.Join(s, "")
	h := md5.New()
	h.Write([]byte(allStr))
	return hex.EncodeToString(h.Sum(nil))
}

func Fen2yuan(fen interface{}) string {
	var yuan float64
	intFen, ok := fen.(int)
	if ok {
		yuan = float64(intFen) / 100
		return strconv.FormatFloat(yuan, 'f', 2, 64)
	}
	strFen, ok := fen.(string)
	if ok {
		intFen, err := strconv.Atoi(strFen)
		if err == nil {
			yuan = float64(intFen) / 100
			return strconv.FormatFloat(yuan, 'f', 2, 64)
		}
	}
	return strFen
}

// 秒计数的时间戳
func TimestampSecond() int64 {
	return time.Now().Unix()
}

func Xid() string {
	guid := xid.New()
	return guid.String()
}

type preComma struct {
	comma string
}

func NewPreComma() *preComma {
	return &preComma{}
}

func (c *preComma) PreComma() string {
	out := c.comma
	c.comma = ","
	return out
}

var GetColumnNameFn GetColumnNameFromTag = getGormColumnNameFromTag

func struct2GormMap(v interface{}) (m map[string]interface{}, keyOrder []string) {
	rv, ok := v.(reflect.Value)
	if !ok {
		rv = reflect.Indirect(reflect.ValueOf(v))
	}
	m = make(map[string]interface{})
	keyOrder = make([]string, 0)
	rt := rv.Type()
	switch rv.Kind() {
	case reflect.Map:
		iter := rv.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			m[k.String()] = v.Interface()
			keyOrder = append(keyOrder, k.String())
		}
		return m, keyOrder
	case reflect.Struct:
		for i := 0; i < rt.NumField(); i++ {
			tag := rt.Field(i).Tag
			coluName := GetColumnNameFn(tag)
			if coluName == "" {
				continue
			}
			m[coluName] = rv.Field(i).Interface()
			keyOrder = append(keyOrder, coluName)
		}
		return m, keyOrder
	}
	return
}

type GetColumnNameFromTag func(tag reflect.StructTag) (colName string)

func getGormColumnNameFromTag(tag reflect.StructTag) (colName string) {
	gormTag := tag.Get("gorm")
	if gormTag == "" {
		return ""
	}
	tagParts := strings.Split(gormTag, ";")
	for _, part := range tagParts {
		if strings.HasPrefix(part, "column:") {
			colName = strings.TrimPrefix(part, "column:")
			return colName
		}
	}
	return ""
}

func Insert(volume VolumeInterface, data interface{}) (str string, err error) {
	v := reflect.Indirect(reflect.ValueOf(data))
	column := make([]string, 0)
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		rows := v.Len()
		valuesHolder := make([]string, 0)
		for i := 0; i < rows; i++ {
			row := reflect.Indirect(v.Index(i))
			switch row.Kind() {
			case reflect.Struct, reflect.Map:
				vMap, keyOrder := struct2GormMap(row)
				if i == 0 {
					column = keyOrder
				}
				valuePlaceHoder, valueMap := insertValuePlaceholder(vMap, i, column)
				valuesHolder = append(valuesHolder, valuePlaceHoder)
				for named, v := range valueMap {
					volume.SetValue(named, v)
				}
			default:
				err = fmt.Errorf("want []map[string]interface{}/[]struct{} ,got %T", v.Interface())
				return "", err
			}
		}
		columnStr := fmt.Sprintf("(`%s`)", strings.Join(column, "`,`"))
		valuesHolderStr := strings.Join(valuesHolder, ",")
		str = fmt.Sprintf(" %s values %s", columnStr, valuesHolderStr) // 开头留下空格，方便后续拼接
		return str, nil
	case reflect.Map, reflect.Struct:
		vMap, column := struct2GormMap(v)
		valuePlaceHoder, valueMap := insertValuePlaceholder(vMap, 0, column)
		for named, v := range valueMap {
			volume.SetValue(named, v)
		}
		columnStr := fmt.Sprintf("(`%s`)", strings.Join(column, "`,`"))
		str = fmt.Sprintf(" %s values %s", columnStr, valuePlaceHoder) // 开头留下空格，方便后续拼接
		return str, nil

	default:
		err = fmt.Errorf("want slice/array/map/struct ,have %s", v.Kind().String())
		if err != nil {
			return "", err
		}
	}
	return str, nil
}

func insertValuePlaceholder(v map[string]interface{}, index int, column []string) (valuePlaceHolder string, namedMap map[string]interface{}) {
	namedMap = make(map[string]interface{})
	placeholders := make([]string, 0)
	for _, colName := range column {
		named := fmt.Sprintf("insert_%d_%s", index, colName)
		placeholder := ":" + named
		placeholders = append(placeholders, placeholder)
		value := v[colName]
		namedMap[named] = value
	}
	valuePlaceHolder = fmt.Sprintf(`(%s)`, strings.Join(placeholders, ","))
	return
}

func In(volume VolumeInterface, data interface{}) (str string, err error) {
	placeholders := make([]string, 0)
	inIndexKey := IN_INDEX
	var inIndex int
	ok := volume.GetValue(inIndexKey, &inIndex)
	if !ok {
		inIndex = 0
	}

	v := reflect.Indirect(reflect.ValueOf(data))

	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		num := v.Len()
		for i := 0; i < num; i++ {
			inIndex++
			named := fmt.Sprintf("in_%d", inIndex)
			placeholder := ":" + named
			placeholders = append(placeholders, placeholder)
			volume.SetValue(named, v.Index(i).Interface())
		}

	case reflect.String:
		arr := strings.Split(v.String(), ",")
		num := len(arr)
		for i := 0; i < num; i++ {
			inIndex++
			named := fmt.Sprintf("in_%d", inIndex)
			placeholder := ":" + named
			placeholders = append(placeholders, placeholder)
			volume.SetValue(named, arr[i])
		}
	default:
		err = fmt.Errorf("want slice/array/string ,have %s", v.Kind().String())
		if err != nil {
			return "", err
		}
	}
	volume.SetValue(inIndexKey, inIndex) // 更新InIndex_
	str = strings.Join(placeholders, ",")
	return str, nil

}

const (
	PER_CNET_CHART = '%'
)

// NoEmpty variable为空输出空字符；如果format 中没有占位符原样输出format,否则按fmt.Sprintf 打印字符,等价模板中一下if场景： {{if .Variable}} xxx {{.Variable}} {{end}} 和  {{if .Variable}} xxx  {{end}}
func NoEmpty(format string, variable interface{}) (out string, err error) {
	if variable == nil {
		return "", nil
	}
	if isBlank(reflect.Indirect(reflect.ValueOf(variable))) {
		return "", nil
	}
	b := []byte(format)
	hasPlaceHolder := false
	onOff := false
	for _, char := range b {
		if char == PER_CNET_CHART {
			onOff = !onOff
			continue
		}
		if onOff {
			hasPlaceHolder = true
			break
		}
	}
	out = format
	if hasPlaceHolder {
		out = fmt.Sprintf(format, variable)
	}
	return out, nil
}

func isBlank(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return value.IsNil()
	}
	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}
