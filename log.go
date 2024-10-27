package main

import (
	"fmt"
	"path"
	"runtime"
)

func getCallerInfo(skip int) (info string) {

	pc, file, lineNo, ok := runtime.Caller(skip)
	if !ok {

		info = "runtime.Caller() failed"
		return
	}
	funcName := runtime.FuncForPC(pc).Name()
	fileName := path.Base(file) // The Base function returns the last element of the path
	return fmt.Sprintf("FuncName:%s, file:%s, line:%d ", funcName, fileName, lineNo)
}
