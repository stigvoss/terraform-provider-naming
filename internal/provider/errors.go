package provider

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ArgError ties an error to the function argument that caused it, so a bad
// static value is reported against the scheme and a bad call value against the
// variables.
type ArgError struct {
	Arg int64
	Err error
}

func (e *ArgError) Error() string { return e.Err.Error() }
func (e *ArgError) Unwrap() error { return e.Err }

func argErr(arg int64, format string, a ...any) error {
	return &ArgError{Arg: arg, Err: fmt.Errorf(format, a...)}
}

// argOf returns the argument an error belongs to, or def if it carries none.
func argOf(err error, def int64) int64 {
	var ae *ArgError
	if errors.As(err, &ae) {
		return ae.Arg
	}
	return def
}

// withScheme adds the scheme's template to a message, so an error can be
// matched to a definition when many schemes are in use. Messages that already
// quote the template are left alone.
func withScheme(err error, template string) string {
	msg := err.Error()
	if strings.Contains(msg, strconv.Quote(template)) {
		return msg
	}
	return fmt.Sprintf("%s (scheme %q)", msg, template)
}

// label names the rules an error came from, or "" when there are none.
func (r Rules) label() string {
	if r.name == "" {
		return ""
	}
	return fmt.Sprintf(" in rules for %q", r.name)
}

// chars formats a character count with the right plural.
func chars(n int) string {
	if n == 1 {
		return "1 character"
	}
	return fmt.Sprintf("%d characters", n)
}
