package out

import (
	"fmt"
	"log"
)

var Silent = false

func Outf(format string, args ...interface{}) {
	if Silent {
		return
	}
	fmt.Printf(format, args...)
}

func Outln(args ...interface{}) {
	if Silent {
		return
	}
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
