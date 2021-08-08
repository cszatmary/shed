// Package errors defines error handling used by shed.
package errors

import (
	stderrors "errors"
	"fmt"
	"strings"
)

// Error represents a shed error.
type Error struct {
	// Kind is the category of error.
	Kind Kind
	// Reason is a human-readable message containing
	// the details of the error.
	Reason string
	// Op is the operation being performed, usually the
	// name of a function or method being invoked.
	Op Op
	// Err is the underlying error that triggered this one.
	// If no underlying error occurred, it will be nil.
	Err error
}

// Op describes an operation, usually a function or method name
// such as "Shed.Get".
type Op string

// Kind identifies the category of an error.
//
// Kind is used to group errors based on how they can be actioned.
type Kind uint8

const (
	Unspecified  Kind = iota // Error that does not fall into any category.
	Invalid                  // Invalid operation on an item.
	NotInstalled             // A tool needs to be installed for the operation to work.
	BadState                 // Shed is in a bad state, but it can be fixed.
	Internal                 // Internal error or inconsistency.
	IO                       // An OS level I/O error.
	Go                       // An error returned from the go command.
)

func (k Kind) String() string {
	switch k {
	case Unspecified:
		return "unspecified error"
	case Invalid:
		return "invalid operation"
	case NotInstalled:
		return "tool not installed"
	case BadState:
		return "bad state"
	case Internal:
		return "internal error"
	case IO:
		return "I/O error"
	case Go:
		return "go error"
	}
	return "unknown error kind"
}

// New creates an error value from its arguments.
// There must be at least one argument or New panics.
// The type of each argument determines what field of Error
// it is assigned to. If an argument has an invalid type New panics.
func New(args ...interface{}) error {
	if len(args) == 0 {
		panic("errors.New called with no arguments")
	}
	e := &Error{}
	for _, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			e.Kind = arg
		case string:
			e.Reason = arg
		case Op:
			e.Op = arg
		case *Error:
			// Make a copy so error chains are immutable.
			copy := *arg
			e.Err = &copy
		case error:
			e.Err = arg
		default:
			panic(fmt.Sprintf("unknown type %T, value %v passed to errors.New", arg, arg))
		}
	}
	return e
}

func (e *Error) Error() string {
	sb := &strings.Builder{}
	if e.Kind != Unspecified {
		pad(sb, ": ")
		sb.WriteString(e.Kind.String())
	}
	if e.Reason != "" {
		pad(sb, ": ")
		sb.WriteString(e.Reason)
	}
	if e.Err != nil {
		pad(sb, ": ")
		sb.WriteString(e.Err.Error())
	}
	return sb.String()
}

func (e *Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		// If '%+v' print a detailed description for debugging purposes.
		if s.Flag('+') {
			sb := &strings.Builder{}
			if e.Op != "" {
				pad(sb, ": ")
				sb.WriteString(string(e.Op))
			}
			if e.Kind != Unspecified {
				pad(sb, ": ")
				sb.WriteString(e.Kind.String())
			}
			if e.Reason != "" {
				pad(sb, ": ")
				sb.WriteString(e.Reason)
			}
			if e.Err != nil {
				if prevErr, ok := e.Err.(*Error); ok {
					pad(sb, ":\n\t")
					fmt.Fprintf(sb, "%+v", prevErr)
				} else {
					pad(sb, ": ")
					sb.WriteString(e.Err.Error())
				}
			}
			fmt.Fprint(s, sb.String())
			return
		}
		fallthrough
	case 's':
		fmt.Fprint(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

// pad appends s to sb if b already has some data.
func pad(sb *strings.Builder, s string) {
	if sb.Len() == 0 {
		return
	}
	sb.WriteString(s)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Root finds the root error in the error chain that is of type *Error.
// It will keep unwrapping errors that have a non-nil Err field.
// If err is not of type *Error or does not wrap an *Error, nil will be returned.
func Root(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if !stderrors.As(err, &e) {
		return nil
	}
	// See if there is another *Error wrapped somewhere
	if re := Root(e.Err); re != nil {
		return re
	}
	return e
}

// List contains multiple errors that occurred while performing an operation.
type List []error

func (e List) Error() string {
	errStrs := make([]string, len(e))
	for i, err := range e {
		errStrs[i] = err.Error()
	}
	return strings.Join(errStrs, "\n")
}

// The following functions are wrappers over the standard library errors package functions.
// This is so that this package can be used exclusively for errors.

// Str is a wrapper over errors.New from the standard library.
func Str(s string) error {
	return stderrors.New(s)
}

// Is is errors.Is from the standard library.
func Is(err, target error) bool {
	return stderrors.Is(err, target)
}
