package opa

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type opaResult struct {
	allowed     bool
	shouldBlock bool
	messages    []string
}

func (r opaResult) Allowed() bool {
	return r.allowed
}

func (r opaResult) ShouldBlock() bool {
	return r.shouldBlock
}

func (r opaResult) Messages() []string {
	return r.messages
}

func ParseOpaResult(bytesResult []byte, opaConfig OpaConfig) (opaResult, error) {
	var mapResult map[string]interface{}
	err := json.Unmarshal(bytesResult, &mapResult)
	if err != nil {
		return opaResult{}, err
	}

	var allowed, shouldBlock, ok bool
	var messages []string

	v, err := getRawValue(opaConfig.ResultAllowedKey, mapResult)
	if err != nil {
		return opaResult{}, err
	}
	if allowed, ok = v.(bool); !ok {
		return opaResult{}, fmt.Errorf("not found allowed key %s from opa result", opaConfig.ResultAllowedKey)
	}

	v, err = getRawValue(opaConfig.ResultShouldBlockKey, mapResult)
	if err != nil {
		return opaResult{}, err
	}
	if shouldBlock, ok = v.(bool); v != nil && !ok {
		return opaResult{}, fmt.Errorf("not found shouldBlock key %s from opa result", opaConfig.ResultShouldBlockKey)
	}

	v, err = getRawValue(opaConfig.ResultMessagesKey, mapResult)
	if err != nil {
		return opaResult{}, err
	}
	if arr, ok := v.([]interface{}); v != nil && !ok {
		return opaResult{}, fmt.Errorf("not found messages key %s from opa result", opaConfig.ResultMessagesKey)
	} else {
		for _, item := range arr {
			switch v := item.(type) {
			case string:
				messages = append(messages, v)
			default:
				return opaResult{}, fmt.Errorf("invalid messages")
			}
		}
	}

	return opaResult{allowed, shouldBlock, messages}, nil
}

func getRawValue(key string, h map[string]interface{}) (interface{}, error) {
	parts := strings.Split(key, ".")
	for i, part := range parts {
		switch v := h[part].(type) {
		case nil:
			return nil, nil
		case map[string]interface{}:
			h = v
		default:
			if i == len(parts)-1 {
				return v, nil
			} else {
				return nil, fmt.Errorf("%s expecting a map but %s found", part, reflect.TypeOf(v))
			}
		}
	}
	return nil, errors.New("should not reach here")
}
