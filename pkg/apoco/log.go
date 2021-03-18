package apoco

import "log"

var enableLog = false

// SetLog enables or disables logging.  This function is not safe for
// concurrent usage and should be used once at application start.
func SetLog(enable bool) {
	enableLog = enable
}

// Log logs the given message if logging is enabled.  This function
// uses log.Printf for logging, so it is save to be used concurrently.
func Log(f string, args ...interface{}) {
	if enableLog {
		log.Printf(f, args...)
	}
}
