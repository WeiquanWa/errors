// Package errors provides simple error handling primitives.
//
// The traditional error handling idiom in Go is roughly akin to
//
//	if err != nil {
//	        return err
//	}
//
// which when applied recursively up the call stack results in error reports
// without context or debugging information. The errors package allows
// programmers to add context to the failure path in their code in a way
// that does not destroy the original value of the error.
//
// # Adding context to an error
//
// The errors.Wrap function returns a new error that adds context to the
// original error by recording a stack trace at the point Wrap is called,
// together with the supplied message. For example
//
//	_, err := ioutil.ReadAll(r)
//	if err != nil {
//	        return errors.Wrap(err, "read failed")
//	}
//
// If additional control is required, the errors.WithStack and
// errors.WithMessage functions destructure errors.Wrap into its component
// operations: annotating an error with a stack trace and with a message,
// respectively.
//
// # Retrieving the cause of an error
//
// Using errors.Wrap constructs a stack of errors, adding context to the
// preceding error. Depending on the nature of the error it may be necessary
// to reverse the operation of errors.Wrap to retrieve the original error
// for inspection. Any error value which implements this interface
//
//	type causer interface {
//	        Cause() error
//	}
//
// can be inspected by errors.Cause. errors.Cause will recursively retrieve
// the topmost error that does not implement causer, which is assumed to be
// the original cause. For example:
//
//	switch err := errors.Cause(err).(type) {
//	case *MyError:
//	        // handle specifically
//	default:
//	        // unknown error
//	}
//
// Although the causer interface is not exported by this package, it is
// considered a part of its stable public interface.
//
// # Formatted printing of errors
//
// All error values returned from this package implement fmt.Formatter and can
// be formatted by the fmt package. The following verbs are supported:
//
//	%s    print the error. If the error has a Cause it will be
//	      printed recursively.
//	%v    see %s
//	%+v   extended format. Each Frame of the error's StackTrace will
//	      be printed in detail.
//
// # Retrieving the stack trace of an error or wrapper
//
// New, Errorf, Wrap, and Wrapf record a stack trace at the point they are
// invoked. This information can be retrieved with the following interface:
//
//	type stackTracer interface {
//	        StackTrace() errors.StackTrace
//	}
//
// The returned errors.StackTrace type is defined as
//
//	type StackTrace []Frame
//
// The Frame type represents a call site in the stack trace. Frame supports
// the fmt.Formatter interface that can be used for printing information about
// the stack trace of this error. For example:
//
//	if err, ok := err.(stackTracer); ok {
//	        for _, f := range err.StackTrace() {
//	                fmt.Printf("%+s:%d\n", f, f)
//	        }
//	}
//
// Although the stackTracer interface is not exported by this package, it is
// considered a part of its stable public interface.
//
// See the documentation for Frame.Format for more details.
package errors

import (
	"fmt"
	"io"
)

// New returns an error with the supplied message.
// New also records the stack trace at the point it was called.
func New(message string) *MsgCodeErr {
	return &MsgCodeErr{
		msg:   message,
		code:  ErrCodeNotDefined,
		stack: callers(),
	}
}

// Errorf formats according to a format specifier and returns the string
// as a value that satisfies error.
// Errorf also records the stack trace at the point it was called.
func Errorf(format string, args ...interface{}) *MsgCodeErr {
	return &MsgCodeErr{
		msg:   fmt.Sprintf(format, args...),
		stack: callers(),
	}
}

// MsgCodeErr is an error that has a message and a stack, but no caller.
type MsgCodeErr struct {
	code int
	msg  string
	*stack
}

// MsgCodeErr implements the error interface.
func (f *MsgCodeErr) Error() string { return f.msg }

// Format implements fmt.Formatter.
func (f *MsgCodeErr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, f.msg)
			f.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, f.msg)
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", f.msg)
	}
}

// Code returns the error code.
func (f *MsgCodeErr) Code() int { return f.code }

