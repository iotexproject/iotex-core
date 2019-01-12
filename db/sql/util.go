// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"reflect"

	"database/sql"
)

// ParseSQLRows will parse the row
func ParseSQLRows(rows *sql.Rows, schema interface{}) ([]interface{}, error) {
	var parsedRows []interface{}

	// Fetch rows
	for rows.Next() {
		newSchema := reflect.New(reflect.ValueOf(schema).Elem().Type()).Interface()

		s := reflect.ValueOf(newSchema).Elem()

		var fields []interface{}
		for i := 0; i < s.NumField(); i++ {
			fields = append(fields, s.Field(i).Addr().Interface())
		}

		err := rows.Scan(fields...)
		if err != nil {
			return nil, err
		}

		parsedRows = append(parsedRows, newSchema)
	}

	return parsedRows, nil
}
