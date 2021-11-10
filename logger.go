package yogin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[97;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

var DefaultWriter io.Writer = os.Stdout

func Logger() HandlerFunc {
	formatter := defaultLogFormatter
	out := DefaultWriter

	return func(c *Context) {
		start := time.Now() // Start timer

		c.Next() // Process request

		param := LogFormatterParams{
			Request: c.Request,
			Path: c.Path,
		}

		param.TimeStamp = time.Now() // Stop timer
		param.Latency = param.TimeStamp.Sub(start)

		param.ClientIP = c.ClientIP
		param.Method = c.Request.Method
		param.StatusCode = c.statusCode
		param.ErrorMessage = errorMessage(c.Errors)

		param.BodySize = c.bodySize

		fmt.Fprint(out, formatter(param))
	}
}

type LogFormatter func(params LogFormatterParams) string

type LogFormatterParams struct {
	Request *http.Request

	// TimeStamp shows the time after the server returns a response.
	TimeStamp time.Time
	// StatusCode is HTTP response code.
	StatusCode int
	// Latency is how much time the server cost to process a certain request.
	Latency time.Duration
	// ClientIP equals Context's ClientIP method.
	ClientIP string
	// Method is the HTTP method given to the request.
	Method string
	// Path is a path the client requests.
	Path string
	// ErrorMessage is set if error has occurred in processing the request.
	ErrorMessage string
	// BodySize is the size of the Response Body
	BodySize int
}

func (p *LogFormatterParams) StatusCodeColor() string {
	code := p.StatusCode

	switch {
	case code >= http.StatusOK && code < http.StatusMultipleChoices:
		return green
	case code >= http.StatusMultipleChoices && code < http.StatusBadRequest:
		return white
	case code >= http.StatusBadRequest && code < http.StatusInternalServerError:
		return magenta
	default:
		return red
	}
}

func (p *LogFormatterParams) MethodColor() string {
	method := p.Method

	switch method {
	case http.MethodGet:
		return blue
	case http.MethodPost:
		return cyan
	default:
		return reset
	}
}

func (p *LogFormatterParams) ResetColor() string {
	return reset
}

var defaultLogFormatter = func(param LogFormatterParams) string {
	statusColor := param.StatusCodeColor()
	methodColor := param.MethodColor()
	resetColor := param.ResetColor()

	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}
	return fmt.Sprintf("[YOGIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v %d\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		param.Path,
		param.BodySize,
		param.ErrorMessage,
	)
}

func errorMessage(errors []error) string {
	if len(errors) == 0 {
		return ""
	}
	var b strings.Builder
	for i, err := range errors {
		fmt.Fprintf(&b, "Error #%02d: %s\n", i+1, err)
	}
	return b.String()
}