// SetCode sets the error code.
func (f *MsgCodeErr) SetCode(code int) error {
	f.code = code
	return f
}

// WithStack annotates err with a stack trace at the point WithStack was called.
// If err is nil, WithStack returns nil.
func WithStack(err error) *StackError {
	if err == nil {
		return nil
	}
	return &StackError{
		err,
		callers(),
	}
}

type StackError struct {
	error
	*stack
}

// Cause returns the underlying cause of the error
func (w *StackError) Cause() error { return w.error }

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *StackError) Unwrap() error { return w.error }

// Format implements fmt.Formatter.
func (w *StackError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "%+v", w.Cause())
			w.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, w.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", w.Error())
	}
}

// Code returns the error code, if defined.
func (w *StackError) Code() int {
	if err, ok := w.error.(interface{ Code() int }); ok {
		return err.Code()
	}
	return 0
}

// SetCode sets the error code, if defined.
func (w *StackError) SetCode(code int) error {
	if err, ok := w.error.(interface{ SetCode(int) error }); ok {
		_ = err.SetCode(code)
	}
	return w
}

// Wrap returns an error annotating err with a stack trace
// at the point Wrap is called, and the supplied message.
// If err is nil, Wrap returns nil.
func Wrap(err error, message string) *StackError {
	if err == nil {
		return nil
	}
	err = &CauseMsgCodeError{
		cause: err,
		msg:   message,
		code:  ErrCodeNotDefined,
	}
	return &StackError{
		err,
		callers(),
	}
}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is called, and the format specifier.
// If err is nil, Wrapf returns nil.
func Wrapf(err error, format string, args ...interface{}) *StackError {
	if err == nil {
		return nil
	}
	err = &CauseMsgCodeError{
		cause: err,
		msg:   fmt.Sprintf(format, args...),
		code:  ErrCodeNotDefined,
	}
	return &StackError{
		err,
		callers(),
	}
}

// WithMessage annotates err with a new message.
// If err is nil, WithMessage returns nil.
func WithMessage(err error, message string) *CauseMsgCodeError {
	if err == nil {
		return nil
	}
	return &CauseMsgCodeError{
		cause: err,
		msg:   message,
		code:  ErrCodeFailed,
	}
}

// WithMessagef annotates err with the format specifier.
// If err is nil, WithMessagef returns nil.
func WithMessagef(err error, format string, args ...interface{}) *CauseMsgCodeError {
	if err == nil {
		return nil
	}
	return &CauseMsgCodeError{
		cause: err,
		msg:   fmt.Sprintf(format, args...),
		code:  ErrCodeFailed,
	}
}

type CauseMsgCodeError struct {
	cause error
	code  int
	msg   string
}

// MsgCodeErr implements the error interface.
func (w *CauseMsgCodeError) Error() string { return w.msg + ": " + w.cause.Error() }

// Cause returns the underlying cause of the error.
func (w *CauseMsgCodeError) Cause() error { return w.cause }

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *CauseMsgCodeError) Unwrap() error { return w.cause }

// Format implements fmt.Formatter.
func (w *CauseMsgCodeError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "%+v\n", w.Cause())
			_, _ = io.WriteString(s, w.msg)
			return
		}
		fallthrough
	case 's', 'q':
		_, _ = io.WriteString(s, w.Error())
	}
}

// Code returns the error code.
func (w *CauseMsgCodeError) Code() int { return w.code }

// SetCode sets the error code.
func (w *CauseMsgCodeError) SetCode(code int) error {
	w.code = code
	return w
}

// Cause returns the underlying cause of the error, if possible.
// An error value has a cause if it implements the following
// interface:
//
//	type causer interface {
//	       Cause() error
//	}
//
// If the error does not implement Cause, the original error will
// be returned. If the error is nil, nil will be returned without further
// investigation.
func Cause(err error) error {
	type causer interface {
		Cause() error
	}

	for err != nil {
		cause, ok := err.(causer)
		if !ok {
			break
		}
		err = cause.Cause()
	}
	return err
}
