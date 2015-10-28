// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package errs

type errorType string

// Source: RFC6241 section 4.3
const (
	// TypeNone indicates that the error type is not defined
	TypeNone errorType = "none"
	// TypeTransport indicates that the layer is Secure Transport
	TypeTransport errorType = "transport"
	// TypeRPC indicates that the layer is Messages
	TypeRPC errorType = "rpc"
	// TypeProtocol indicates that the layer is Operations
	TypeProtocol errorType = "protocol"
	// TypeApplication indicates that the layer is Content
	TypeApplication errorType = "application"
)

type errorTag string

// Source: RFC6241 Appendix A
const (
	// TagNone indicates that the error condition is not defined
	TagNone errorTag = "none"
	// TagInUse indicates that the request requires a resource that already is in use
	TagInUse errorTag = "in-use"
	// TagInvalidValue indicates that the request specifies an unacceptable value for one
	// or more parameters
	TagInvalidValue errorTag = "invalid-value"
	// TagTooBig indicates that the request or response (that would be generated) is
	// too large for the implementation to handle
	TagTooBig errorTag = "too-big"
	// TagMissingAttribute indicates that an expected attribute is missing
	TagMissingAttribute errorTag = "missing-attribute"
	// TagBadAttribute indicates that an attribute value is not correct; e.g., wrong type,
	// out of range, pattern mismatch
	TagBadAttribute errorTag = "bad-attribute"
	// TagUnknownAttribute indicates that an unexpected attribute is present
	TagUnknownAttribute errorTag = "unknown-attribute"
	// TagMissingElement indicates that an expected element is missing
	TagMissingElement errorTag = "missing-element"
	// TagBadElement indicates that an element value is not correct; e.g., wrong type,
	// out of range, pattern mismatch
	TagBadElement errorTag = "bad-element"
	// TagUnknownElement indicates that an unexpected element is present
	TagUnknownElement errorTag = "unknown-element"
	// TagUnknownNamespace indicates that an unexpected namespace is present
	TagUnknownNamespace errorTag = "unknown-namespace"
	// TagAccessDenied indicates that access to the requested protocol operation or
	// data model is denied because authorization failed
	TagAccessDenied errorTag = "access-denied"
	// TagLockDenied indicates that access to the requested lock is denied because the
	// lock is currently held by another entity
	TagLockDenied errorTag = "lock-denied"
	// TagResourceDenied indicates that the request could not be completed because of
	// insufficient resources
	TagResourceDenied errorTag = "resource-denied"
	// TagRollbackFailed indicates that the request to roll back some configuration change
	// (via rollback-on-error or <discard-changes> operations) was not completed for some reason
	TagRollbackFailed errorTag = "rollback-failed"
	// TagDataExists indicates that the request could not be completed because the relevant
	// data model content already exists.  For example, a 'create' operation was attempted
	// on data that already exists
	TagDataExists errorTag = "data-exists"
	// TagDataMissing indicates that the request could not be completed because the relevant
	// data model content does not exist.  For example, a 'delete' operation was attempted on
	// data that does not exist
	TagDataMissing errorTag = "data-missing"
	// TagOperationNotSupported indicates that the request could not be completed because
	// the requested operation is not supported by this implementation
	TagOperationNotSupported errorTag = "operation-not-supported"
	// TagOperationFailed indicates that the request could not be completed because the
	// requested operation failed for some reason not covered by any other error condition
	TagOperationFailed errorTag = "operation-failed"
	// TagMalformedMessage indicates that a message could not be handled because it failed to
	// be parsed correctly.
	TagMalformedMessage errorTag = "malformed-message"
)

type errorSeverity string

// Source: RFC6241 section 4.3
const (
	// SevNone indicates that the severity is not set
	SevNone errorSeverity = "none"
	// SevError indicates that the severity is error level
	SevError errorSeverity = "error"
	// SevWarning indicates that the severity is warning level
	SevWarning errorSeverity = "warning"
)

type errorInfoType string

// Source: RFC6241 Appendix A
const (
	ETypeNone         errorInfoType = "none"
	ETypeSessionID    errorInfoType = "session-id"
	ETypeBadAttribute errorInfoType = "bad-attribute"
	ETypeBadElement   errorInfoType = "bad-element"
	ETypeOkElement    errorInfoType = "ok-element"
	ETypeErrElement   errorInfoType = "err-element"
	ETypeNoopElement  errorInfoType = "noop-element"
)

// NetconfError defines a custom error struct
type NetconfError struct {
	// Type defines the conceptual layer that the error occurred in.
	Type []errorType `json:"error-type" xml:"error-type"`
	// Tag contains a string identifying the error condition
	Tag errorTag `json:"error-tag" xml:"error-tag"`
	// Severity contains a string identifying the error severity
	Severity errorSeverity `json:"error-severity" xml:"error-severity"`
	// AppTag contains a string identifying the data-model-specific
	// or implementation-specific error condition, if one exists
	AppTag string `json:"error-app-tag" xml:"error-app-tag"`
	// Path contains the absolute XPath expression identifying the element path to the node
	// that is associated with the error being reported
	Path string `json:"error-path" xml:"error-path"`
	// Message contains a string suitable for human display that describes the error condition
	Message string `json:"error-message" xml:"error-message"`
	// Info contains protocol- or data-model-specific error content
	Info map[errorInfoType][]string `json:"error-info" xml:"error-info"`
	// eDescription describes the error being reported
	Description string `json:"error-description" xml:"error-description"`
}

func (e NetconfError) Error() string {
	return e.Message
}

// AddType adds an errorType to the slice 'Type' in NetconfError
func (e *NetconfError) AddType(eType errorType) {
	e.Type = append(e.Type, eType)
}

// AddErrorInfo adds string data for an errorInfoType to the map 'Info' in NetconfError
func (e *NetconfError) AddErrorInfo(eInfoType errorInfoType, data string) {
	e.Info[eInfoType] = append(e.Info[eInfoType], data)
}

// New creates a new NetconfError err with the provided message.
func New(message string) *NetconfError {
	err := &NetconfError{
		Type:        []errorType{},
		Tag:         TagNone,
		Severity:    SevNone,
		AppTag:      "",
		Path:        "",
		Message:     message,
		Info:        map[errorInfoType][]string{},
		Description: "",
	}
	return err
}

// IsNetconfError allows receivers of a generic error to see if it's one of the
// NetconfError error types
func IsNetconfError(e error) (ok bool) {
	_, ok = e.(*NetconfError)
	return
}
