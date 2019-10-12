package logger

import (
	"encoding/json"
)

// Parameter is a standardized parameter struct for logger.Data()
type Parameter struct {
	K string      `json:"key"`
	V interface{} `json:"value"`
}

// Key implements the Logger KeyValue interface
func (p *Parameter) Key() string {
	return p.K
}

// Value implements the Logger KeyValue interface
func (p *Parameter) Value() interface{} {
	return p.V
}

// Parameter json encodes the parameter into standard key=>value JSON
func (p *Parameter) String() string {
	data, _ := json.Marshal(p) // disregard error
	return string(data)
}

// MakeParameter creates a new Parameter (key/value)
func MakeParameter(key string, value interface{}) *Parameter {
	return &Parameter{
		K: key,
		V: value,
	}
}
