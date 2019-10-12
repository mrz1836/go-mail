package logger

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// msgQueue message queue
type msgQueue struct {
	messagesToSend chan *bytes.Buffer
}

// Enqueue enqueue the message
func (m *msgQueue) Enqueue(msg *bytes.Buffer) {
	m.messagesToSend <- msg
}

// PushFront push to front
func (m *msgQueue) PushFront(msg *bytes.Buffer) {
	messages := []*bytes.Buffer{msg}
	for {
		select {
		case msg := <-m.messagesToSend:
			messages = append(messages, msg)
		default:
			for _, msg := range messages {
				m.messagesToSend <- msg
			}
			return
		}
	}
}

// logEntries configuration
type logEntries struct {
	conn       *net.TCPConn
	messages   msgQueue
	retryDelay time.Duration
	token      string
}

// NewLogEntriesClient new client
func NewLogEntriesClient(token string) (*logEntries, error) {
	l := &logEntries{
		token:      token,
		retryDelay: RetryDelay,
	}
	l.messages.messagesToSend = make(chan *bytes.Buffer, 1000)

	if err := l.Connect(); err != nil {
		return l, err
	}

	return l, nil
}

// Connect connect to Log Entries
func (l *logEntries) Connect() error {
	if l.conn != nil {
		_ = l.conn.Close() // close the connection, don't care about the error
	}
	l.conn = nil

	addr, err := net.ResolveTCPAddr("tcp", LogEntriesURL+":"+LogEntriesPort)
	if err != nil {
		return err
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		l.retryDelay *= 2
		if l.retryDelay > MaxRetryDelay {
			l.retryDelay = MaxRetryDelay
		}
		return err
	}

	_ = conn.SetNoDelay(true)
	//if err != nil {
	//	return err
	//}
	_ = conn.SetKeepAlive(true)
	//if err != nil {
	//	return err
	//}

	l.conn = conn
	l.retryDelay = RetryDelay

	return nil
}

// ProcessQueue process the queue
func (l *logEntries) ProcessQueue() {
	for msg := range l.messages.messagesToSend {
		if l.conn == nil {
			l.messages.PushFront(msg)
			time.Sleep(l.retryDelay)
			if err := l.Connect(); err != nil {
				log.Println("failed reconnecting to logEntries", err)
				continue
			}
		}
		if _, err := l.conn.Write(msg.Bytes()); err != nil {
			l.messages.PushFront(msg)
			log.Println("failed to write to logEntries", err)
			time.Sleep(l.retryDelay)
			if err := l.Connect(); err != nil {
				log.Println("failed reconnecting to logEntries after failing to write", err)
				continue
			}
		}
	}
}

// write writing the data
func (l *logEntries) write(data string) {
	var buff bytes.Buffer
	buff.WriteString(l.token)
	buff.WriteByte(' ')
	buff.WriteString(data)
	l.messages.Enqueue(&buff)
}

// Println over writes built-in method
func (l *logEntries) Println(v ...interface{}) {
	l.write(fmt.Sprintln(v...))
}

// Printf over writes built-in method
func (l *logEntries) Printf(format string, v ...interface{}) {
	l.write(fmt.Sprintf(format, v...))
}

// Fatalln over writes built-in method
func (l *logEntries) Fatalln(v ...interface{}) {
	var buff bytes.Buffer
	buff.WriteString(l.token)
	buff.WriteByte(' ')
	buff.WriteString(fmt.Sprintln(v...))
	_ = l.sendOne(&buff)
	os.Exit(1)
}

// Fatalf over writes built-in method
func (l *logEntries) Fatalf(format string, v ...interface{}) {
	var buff bytes.Buffer
	buff.WriteString(l.token)
	buff.WriteByte(' ')
	buff.WriteString(fmt.Sprintf(format, v...))
	_ = l.sendOne(&buff)
	os.Exit(1)
}

// sendOne sends one log
func (l *logEntries) sendOne(msg *bytes.Buffer) (err error) {
	if l.conn == nil {
		if err = l.Connect(); err != nil {
			log.Println(msg.String())
			log.Println("failed reconnecting to logEntries", err)
			return
		}
	}
	if _, err = l.conn.Write(msg.Bytes()); err != nil {
		log.Println(msg.String())
		log.Println("failed to write to logEntries", err)
		return
	}
	return
}
