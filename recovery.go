package yogin

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime"
	"strings"
	"time"
)

var DefaultErrorWriter io.Writer = os.Stderr

// RecoveryFunc defines the function passable to CustomRecovery.
type RecoveryFunc func(c *Context, err interface{})

func defaultHandleRecovery(c *Context, err interface{}) {
	c.AbortWithStatus(http.StatusInternalServerError)
}

// Recovery returns a middleware that recovers from any panics and writes a 500 if there was one.
func Recovery() HandlerFunc {
	logger := log.New(DefaultErrorWriter, "\n\n\x1b[31m", log.LstdFlags)

	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				brokenPipe := false
				if ne, ok := err.(*net.OpError); ok {
					var se *os.SyscallError
					if errors.As(ne, &se) {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				stack := stack(3)
				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				headersToStr := string(httpRequest)
				if brokenPipe {
					logger.Printf("%s\n%s%s", err, headersToStr, reset)
				} else {
					logger.Printf("[Recovery] %s panic recovered:\n%s\n%s\n%s%s",
						time.Now().Format("2006/01/02 - 15:04:05"), headersToStr, err, stack, reset)
				}

				if brokenPipe {
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error))
					c.Abort()
				} else {
					defaultHandleRecovery(c, err)
				}
			}
		}()

		c.Next()
	}
}

// stack returns a nicely formatted stack frame, skipping skip frames.
func stack(skip int) string {
	var pcs [32]uintptr
	n := runtime.Callers(skip, pcs[:])

	var b strings.Builder
	b.WriteString("Traceback:")

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		b.WriteString(fmt.Sprintf("\n\t%s:%d", frame.File, frame.Line))
		if !more {
			break
		}
	}
	return b.String()
}
