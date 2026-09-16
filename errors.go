package fireimg

import (
	"fmt"
	"net/http"
	"strings"
)

// Error is a FireImg client or API failure.
type Error struct {
	Op      string
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return "fireimg: unknown error"
	}
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		if e.Status > 0 {
			msg = http.StatusText(e.Status)
		} else {
			msg = "unknown error"
		}
	}
	switch {
	case e.Op != "" && e.Status > 0:
		return fmt.Sprintf("fireimg %s: %s (%d)", e.Op, msg, e.Status)
	case e.Op != "":
		return fmt.Sprintf("fireimg %s: %s", e.Op, msg)
	case e.Status > 0:
		return fmt.Sprintf("fireimg: %s (%d)", msg, e.Status)
	default:
		return "fireimg: " + msg
	}
}

func clientError(op, message string) error {
	return &Error{Op: op, Message: message}
}

func statusError(op string, status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &Error{Op: op, Status: status, Message: msg}
}
