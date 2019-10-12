/*
Package logger is an easy to use, super fast and extendable logging package for Go
*/
package logger

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Logger interface describes the functionality that a log service must implement
type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
	Fatalln(v ...interface{})
	Fatalf(format string, v ...interface{})
}

// KeyValue key value for errors
type KeyValue interface {
	Key() string
	Value() interface{}
}

// LogLevel our log level
type LogLevel uint8

// String turn log level to string
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	}
	return ""
}

// Global constants
const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// logPkg is the log package interface
type logPkg struct{}

// implementation is the current implementation of Logger
var implementation Logger

// init function (different services)
func init() {

	// Detect env var
	logEntriesToken := os.Getenv("LOG_ENTRIES_TOKEN")

	// Do we have a Log Entries token?
	if len(logEntriesToken) > 0 {
		log.Println("go-logger: Log Entries token detected")
		var err error
		implementation, err = NewLogEntriesClient(logEntriesToken)
		if err != nil {
			log.Printf("go-logger: failed to eager connect to Log Entries: %s", err.Error())
		} else {
			log.Println("go-logger: Log Entries connection started")
			go implementation.(*logEntries).ProcessQueue()
		}
	} else { // Basic implementation for local logging
		log.Println("go-logger: internal logging")
		implementation = &logPkg{}
	}
}

// SetImplementation allows the log implementation to be swapped at runtime
func SetImplementation(impl Logger) {
	implementation = impl
}

// FileTag tag file
func FileTag(level int) string {
	comps := FileTagComponents(level + 1)
	return strings.Join(comps, ":")
}

// FileTagComponents file tag components
func FileTagComponents(level int) []string {
	pc, file, line, _ := runtime.Caller(level)
	path := strings.Split(file, "/")
	fn := runtime.FuncForPC(pc)
	methodPath := strings.Split(fn.Name(), "/")
	return []string{strings.Join(path[len(path)-2:], "/"), methodPath[len(methodPath)-1], strconv.Itoa(line)}
}

// Println calls Output to print to the connected logger.
// Arguments are handled in the manner of fmt.Println.
func Println(v ...interface{}) {
	values := []interface{}{FileTag(2)}
	values = append(values, v...)
	implementation.Println(values...)
}

// Printf calls Output to print to the connected logger.
// Arguments are handled in the manner of fmt.Printf.
func Printf(format string, v ...interface{}) {
	implementation.Printf(FileTag(2)+" "+format, v...)
}

// Fatalln is equivalent to Println() followed by a call to os.Exit(1)
func Fatalln(v ...interface{}) {
	values := []interface{}{FileTag(2)}
	values = append(values, v...)
	implementation.Fatalln(values...)
}

// Fatalf is equivalent to Printf() followed by a call to os.Exit(1)
func Fatalf(format string, v ...interface{}) {
	implementation.Fatalf(FileTag(2)+" "+format, v...)
}

// Errorln is equivalent to Println() except the stack level can be set to
// generate the correct log tag. A stack level of 2 is will tag the log with the
// location from where Errorln is called, and is equivalent to Println.
// Larger numbers step further back in the stack
func Errorln(stackLevel int, v ...interface{}) {
	values := []interface{}{FileTag(stackLevel)}
	values = append(values, v...)
	implementation.Println(values...)
}

// Errorfmt is equivalent to Printf with a custom stack level, see Errorln for details
func Errorfmt(stackLevel int, format string, v ...interface{}) {
	implementation.Printf(FileTag(stackLevel)+" "+format, v...)
}

// Data will format the log message to a standardized log entries compatible
// format. stackLevel 2 will tag the log with the location from where Data is
// called. This will print using the implementation's Println function
func Data(stackLevel int, logLevel LogLevel, message string, args ...KeyValue) {
	var buf bytes.Buffer
	comps := FileTagComponents(stackLevel)
	buf.WriteString(`type="`)
	buf.WriteString(strings.ToLower(logLevel.String()))
	buf.WriteString(`" file="`)
	buf.WriteString(comps[0])
	buf.WriteString(`" method="`)
	buf.WriteString(comps[1])
	buf.WriteString(`" line="`)
	buf.WriteString(comps[2])
	buf.WriteString(`" message="`)
	buf.WriteString(message)
	buf.WriteString(`"`)

	for _, arg := range args {
		buf.WriteByte(' ')
		buf.WriteString(arg.Key())
		buf.WriteString(`="`)
		buf.WriteString(fmt.Sprint(arg.Value()))
		buf.WriteByte('"')
	}

	implementation.Println(buf.String())
}

// Println print line
func (l *logPkg) Println(v ...interface{}) {
	log.Println(v...)
}

// Printf print sprint line
func (l *logPkg) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// Fatalln fatal line
func (l *logPkg) Fatalln(v ...interface{}) {
	log.Fatalln(v...)
}

// Fatalf fatal sprint line
func (l *logPkg) Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}
