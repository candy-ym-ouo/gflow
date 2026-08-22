package condition

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func Value(expr string, ctx map[string]any) any {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "$.") {
		v := ctx[strings.TrimPrefix(expr, "$.")]
		return v
	}
	if v, err := strconv.ParseFloat(expr, 64); err == nil {
		return v
	}
	return strings.Trim(expr, "\"")
}
func Eval(expr string, ctx map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" || expr == "true" {
		return true, nil
	}
	for _, op := range []string{" or ", " and "} {
		if p := strings.Index(expr, op); p >= 0 {
			a, e := Eval(expr[:p], ctx)
			if e != nil {
				return false, e
			}
			b, e := Eval(expr[p+len(op):], ctx)
			if e != nil {
				return false, e
			}
			if op == " or " {
				return a || b, nil
			}
			return a && b, nil
		}
	}
	if strings.HasPrefix(expr, "not ") {
		v, e := Eval(expr[4:], ctx)
		return !v, e
	}
	if p := strings.Index(expr, " matches "); p > 0 {
		x := fmt.Sprint(Value(expr[:p], ctx))
		pat := fmt.Sprint(Value(expr[p+9:], ctx))
		return regexp.MatchString(pat, x)
	}
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<"} {
		if p := strings.Index(expr, op); p > 0 {
			a, b := Value(expr[:p], ctx), Value(expr[p+len(op):], ctx)
			af, _ := strconv.ParseFloat(fmt.Sprint(a), 64)
			bf, _ := strconv.ParseFloat(fmt.Sprint(b), 64)
			switch op {
			case "==":
				return fmt.Sprint(a) == fmt.Sprint(b), nil
			case "!=":
				return fmt.Sprint(a) != fmt.Sprint(b), nil
			case ">":
				return af > bf, nil
			case "<":
				return af < bf, nil
			case ">=":
				return af >= bf, nil
			case "<=":
				return af <= bf, nil
			}
		}
	}
	return false, fmt.Errorf("invalid condition: %s", expr)
}

func Lookup(path string, ctx map[string]any) (any, bool) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return ctx, true
	}
	var current any = ctx
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch value := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = value[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false
			}
			current = value[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func Exists(path string, ctx map[string]any) bool { _, ok := Lookup(path, ctx); return ok }

func Empty(path string, ctx map[string]any) bool {
	v, ok := Lookup(path, ctx)
	if !ok || v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

func Function(name string, args []string, ctx map[string]any) (any, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "exists":
		if len(args) != 1 {
			return nil, fmt.Errorf("exists expects one argument")
		}
		return Exists(args[0], ctx), nil
	case "empty":
		if len(args) != 1 {
			return nil, fmt.Errorf("empty expects one argument")
		}
		return Empty(args[0], ctx), nil
	case "len":
		if len(args) != 1 {
			return nil, fmt.Errorf("len expects one argument")
		}
		v, ok := Lookup(args[0], ctx)
		if !ok {
			return 0, nil
		}
		return length(v), nil
	case "contains":
		if len(args) != 2 {
			return nil, fmt.Errorf("contains expects two arguments")
		}
		v, _ := Lookup(args[0], ctx)
		return strings.Contains(fmt.Sprint(v), unquote(args[1])), nil
	case "matches":
		if len(args) != 2 {
			return nil, fmt.Errorf("matches expects two arguments")
		}
		v, _ := Lookup(args[0], ctx)
		return regexp.MatchString(unquote(args[1]), fmt.Sprint(v))
	case "now":
		if len(args) != 0 {
			return nil, fmt.Errorf("now expects no arguments")
		}
		return time.Now(), nil
	case "json":
		if len(args) != 1 {
			return nil, fmt.Errorf("json expects one argument")
		}
		return json.Marshal(args[0])
	default:
		return nil, fmt.Errorf("unknown function %s", name)
	}
}

func length(v any) int {
	switch x := v.(type) {
	case string:
		return len(x)
	case []any:
		return len(x)
	case map[string]any:
		return len(x)
	default:
		return 0
	}
}

func unquote(s string) string { return strings.Trim(strings.TrimSpace(s), "\"") }

func SplitArguments(s string) []string {
	parts := []string{}
	start, depth := 0, 0
	quoted := false
	for i, r := range s {
		switch r {
		case '"':
			quoted = !quoted
		case '(':
			if !quoted {
				depth++
			}
		case ')':
			if !quoted {
				depth--
			}
		case ',':
			if !quoted && depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

func EvaluateValue(expr string, ctx map[string]any) (any, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "$") {
		v, ok := Lookup(expr, ctx)
		if !ok {
			return nil, nil
		}
		return v, nil
	}
	if p := strings.Index(expr, "("); p > 0 && strings.HasSuffix(expr, ")") {
		return Function(expr[:p], SplitArguments(expr[p+1:len(expr)-1]), ctx)
	}
	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
	}
	if f, err := strconv.ParseFloat(expr, 64); err == nil {
		return f, nil
	}
	return unquote(expr), nil
}
