package out

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/logrusorgru/aurora"
)

var loggerOut = log.New(os.Stderr, "", 0)
var loggerDebug = log.New(ioutil.Discard, "", 0)

func Reinit(debug bool) {
	if debug {
		loggerOut = log.New(os.Stderr, aurora.Magenta(" [USER] ").String(), log.LstdFlags)
		loggerDebug = log.New(os.Stderr, "[DEBUG] ", log.LstdFlags)
	} else {
		loggerOut = log.New(os.Stderr, "", 0)
		loggerDebug = log.New(ioutil.Discard, "", 0)
	}
}

func Outf(format string, args ...interface{}) {
	loggerOut.Printf(format, args...)
}

func Outln(args ...interface{}) {
	loggerOut.Println(args...)
}

func Debugf(format string, args ...interface{}) {
	loggerDebug.Printf(format, args...)
}

func Debugln(args ...interface{}) {
	loggerDebug.Println(args...)
}
