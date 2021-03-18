package apoco

import "log"

var enableLog = false

// SetLog enables or disables logging.
func SetLog(enable bool) {
	enableLog = enable
}

// Log logs the given message if logging is enabled.
func Log(f string, args ...interface{}) {
	if enableLog {
		log.Printf(f, args...)
	}
}
