package logger

import "time"

// Package constants
const (
	LogEntriesPort = "10000"
	LogEntriesURL  = "data.logentries.com"
	MaxRetryDelay  = 2 * time.Minute
	RetryDelay     = 100 * time.Millisecond
)
