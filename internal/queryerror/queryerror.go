package queryerror

import "fmt"

type Code string

const (
	CodeUnspecified      Code = "UNSPECIFIED"
	CodePermissionDenied Code = "PERMISSION_DENIED"
	CodeInternal         Code = "INTERNAL"
)

type Error struct {
	Code              Code
	Reason            string
	BlockedConstructs []string
	Err               error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Reason != "" {
		return e.Reason
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "query failed"
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func PermissionDenied(reason string, blockedConstructs []string) *Error {
	return &Error{
		Code:              CodePermissionDenied,
		Reason:            reason,
		BlockedConstructs: append([]string(nil), blockedConstructs...),
	}
}

func Internal(reason string, err error) *Error {
	if reason == "" && err != nil {
		reason = err.Error()
	}
	return &Error{
		Code:   CodeInternal,
		Reason: reason,
		Err:    err,
	}
}

func (c Code) String() string {
	if c == "" {
		return string(CodeUnspecified)
	}
	return string(c)
}

func Format(reason string, blockedConstructs []string) string {
	if len(blockedConstructs) == 0 {
		return reason
	}
	return fmt.Sprintf("%s (%v)", reason, blockedConstructs)
}
