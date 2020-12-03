package xorm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	digits10 = "0000000000111111111122222222223333333333444444444455555555556666666666777777777788888888889999999999"
	digits01 = "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"
)

var (
	// ErrInvalidArgs args count error
	ErrInvalidArgs = errors.New("invalid args")
	// ErrInvalidType unknow type
	ErrInvalidType = errors.New("invalid type")
	// ErrInvalidParseArgs parse args not equal to args
	ErrInvalidParseArgs = errors.New("invalid parse args")
)

// reserveBuffer checks cap(buf) and expand buffer to len(buf) + appendSize.
// If cap(buf) is not enough, reallocate new buffer.
func reserveBuffer(buf []byte, appendSize int) []byte {
	newSize := len(buf) + appendSize
	if cap(buf) < newSize {
		// Grow buffer exponentially
		newBuf := make([]byte, len(buf)*2+appendSize)
		copy(newBuf, buf)
		buf = newBuf
	}
	return buf[:newSize]
}

func escapeBytesQuotes(buf, v []byte) []byte {
	pos := len(buf)
	buf = reserveBuffer(buf, len(v)*2)

	for _, c := range v {
		if c == '\'' {
			buf[pos] = '\''
			buf[pos+1] = '\''
			pos += 2
		} else {
			buf[pos] = c
			pos++
		}
	}

	return buf[:pos]
}

// escapeStringQuotes is similar to escapeBytesQuotes but for string.
func escapeStringQuotes(buf []byte, v string) []byte {
	pos := len(buf)
	buf = reserveBuffer(buf, len(v)*2)

	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '\'' {
			buf[pos] = '\''
			buf[pos+1] = '\''
			pos += 2
		} else {
			buf[pos] = c
			pos++
		}
	}

	return buf[:pos]
}

func appendDateTime(buf []byte, t time.Time) ([]byte, error) {
	nsec := t.Nanosecond()
	// to round under microsecond
	if nsec%1000 >= 500 { // save half of time.Time.Add calls
		t = t.Add(500 * time.Nanosecond)
		nsec = t.Nanosecond()
	}
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	micro := nsec / 1000

	if year < 1 || year > 9999 {
		return buf, errors.New("year is not in the range [1, 9999]: " + strconv.Itoa(year)) // use errors.New instead of fmt.Errorf to avoid year escape to heap
	}
	year100 := year / 100
	year1 := year % 100

	var localBuf [26]byte // does not escape
	localBuf[0], localBuf[1], localBuf[2], localBuf[3] = digits10[year100], digits01[year100], digits10[year1], digits01[year1]
	localBuf[4] = '-'
	localBuf[5], localBuf[6] = digits10[month], digits01[month]
	localBuf[7] = '-'
	localBuf[8], localBuf[9] = digits10[day], digits01[day]

	if hour == 0 && min == 0 && sec == 0 && micro == 0 {
		return append(buf, localBuf[:10]...), nil
	}

	localBuf[10] = ' '
	localBuf[11], localBuf[12] = digits10[hour], digits01[hour]
	localBuf[13] = ':'
	localBuf[14], localBuf[15] = digits10[min], digits01[min]
	localBuf[16] = ':'
	localBuf[17], localBuf[18] = digits10[sec], digits01[sec]

	if micro == 0 {
		return append(buf, localBuf[:19]...), nil
	}

	micro10000 := micro / 10000
	micro100 := (micro / 100) % 100
	micro1 := micro % 100
	localBuf[19] = '.'
	localBuf[20], localBuf[21], localBuf[22], localBuf[23], localBuf[24], localBuf[25] =
		digits10[micro10000], digits01[micro10000], digits10[micro100], digits01[micro100], digits10[micro1], digits01[micro1]

	return append(buf, localBuf[:]...), nil
}

func interpolateParams(query string, args []interface{}) (string, error) {
	// Number of ? should be same to len(args)
	if strings.Count(query, "?") != len(args) {
		return "", ErrInvalidArgs
	}

	var buf []byte
	buf = buf[:0]
	argPos := 0

	for i := 0; i < len(query); i++ {
		q := strings.IndexByte(query[i:], '?')
		if q == -1 {
			buf = append(buf, query[i:]...)
			break
		}
		buf = append(buf, query[i:i+q]...)
		i += q

		arg := args[argPos]
		argPos++

		if arg == nil {
			buf = append(buf, "NULL"...)
			continue
		}

		switch v := arg.(type) {
		case int:
			buf = strconv.AppendInt(buf, int64(v), 10)
		case int64:
			buf = strconv.AppendInt(buf, v, 10)
		case uint64:
			// Handle uint64 explicitly because our custom ConvertValue emits unsigned values
			buf = strconv.AppendUint(buf, v, 10)
		case float64:
			buf = strconv.AppendFloat(buf, v, 'g', -1, 64)
		case bool:
			if v {
				buf = append(buf, '1')
			} else {
				buf = append(buf, '0')
			}
		case time.Time:
			if v.IsZero() {
				buf = append(buf, "'0000-00-00'"...)
			} else {
				buf = append(buf, '\'')
				var err error
				buf, err = appendDateTime(buf, v.In(time.UTC))
				if err != nil {
					return "", err
				}
				buf = append(buf, '\'')
			}
		case json.RawMessage:
			buf = append(buf, '\'')
			buf = escapeBytesQuotes(buf, v)
			buf = append(buf, '\'')
		case []byte:
			if v == nil {
				buf = append(buf, "NULL"...)
			} else {
				buf = append(buf, "_binary'"...)
				buf = escapeBytesQuotes(buf, v)

				buf = append(buf, '\'')
			}
		case string:
			buf = append(buf, '\'')
			buf = escapeStringQuotes(buf, v)
			buf = append(buf, '\'')
		default:
			serr := fmt.Sprintf("argpos %d unkown type %+v", argPos, v)
			return "", errors.New(serr)
		}
	}
	if argPos != len(args) {
		return "", ErrInvalidParseArgs
	}
	return string(buf), nil
}
