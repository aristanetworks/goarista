// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package errs

import (
	"fmt"
	"net/http"
)

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
	// TagPartialOperation not supported because it is obselete and SHOULD NOT be sent by
	// servers conforming to RFC6241

	// TagMalformedMessage indicates that a message could not be handled because it failed to
	// be parsed correctly.
	// This error-tag is new in :base:1.1 and MUST NOT be sent to old clients.
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
	ETypeSessionID    errorInfoType = "session-id"
	ETypeBadAttribute errorInfoType = "bad-attribute"
	ETypeBadElement   errorInfoType = "bad-element"
	ETypeBadNamespace errorInfoType = "bad-namespace"
)

// NetconfError defines a custom error struct
type NetconfError struct {
	// Type defines the conceptual layer that the error occurred in.
	Type errorType `json:"error-type" xml:"error-type"`
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
	Info map[errorInfoType]string `json:"error-info" xml:"error-info"`
	// eDescription describes the error being reported
	Description string `json:"error-description" xml:"error-description"`
}

func (e *NetconfError) Error() string {
	return e.Message
}

// NewInUse creates the Netconf error of this type. Valid error-type for this type of error is
// protocol or application
func NewInUse(resourceInUse string, eType errorType) *NetconfError {
	return &NetconfError{
		Type:        eType,
		Tag:         TagInUse,
		Severity:    SevError,
		Message:     fmt.Sprintf("Resource %q is already in use", resourceInUse),
		Info:        map[errorInfoType]string{},
		Description: "The request requires a resource that already is in use.",
	}
}

// NewInvalidValue creates the Netconf error of this type. Valid error-type for this type of
// error is protocol or application
func NewInvalidValue(invalidParam string, invalidValue string, eType errorType) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagInvalidValue,
		Severity: SevError,
		Message: fmt.Sprintf("Parameter %q has invalid value: %s", invalidParam,
			invalidValue),
		Info: map[errorInfoType]string{},
		Description: "The request specifies an unacceptable value for one or more " +
			"parameters.",
	}
}

// NewTooBig creates the Netconf error of this type. All four error-types are valid for this error
func NewTooBig(eType errorType) *NetconfError {
	description := "The request or response (that would be generated) is too large for " +
		"the implementation to handle."
	return &NetconfError{
		Type:        eType,
		Tag:         TagTooBig,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// NewMissingAttribute creates the Netconf error of this type.
// Except transport error-type, other three error-types are valid for this error
func NewMissingAttribute(eType errorType, attrName string, elemName string) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagMissingAttribute,
		Severity: SevError,
		Message: fmt.Sprintf("Expected attribute %q is missing from element %q",
			attrName, elemName),
		Info: map[errorInfoType]string{
			// name of the missing attribute
			ETypeBadAttribute: attrName,
			// name of the element that is supposed to contain the missing attribute
			ETypeBadElement: elemName,
		},
		Description: "An expected attribute is missing.",
	}
}

// NewBadAttribute creates the Netconf error of this type.
// Except transport error-type, other three error-types are valid for this error
func NewBadAttribute(eType errorType, attrName string, elemName string) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagBadAttribute,
		Severity: SevError,
		Message: fmt.Sprintf("Element %q contains a bad value for attribute %q",
			attrName, elemName),
		Info: map[errorInfoType]string{
			// name of the attribute with bad value
			ETypeBadAttribute: attrName,
			// name of the element that contains the attribute with the bad value
			ETypeBadElement: elemName,
		},
		Description: "An attribute value is not correct; e.g., wrong type, out of range, " +
			"pattern mismatch.",
	}
}

// NewUnknownAttribute creates the Netconf error of this type.
// Except transport error-type, other three error-types are valid for this error
func NewUnknownAttribute(eType errorType, attrName string, elemName string) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagUnknownAttribute,
		Severity: SevError,
		Message: fmt.Sprintf("Element %q contains an unknown attribute %q",
			elemName, attrName),
		Info: map[errorInfoType]string{
			// name of the unexpected attribute
			ETypeBadAttribute: attrName,
			// name of the element that contains the unexpected attribute
			ETypeBadElement: elemName,
		},
		Description: "An unexpected attribute is present.",
	}
}

// NewMissingElement creates the Netconf error of this type. Valid error-type for this type of
// error is protocol or application
func NewMissingElement(eType errorType, elemName string) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagMissingElement,
		Severity: SevError,
		Message:  fmt.Sprintf("Expected element %q is missing", elemName),
		Info: map[errorInfoType]string{
			// name of the missing element
			ETypeBadElement: elemName,
		},
		Description: "An expected element is missing.",
	}
}

// NewBadElement creates the Netconf error of this type. Valid error-type for this type of
// error is protocol or application
func NewBadElement(eType errorType, elemName string) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagBadElement,
		Severity: SevError,
		Message:  fmt.Sprintf("Bad Value present for element %q", elemName),
		Info: map[errorInfoType]string{
			// name of the element with bad value
			ETypeBadElement: elemName,
		},
		Description: "An element value is not correct; e.g., wrong type, out of range, " +
			"pattern mismatch.",
	}
}

// NewUnknownElement creates the Netconf error of this type. Valid error-type for this type of
// error is protocol or application
func NewUnknownElement(eType errorType, elemName string) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagUnknownElement,
		Severity: SevError,
		Message:  fmt.Sprintf("An unexpected element %q is present", elemName),
		Info: map[errorInfoType]string{
			// name of the unexpected element
			ETypeBadElement: elemName,
		},
		Description: "An unexpected element is present.",
	}
}

