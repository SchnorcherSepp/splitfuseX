package fuse

import (
	"fmt"
	"log"
	"log/syslog"
)

const LOGERROR = "ERROR"
const LOGINFO = "INFO"

var sysl *syslog.Writer

func init() {

	var err error

	sysl, err = syslog.New(syslog.LOG_ERR, "splitfuseX")
	if err != nil {
		panic(err)
	}
}

func debug(debug bool, lvl string, msg string, err error) {
	msg = fmt.Sprintf("%s: %s: %v\n", lvl, msg, err)

	// console
	if debug {
		log.Printf("%s\n", msg)
	}

	// syslog (immer)
	if lvl == LOGINFO {
		sysl.Info(msg)
	} else {
		sysl.Err(msg)
	}
}
