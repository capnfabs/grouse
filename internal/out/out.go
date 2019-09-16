package out

import (
	"fmt"
	"log"
)

func Outf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func Outln(args ...interface{}) {
	fmt.Println(args...)
}

var Debug = false

func Debugf(format string, args ...interface{}) {
	if !Debug {
		return
	}
	log.Printf(format, args...)
}

func Debugln(args ...interface{}) {
	if !Debug {
		return
	}
	log.Println(args...)
}
