package rds

import (
	"reflect"

	"database/sql"
)

// ParseRows will parse the row
func ParseRows(rows *sql.Rows, schema interface{}) ([]interface{}, error) {
	var parsedRows []interface{}
	s := reflect.ValueOf(schema).Elem()

	var fields []interface{}
	for i := 0; i < s.NumField(); i++ {
		fields = append(fields, s.Field(i).Addr().Interface())
	}

	// Fetch rows
	for rows.Next() {
		err := rows.Scan(fields...)
		if err != nil {
			return nil, err
		}

		parsedRows = append(parsedRows, schema)
	}

	return parsedRows, nil
}
