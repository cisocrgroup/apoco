package apoco

import "log"

var enableLog = false

// SetLog enables or disables logging.
func SetLog(enable bool) {
	enableLog = enable
}

// L logs the given message if logging is enabled.
func L(f string, args ...interface{}) {
	if enableLog {
		log.Printf(f, args...)
	}
}