// NewUnknownNamespace creates the Netconf error of this type. Valid error-type for this type of
// error is protocol or application
func NewUnknownNamespace(eType errorType, elemName string, namespace string) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagUnknownNamespace,
		Severity: SevError,
		Message: fmt.Sprintf("An unexpected namespace %q is present in the element %q",
			namespace, elemName),
		Info: map[errorInfoType]string{
			// name of the element that contains the unexpected namespace
			ETypeBadElement: elemName,
			// name of the unexpected namespace
			ETypeBadNamespace: namespace,
		},
		Description: "An unexpected namespace is present.",
	}
}

// NewAccessDenied creates the Netconf error of this type. Valid error-type for this type of
// error is protocol or application
func NewAccessDenied(eType errorType) *NetconfError {
	return &NetconfError{
		Type:     eType,
		Tag:      TagAccessDenied,
		Severity: SevError,
		Message:  fmt.Sprintf("Authorization denied"),
		Info:     map[errorInfoType]string{},
		Description: "Access to the requested protocol operation or data model is denied " +
			"because authorization failed.",
	}
}

// NewLockDenied creates the Netconf error of this type. Valid error-type for this type of
// error is protocol
func NewLockDenied(sessionID string) *NetconfError {
	return &NetconfError{
		Type:     TypeProtocol,
		Tag:      TagLockDenied,
		Severity: SevError,
		Message:  fmt.Sprintf("Lock already held by session ID %s", sessionID),
		Info: map[errorInfoType]string{
			ETypeSessionID: sessionID,
		},
		Description: "Access to the requested lock is denied because the lock is " +
			"currently held by another entity.",
	}
}

// NewResourceDenied creates the Netconf error of this type.
// All four error-types are valid for this error.
func NewResourceDenied(eType errorType) *NetconfError {
	description := "Request could not be completed because of insufficient resources."
	return &NetconfError{
		Type:        eType,
		Tag:         TagResourceDenied,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// NewRollbackFailed creates the Netconf error of this type. Valid error-type for this type of
// error is protocol or application
func NewRollbackFailed(eType errorType) *NetconfError {
	description := "Request to roll back some configuration change (via rollback-on-error " +
		"or <discard-changes> operations) was not completed for some reason."
	return &NetconfError{
		Type:        eType,
		Tag:         TagRollbackFailed,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// NewDataExists creates the Netconf error of this type. Valid error-type for this type of
// error is application
func NewDataExists() *NetconfError {
	description := "Request could not be completed because the relevant data model " +
		"content already exists.  For example, a 'create' operation was attempted " +
		"on data that already exists."
	return &NetconfError{
		Type:        TypeApplication,
		Tag:         TagDataExists,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// NewDataMissing creates the Netconf error of this type. Valid error-type for this type of
// error is application
func NewDataMissing() *NetconfError {
	description := "Request could not be completed because the relevant data model " +
		"content does not exist.  For example, a 'delete' operation was " +
		"attempted on data that does not exist."
	return &NetconfError{
		Type:        TypeApplication,
		Tag:         TagDataMissing,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// NewOperationNotSupported creates the Netconf error of this type. Valid error-type for
// this type of error is protocol or application
func NewOperationNotSupported(eType errorType) *NetconfError {
	description := "Request could not be completed because the requested operation " +
		"is not supported by this implementation."
	return &NetconfError{
		Type:        eType,
		Tag:         TagOperationNotSupported,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// NewOperationFailed creates the Netconf error of this type.
// Except transport error-type, other three error-types are valid for this error
func NewOperationFailed(eType errorType) *NetconfError {
	description := "Request could not be completed because the requested operation " +
		"failed for some reason not covered by any other error condition."
	return &NetconfError{
		Type:        eType,
		Tag:         TagOperationFailed,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// NewMalformedMessage creates the Netconf error of this type.
// Valid error-type for this type of error is rpc
func NewMalformedMessage() *NetconfError {
	description := "A message could not be handled because it failed to be parsed " +
		"correctly.  For example, the message is not well-formed XML or it uses " +
		"an invalid character set."
	return &NetconfError{
		Type:        TypeRPC,
		Tag:         TagMalformedMessage,
		Severity:    SevError,
		Message:     description,
		Info:        map[errorInfoType]string{},
		Description: description,
	}
}

// IsNetconfError allows receivers of a generic error to see if it's one of the
// NetconfError error types
func IsNetconfError(e error) (ok bool) {
	_, ok = e.(*NetconfError)
	return
}

// MapTagToHTTPStatusCode maps the netconf error-tag to http status code as per
// draft-ietf-netconf-restconf-07#section-7
func MapTagToHTTPStatusCode(e *NetconfError) int {
	switch e.Tag {
	case TagInUse, TagLockDenied, TagResourceDenied, TagDataExists, TagDataMissing:
		return http.StatusConflict
	case TagInvalidValue, TagMissingAttribute, TagBadAttribute, TagUnknownAttribute,
		TagMissingElement, TagBadElement, TagUnknownElement, TagUnknownNamespace,
		TagMalformedMessage:
		return http.StatusBadRequest
	case TagTooBig:
		return http.StatusRequestEntityTooLarge
	case TagAccessDenied:
		return http.StatusForbidden
	case TagRollbackFailed, TagOperationFailed:
		return http.StatusInternalServerError
	case TagOperationNotSupported:
		return http.StatusNotImplemented
	default:
		return 0
	}
}
