package scw

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/scaleway/scaleway-sdk-go/internal/errors"
)

// SdkError is a base interface for all Scaleway SDK errors.
type SdkError interface {
	Error() string
	IsScwSdkError()
}

// ResponseError is an error type for the Scaleway API
type ResponseError struct {
	// Message is a human-friendly error message
	Message string `json:"message"`

	// Type is a string code that defines the kind of error. This field is only used by instance API
	Type string `json:"type,omitempty"`

	// Resource is a string code that defines the resource concerned by the error. This field is only used by instance API
	Resource string `json:"resource,omitempty"`

	// Fields contains detail about validation error. This field is only used by instance API
	Fields map[string][]string `json:"fields,omitempty"`

	// StatusCode is the HTTP status code received
	StatusCode int `json:"-"`

	// Status is the HTTP status received
	Status string `json:"-"`

	RawBody json.RawMessage `json:"-"`
}

// IsScwSdkError implement SdkError interface
func (e *ResponseError) IsScwSdkError() {}
func (e *ResponseError) Error() string {
	s := fmt.Sprintf("scaleway-sdk-go: http error %s", e.Status)

	if e.Resource != "" {
		s = fmt.Sprintf("%s: resource %s", s, e.Resource)
	}

	if e.Message != "" {
		s = fmt.Sprintf("%s: %s", s, e.Message)
	}

	if len(e.Fields) > 0 {
		s = fmt.Sprintf("%s: %v", s, e.Fields)
	}

	return s
}
func (e *ResponseError) GetRawBody() json.RawMessage {
	return e.RawBody
}

// hasResponseError throws an error when the HTTP status is not OK
func hasResponseError(res *http.Response) SdkError {
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return nil
	}

	newErr := &ResponseError{
		StatusCode: res.StatusCode,
		Status:     res.Status,
	}

	if res.Body == nil {
		return newErr
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "cannot read error response body")
	}
	newErr.RawBody = body

	err = json.Unmarshal(body, newErr)
	if err != nil {
		return errors.Wrap(err, "could not parse error response body")
	}

	stdErr := unmarshalStandardError(newErr.Type, body)
	if stdErr != nil {
		return stdErr
	}

	return newErr
}

func unmarshalStandardError(errorType string, body []byte) SdkError {
	var stdErr SdkError

	switch errorType {
	case "invalid_arguments":
		stdErr = &InvalidArgumentsError{RawBody: body}
	case "quotas_exceeded":
		stdErr = &QuotasExceededError{RawBody: body}
	case "transient_state":
		stdErr = &TransientStateError{RawBody: body}
	case "not_found":
		stdErr = &ResourceNotFoundError{RawBody: body}
	case "permissions_denied":
		stdErr = &PermissionsDeniedError{RawBody: body}
	case "out_of_stock":
		stdErr = &OutOfStockError{RawBody: body}
	default:
		return nil
	}

	err := json.Unmarshal(body, stdErr)
	if err != nil {
		return errors.Wrap(err, "could not parse error %s response body", errorType)
	}

	return stdErr
}

type InvalidArgumentsError struct {
	Details []struct {
		ArgumentName string `json:"argument_name"`
		Reason       string `json:"reason"`
		HelpMessage  string `json:"help_message"`
	} `json:"details"`

	RawBody json.RawMessage `json:"-"`
}

// IsScwSdkError implements the SdkError interface
func (e *InvalidArgumentsError) IsScwSdkError() {}
func (e *InvalidArgumentsError) Error() string {
	invalidArgs := make([]string, len(e.Details))
	for i, d := range e.Details {
		invalidArgs[i] = d.ArgumentName
		switch d.Reason {
		case "unknown":
			invalidArgs[i] += " is invalid for unexpected reason"
		case "required":
			invalidArgs[i] += " is required"
		case "format":
			invalidArgs[i] += " is wrongly formatted"
		case "constraint":
			invalidArgs[i] += " does not respect constraint"
		}
		if d.HelpMessage != "" {
			invalidArgs[i] += ", " + d.HelpMessage
		}
	}

	return "scaleway-sdk-go: invalid argument(s): " + strings.Join(invalidArgs, "; ")
}
func (e *InvalidArgumentsError) GetRawBody() json.RawMessage {
	return e.RawBody
}

type QuotasExceededError struct {
	Details []struct {
		Resource string `json:"resource"`
		Quota    uint32 `json:"quota"`
		Current  uint32 `json:"current"`
	} `json:"details"`

	RawBody json.RawMessage `json:"-"`
}

// IsScwSdkError implements the SdkError interface
func (e *QuotasExceededError) IsScwSdkError() {}
func (e *QuotasExceededError) Error() string {
	invalidArgs := make([]string, len(e.Details))
	for i, d := range e.Details {
		invalidArgs[i] = fmt.Sprintf("%s has reached its quota (%d/%d)", d.Resource, d.Current, d.Current)
	}

	return "scaleway-sdk-go: quota exceeded(s): " + strings.Join(invalidArgs, "; ")
}
func (e *QuotasExceededError) GetRawBody() json.RawMessage {
	return e.RawBody
}

type PermissionsDeniedError struct {
	Details []struct {
		Resource string `json:"resource"`
		Action   string `json:"action"`
	} `json:"details"`

	RawBody json.RawMessage `json:"-"`
}

// IsScwSdkError implements the SdkError interface
func (e *PermissionsDeniedError) IsScwSdkError() {}
func (e *PermissionsDeniedError) Error() string {
	invalidArgs := make([]string, len(e.Details))
	for i, d := range e.Details {
		invalidArgs[i] = fmt.Sprintf("%s %s", d.Action, d.Resource)
	}

	return "scaleway-sdk-go: insufficient permissions: " + strings.Join(invalidArgs, "; ")
}
func (e *PermissionsDeniedError) GetRawBody() json.RawMessage {
	return e.RawBody
}

type TransientStateError struct {
	Resource     string `json:"resource"`
	ResourceID   string `json:"resource_id"`
	CurrentState string `json:"current_state"`

	RawBody json.RawMessage `json:"-"`
}

// IsScwSdkError implements the SdkError interface
func (e *TransientStateError) IsScwSdkError() {}
func (e *TransientStateError) Error() string {
	return fmt.Sprintf("scaleway-sdk-go: resource %s with ID %s is in a transient state: %s", e.Resource, e.ResourceID, e.CurrentState)
}
func (e *TransientStateError) GetRawBody() json.RawMessage {
	return e.RawBody
}

type ResourceNotFoundError struct {
	Resource   string `json:"resource"`
	ResourceID string `json:"resource_id"`

	RawBody json.RawMessage `json:"-"`
}

// IsScwSdkError implements the SdkError interface
func (e *ResourceNotFoundError) IsScwSdkError() {}
func (e *ResourceNotFoundError) Error() string {
	return fmt.Sprintf("scaleway-sdk-go: resource %s with ID %s is not found", e.Resource, e.ResourceID)
}
func (e *ResourceNotFoundError) GetRawBody() json.RawMessage {
	return e.RawBody
}

type OutOfStockError struct {
	Resource string `json:"resource"`

	RawBody json.RawMessage `json:"-"`
}

// IsScwSdkError implements the SdkError interface
func (e *OutOfStockError) IsScwSdkError() {}
func (e *OutOfStockError) Error() string {
	return fmt.Sprintf("scaleway-sdk-go: resource %s is out of stock", e.Resource)
}
func (e *OutOfStockError) GetRawBody() json.RawMessage {
	return e.RawBody
}
