package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
)

// NVL returns def if str is null
func NVL(str string, def string) string {
	if len(str) == 0 {
		return def
	}
	return str
}

// PrettyPrint prints an interface as a pretty JSON document
func PrettyPrint(v interface{}) (err error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		fmt.Println(string(b))
	}
	return
}

// NewError returns an error given a reason
func NewError(reason string) (err error) {
	return errors.New(reason)
}

// ReadFile given base path and file name
func ReadFile(basePath string, fileName string) (string, error) {
	filePath := basePath + "/" + fileName
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
