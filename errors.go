// Copyright (C) 2013 Space Monkey, Inc.

package errors

import (
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "runtime"
    "strings"
    "syscall"
)

var (
    stackLogSize = flag.Int("errors.stack_trace_log_length", 4096,
        "The max stack trace byte length to log")
    stackCaptureSize = flag.Int("errors.stack_trace_capture_length", 2048,
        "The max stack trace byte length to capture")
)

type ErrorClassFlags uint64

const (
    LogOnCreation ErrorClassFlags = 1 << iota
    CaptureStack
)

type ErrorClass struct {
    parent *ErrorClass
    name   string
    flags  ErrorClassFlags
}

var (
    // base error classes. To construct your own error class, use New.
    SystemError = &ErrorClass{
        parent: nil,
        name:   "System Error"}
    HierarchicalError = &ErrorClass{
        parent: nil,
        name:   "Error",
        flags:  CaptureStack}
)

// NewSpecified creates an error class for making specific errors. Regardless
// of where the error class is in the error class hierarchy, the error class
// flags for this error class are final, and no other context is used to
// determine the final operating set.
func NewSpecified(ec *ErrorClass, name string, flags ErrorClassFlags) *ErrorClass {
    if ec == nil {
        ec = HierarchicalError
    }
    return &ErrorClass{parent: ec, name: name, flags: flags}
}

// NewWith creates an error class for making specific errors. NewWith takes the
// parent's error class flags, appends them to the provided flags, and
// configures the new error class to use them.
func NewWith(ec *ErrorClass, name string, flags_to_add ErrorClassFlags) *ErrorClass {
    if ec == nil {
        ec = HierarchicalError
    }
    return &ErrorClass{parent: ec, name: name, flags: ec.flags | flags_to_add}
}

// NewWithout creates an error class for making specific errors. NewWithout
// takes the parent's error class flags, ensures the provided flags are
// stripped, and configures the new error class to use the resulting set.
func NewWithout(ec *ErrorClass, name string, flags_to_remove ErrorClassFlags) *ErrorClass {
    if ec == nil {
        ec = HierarchicalError
    }
    return &ErrorClass{parent: ec, name: name, flags: ec.flags & ^flags_to_remove}
}

// New is like NewWith or NewWithout without any flags provided.
func New(ec *ErrorClass, name string) *ErrorClass {
    if ec == nil {
        ec = HierarchicalError
    }
    return &ErrorClass{parent: ec, name: name, flags: ec.flags}
}

func (e *ErrorClass) Parent() *ErrorClass {
    return e.parent
}

func (e *ErrorClass) String() string {
    return e.name
}

func (e *ErrorClass) Is(parent *ErrorClass) bool {
    for check := e; check != nil; check = check.parent {
        if check == parent {
            return true
        }
    }
    return false
}

type Error struct {
    err   error
    class *ErrorClass
    stack []byte
}

func (e *ErrorClass) Wrap(err error, classes ...*ErrorClass) error {
    if err == nil {
        return nil
    }
    if ec, ok := err.(*Error); ok {
        if ec.Is(e) {
            return err
        }
        for _, class := range classes {
            if ec.Is(class) {
                return err
            }
        }
    }
    rv := &Error{err: err, class: e}
    if e.flags&CaptureStack > 0 {
        buf := make([]byte, *stackCaptureSize)
        rv.stack = buf[:runtime.Stack(buf, false)]
    }
    if e.flags&LogOnCreation > 0 {
        LogWithStack(rv.Error())
    }
    return rv
}

func (e *ErrorClass) New(format string, args ...interface{}) error {
    return e.Wrap(fmt.Errorf(format, args...))
}

func (e *Error) Error() string {
    message := strings.TrimRight(e.err.Error(), "\n ")
    if strings.Contains(message, "\n") {
        message = fmt.Sprintf("%s:\n  %s", e.class.String(),
            strings.Replace(message, "\n", "\n  ", -1))
    } else {
        message = fmt.Sprintf("%s: %s", e.class.String(), message)
    }
    if e.stack == nil {
        return message
    }
    return fmt.Sprintf("%s\n\n%s backtrace: %s", message, e.class.String(), e.stack)
}

func (e *Error) WrappedErr() error {
    return e.err
}

func (e *Error) Class() *ErrorClass {
    return e.class
}

func (e *Error) Stack() []byte {
    return e.stack
}

func WrappedErr(err error) error {
    cast, ok := err.(*Error)
    if !ok {
        return err
    }
    return cast.WrappedErr()
}

func GetClass(err error) *ErrorClass {
    cast, ok := err.(*Error)
    if !ok {
        return findSystemErrorClass(err)
    }
    return cast.Class()
}

func (e *Error) Is(ec *ErrorClass) bool {
    return e.class.Is(ec)
}

func (e *ErrorClass) Contains(err error) bool {
    return GetClass(err).Is(e)
}

func LogWithStack(message string) {
    buf := make([]byte, *stackLogSize)
    buf = buf[:runtime.Stack(buf, false)]
    log.Printf("%s\n%s", message, buf)
}

var (
    // useful error classes
    NotImplementedError = NewWith(nil, "Not Implemented Error", LogOnCreation)
    ProgrammerError     = NewWith(nil, "Programmer Error", LogOnCreation)

    // classes we fake

    // from os
    SyscallError = New(SystemError, "Syscall Error")

    // from syscall
    ErrnoError = New(SystemError, "Errno Error")

    // from net
    NetworkError        = New(SystemError, "Network Error")
    UnknownNetworkError = New(NetworkError, "Unknown Network Error")
    AddrError           = New(NetworkError, "Addr Error")
    InvalidAddrError    = New(AddrError, "Invalid Addr Error")
    NetOpError          = New(NetworkError, "Network Op Error")
    NetParseError       = New(NetworkError, "Network Parse Error")
    DNSError            = New(NetworkError, "DNS Error")
    DNSConfigError      = New(DNSError, "DNS Config Error")
)

func findSystemErrorClass(err error) *ErrorClass {
    switch err.(type) {
    case *os.SyscallError:
        return SyscallError
    case syscall.Errno:
        return ErrnoError
    case net.UnknownNetworkError:
        return UnknownNetworkError
    case *net.AddrError:
        return AddrError
    case net.InvalidAddrError:
        return InvalidAddrError
    case *net.OpError:
        return NetOpError
    case *net.ParseError:
        return NetParseError
    case *net.DNSError:
        return DNSError
    case *net.DNSConfigError:
        return DNSConfigError
    case net.Error:
        return NetworkError
    default:
        return SystemError
    }
}
