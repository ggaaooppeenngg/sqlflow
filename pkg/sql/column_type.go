// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	// hive column type ends with _TYPE
	hiveCTypeSuffix = "_TYPE"

	timeFormat = "2006-01-02 15:04:05.999999"
)

func createByType(t reflect.Type) interface{} {
	return reflect.New(t).Interface()
}

// Copied from github.com/go-sql-driver/mysql
type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
// The value type must be time.Time or string / []byte (formatted time-string),
// otherwise Scan fails.
func (nt *NullTime) Scan(value interface{}) (err error) {
	if value == nil {
		nt.Time, nt.Valid = time.Time{}, false
		return
	}

	switch v := value.(type) {
	case time.Time:
		nt.Time, nt.Valid = v, true
		return
	case []byte:
		nt.Time, err = parseDateTime(string(v), time.UTC)
		nt.Valid = (err == nil)
		return
	case string:
		// Handle NULL timestamp from hive
		if v == "" {
			nt.Time, nt.Valid = time.Time{}, false
			return
		}
		nt.Time, err = parseDateTime(v, time.UTC)
		nt.Valid = (err == nil)
		return
	}

	nt.Valid = false
	return fmt.Errorf("Can't convert %T to time.Time", value)
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

func parseDateTime(str string, loc *time.Location) (t time.Time, err error) {
	base := "0000-00-00 00:00:00.0000000"
	switch len(str) {
	case 10, 19, 21, 22, 23, 24, 25, 26: // up to "YYYY-MM-DD HH:MM:SS.MMMMMM"
		if str == base[:len(str)] {
			return
		}
		t, err = time.Parse(timeFormat[:len(str)], str)
	default:
		err = fmt.Errorf("invalid time string: %s", str)
		return
	}

	// Adjust location
	if err == nil && loc != time.UTC {
		y, mo, d := t.Date()
		h, mi, s := t.Clock()
		t, err = time.Date(y, mo, d, h, mi, s, t.Nanosecond(), loc), nil
	}

	return
}

func parseVal(val interface{}) (interface{}, error) {
	switch v := val.(type) {
	case *sql.NullBool:
		if (*v).Valid {
			return (*v).Bool, nil
		}
		return nil, nil
	case *sql.NullInt64:
		if (*v).Valid {
			return (*v).Int64, nil
		}
		return nil, nil
	case *sql.NullFloat64:
		if (*v).Valid {
			return (*v).Float64, nil
		}
		return nil, nil
	case *sql.RawBytes:
		if *v == nil {
			return nil, nil
		}
		return string(*v), nil
	case *sql.NullString:
		if (*v).Valid {
			return (*v).String, nil
		}
		return nil, nil
	case *mysql.NullTime:
		if (*v).Valid {
			return (*v).Time, nil
		}
		return nil, nil
	// hive  time
	case *NullTime:
		if (*v).Valid {
			return (*v).Time, nil
		}
		return nil, nil
	case *(time.Time):
		return *v, nil
	case *([]byte):
		if *v == nil {
			return nil, nil
		}
		return *v, nil
	case *bool:
		return *v, nil
	case *string:
		return *v, nil
	case *int:
		return *v, nil
	case *int8:
		return *v, nil
	case *int16:
		return *v, nil
	case *int32:
		return *v, nil
	case *int64:
		return *v, nil
	case *uint:
		return *v, nil
	case *uint8:
		return *v, nil
	case *uint16:
		return *v, nil
	case *uint32:
		return *v, nil
	case *uint64:
		return *v, nil
	case *float32:
		return *v, nil
	case *float64:
		return *v, nil
	default:
		return nil, fmt.Errorf("unrecognized type %v", v)
	}
}

func universalizeColumnType(driverName, dialectType string) (string, error) {
	if driverName == "mysql" || driverName == "maxcompute" {
		if dialectType == "VARCHAR" {
			// FIXME(tony): MySQL driver DatabaseName doesn't include the type length of a field.
			// Hardcoded to 255 for now.
			// ref: https://github.com/go-sql-driver/mysql/blob/877a9775f06853f611fb2d4e817d92479242d1cd/fields.go#L87
			return "VARCHAR(255)", nil
		}
		return dialectType, nil
	} else if driverName == "hive" {
		if strings.HasSuffix(dialectType, hiveCTypeSuffix) {
			return dialectType[:len(dialectType)-len(hiveCTypeSuffix)], nil
		}
		// In hive, capacity is also needed when define a VARCHAR field, so we replace it with STRING.
		if dialectType == "VARCHAR" {
			return "STRING", nil
		}
		return dialectType, nil
	}
	return "", fmt.Errorf("not support driver:%s", driverName)
}
