package eval

import (
	"fmt"
	"github.com/PaesslerAG/gval"
	"regexp"
	"strconv"
)

func VarEval(expr string, data interface{}) (interface{}, error) {
	value, err := gval.Evaluate(
		expr,
		data,
		gval.Full(),
		gval.Function("strlen", strlen),
		gval.Function("len", arrlen),
		gval.Function("rematch", rematch),
	)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func strlen(args ...interface{}) (interface{}, error) {
	length := len(args[0].(string))
	return strconv.Itoa(length), nil
}

func arrlen(args ...interface{}) (interface{}, error) {
	length := len(args[0].([]interface{}))
	return strconv.Itoa(length), nil
}

func rematch(args ...interface{}) (interface{}, error) {
	str := args[0].(string)
	expr := args[1].(string)
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	result := re.FindAllStringSubmatch(str, 1)
	if result == nil {
		return nil, fmt.Errorf("no match")
	}
	return result[0][1], nil
}
