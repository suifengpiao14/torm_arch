package tormfunc

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoEmpty(t *testing.T) {
	format := "and id=%d"

	t.Run("zero int", func(t *testing.T) {
		id := 0
		out, err := NoEmpty(format, id)
		require.NoError(t, err)
		assert.Equal(t, "", out)
	})
	t.Run("zero float", func(t *testing.T) {
		id := 0.0
		out, err := NoEmpty(format, id)
		require.NoError(t, err)
		assert.Equal(t, "", out)
	})

	t.Run("empty string", func(t *testing.T) {
		id := 0.0
		out, err := NoEmpty(format, id)
		require.NoError(t, err)
		assert.Equal(t, "", out)
	})
	t.Run("and id=1", func(t *testing.T) {
		id := 1
		out, err := NoEmpty(format, id)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf(format, id), out)
	})

	t.Run("and name like %xxs%", func(t *testing.T) {
		format := "and name like %%%s%%"
		name := "test"
		out, err := NoEmpty(format, name)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf(format, name), out)
	})
	t.Run("and name =:Name", func(t *testing.T) {
		format := "and name =:Name"
		name := "test"
		out, err := NoEmpty(format, name)
		require.NoError(t, err)
		assert.Equal(t, format, out)
	})

	t.Run("and name =:Name%%", func(t *testing.T) {
		format := "and name =:Name%%"
		name := "test"
		out, err := NoEmpty(format, name)
		require.NoError(t, err)
		assert.Equal(t, format, out)
	})
	t.Run("and name =:Name%%s", func(t *testing.T) {
		format := "and name =:Name%%s"
		name := "test"
		out, err := NoEmpty(format, name)
		require.NoError(t, err)
		assert.Equal(t, format, out)
	})

	t.Run("and name =:Name%", func(t *testing.T) {
		format := "and name =:Name%"
		name := "test"
		out, err := NoEmpty(format, name)
		require.NoError(t, err)
		assert.Equal(t, format, out)
	})

}

func TestStruct2GormMap(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		user := UserModel{
			ID:      1,
			Name:    "张三",
			Address: "深圳",
		}
		m, _ := struct2GormMap(user)
		fmt.Println(m)
	})

	t.Run("map", func(t *testing.T) {
		user := map[string]string{
			"Name":    "张三",
			"Address": "深圳",
		}
		m, keyOrder := struct2GormMap(user)
		fmt.Println(m)
		fmt.Println(keyOrder)
	})

}

type UserModel struct {
	ID      int    `gorm:"column:Fid"`
	Name    string `gorm:"column:Fname"`
	Address string `gorm:"column:Faddress"`
}

func TestInsert(t *testing.T) {

	users := make([]UserModel, 0)
	users = append(users, UserModel{
		ID:      1,
		Name:    "张三",
		Address: "深圳",
	})
	users = append(users, UserModel{
		ID:      2,
		Name:    "李四",
		Address: "北京",
	})
	users = append(users, UserModel{
		ID:      3,
		Name:    "王伟",
		Address: "深圳",
	})

	t.Run("batch", func(t *testing.T) {
		v := NewVolumeMap()
		str, err := Insert(v, users)
		require.NoError(t, err)
		expected := " (`Fid`,`Fname`,`Faddress`) values (:insert_0_Fid,:insert_0_Fname,:insert_0_Faddress),(:insert_1_Fid,:insert_1_Fname,:insert_1_Faddress),(:insert_2_Fid,:insert_2_Fname,:insert_2_Faddress)"
		assert.Equal(t, expected, str)
		b, err := json.Marshal(v)
		require.NoError(t, err)
		batchData := string(b)
		expectedBatchData := `{"insert_0_Faddress":"深圳","insert_0_Fid":1,"insert_0_Fname":"张三","insert_1_Faddress":"北京","insert_1_Fid":2,"insert_1_Fname":"李四","insert_2_Faddress":"深圳","insert_2_Fid":3,"insert_2_Fname":"王伟"}`
		assert.JSONEq(t, expectedBatchData, batchData)

	})

	t.Run("one", func(t *testing.T) {
		v := NewVolumeMap()
		str, err := Insert(v, users[0])
		require.NoError(t, err)
		expected := " (`Fid`,`Fname`,`Faddress`) values (:insert_0_Fid,:insert_0_Fname,:insert_0_Faddress)"
		assert.Equal(t, expected, str)
		b, err := json.Marshal(v)
		require.NoError(t, err)
		oneData := string(b)
		expectedOneData := `{"insert_0_Faddress":"深圳","insert_0_Fid":1,"insert_0_Fname":"张三"}`
		assert.JSONEq(t, expectedOneData, oneData)
	})
}
