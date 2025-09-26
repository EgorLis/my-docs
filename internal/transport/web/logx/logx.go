package logx

import (
	"fmt"
	"log"
)

// Info/Error — единый стиль логов: key=value, с op и req_id

func Info(l *log.Logger, reqID, op, msg string, kv ...any) {
	l.Printf("lvl=info op=%s req_id=%s msg=%q %s", op, reqID, msg, joinKV(kv...))
}

func Error(l *log.Logger, reqID, op, msg string, err error, kv ...any) {
	l.Printf("lvl=error op=%s req_id=%s msg=%q err=%q %s", op, reqID, msg, err, joinKV(kv...))
}

func joinKV(kv ...any) string {
	if len(kv) == 0 {
		return ""
	}
	out := ""
	for i := 0; i+1 < len(kv); i += 2 {
		out += toKV(kv[i], kv[i+1]) + " "
	}
	return out
}

func toKV(k, v any) string { return fmt.Sprintf("%v=%v", k, v) }
