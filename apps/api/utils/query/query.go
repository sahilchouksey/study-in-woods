package queryHelper

import (
	"fmt"
	"reflect"
	"strings"
)

//func UpdateQueryBuilder(table_name string, identifier string, id int64, data interface{}) (string, []interface{}) {
//	query := fmt.Sprintf("UPDATE %s SET ", table_name)
//	values := []interface{}{}
//
//	v := reflect.ValueOf(data)
//
//	index := 1
//	for i := 0; i < v.NumField(); i++ {
//		if v.Field(i).Interface() != "" && strings.ToLower(v.Type().Field(i).Name) != identifier {
//			query += fmt.Sprintf("%s=$%d, ", strings.ToLower(v.Type().Field(i).Name), index)
//			values = append(values, v.Field(i).Interface())
//			index++
//		}
//	}
//
//	query = strings.TrimSuffix(query, ", ")
//	query += fmt.Sprintf(" WHERE %s=$%d;", identifier, len(values)+1)
//
//	values = append(values, id)
//
//	return query, values
//}

func UpdateQueryBuilder(tableName string, identifier string, id int64, data interface{}) (string, []interface{}) {
	fmt.Println("UpdateQueryBuilder", tableName, identifier, id, data)
	query := fmt.Sprintf("UPDATE %s SET ", tableName)
	values := []interface{}{}

	v := reflect.ValueOf(data)

	index := 1
	for i := 0; i < v.NumField(); i++ {
		if strings.ToLower(v.Type().Field(i).Name) != identifier && !reflect.DeepEqual(v.Field(i).Interface(), reflect.Zero(v.Field(i).Type()).Interface()) {
			query += fmt.Sprintf("%s=$%d, ", strings.ToLower(v.Type().Field(i).Name), index)
			values = append(values, v.Field(i).Interface())
			index++
		}
	}

	query = strings.TrimSuffix(query, ", ")
	query += fmt.Sprintf(" WHERE %s=$%d;", identifier, len(values)+1)

	values = append(values, id)

	fmt.Println(query)

	return query, values
}
