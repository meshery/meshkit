//go:build !js

package utils

import (
	"fmt"

	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checks if the error is of type kubeerror.StatusError
func IsErrKubeStatusErr(err error) bool {
	switch err.(type) {
	case *kubeerror.StatusError:
		return true
	default:
		return false
	}
}

// handleStatusReason processes the high-level reason for the error and generates appropriate messaging
func handleStatusReason(reason v1.StatusReason) (probableCause, remedy string) {
	switch reason {
	case v1.StatusReasonUnauthorized:
		return "User authentication failed or authentication credentials were not provided",
			"Ensure you have provided valid authentication credentials and they have not expired"

	case v1.StatusReasonForbidden:
		return "The server understood the request but refuses to authorize it",
			"Verify you have the necessary permissions to perform this operation"

	case v1.StatusReasonNotFound:
		return "The requested resource does not exist on the server",
			"Check if the resource name and namespace are correct, and the resource exists"

	case v1.StatusReasonAlreadyExists:
		return "The resource you are trying to create already exists",
			"Either use a different name for your resource or update the existing resource instead"

	case v1.StatusReasonConflict:
		return "The requested operation conflicts with an existing resource or operation",
			"Retrieve the latest state of the resource and retry your operation"

	case v1.StatusReasonGone:
		return "The requested resource is no longer available",
			"The resource has been deleted or moved. Update your configuration to reference existing resources"

	case v1.StatusReasonInvalid:
		return "The provided resource specification is invalid",
			"Review the resource specification and correct any validation errors"

	case v1.StatusReasonServerTimeout:
		return "The server timed out while processing the request",
			"The server is temporarily unable to handle the request. Try again later"

	case v1.StatusReasonTimeout:
		return "The operation could not be completed within the specified time",
			"Consider increasing timeout values or retry the operation"

	case v1.StatusReasonTooManyRequests:
		return "Too many requests are being sent to the server",
			"Reduce the rate of requests or wait before retrying"

	case v1.StatusReasonBadRequest:
		return "The request was invalid or cannot be served",
			"Review and correct the format of your request"

	case v1.StatusReasonMethodNotAllowed:
		return "The requested operation is not supported",
			"Verify that the operation is valid for this type of resource"

	case v1.StatusReasonInternalError:
		return "An internal error occurred while processing the request",
			"This is a server-side issue. Contact your cluster administrator if the problem persists"

	case v1.StatusReasonExpired:
		return "The requested resource has expired",
			"The resource needs to be recreated or refreshed"

	case v1.StatusReasonServiceUnavailable:
		return "The service is currently unavailable",
			"The server is temporarily unable to handle requests. Try again later"

	default:
		return "An unexpected error occurred while processing the request",
			"Review the error details and ensure your request is valid"
	}
}

// handleStatusCause processes specific validation errors and field-level issues
func handleStatusCause(cause v1.StatusCause, kind string) (probableCause, remedy string) {
	switch cause.Type {
	case v1.CauseTypeFieldValueNotFound:
		return fmt.Sprintf("The specified value for field '%s' was not found", cause.Field),
			fmt.Sprintf("Ensure the value referenced in field '%s' exists before creating this resource", cause.Field)

	case v1.CauseTypeFieldValueRequired:
		return fmt.Sprintf("Required field '%s' was not provided in the %s specification", cause.Field, kind),
			fmt.Sprintf("Add the required field '%s' to your %s manifest", cause.Field, kind)

	case v1.CauseTypeFieldValueDuplicate:
		return fmt.Sprintf("Duplicate value found for field '%s'", cause.Field),
			fmt.Sprintf("Ensure the value for field '%s' is unique", cause.Field)

	case v1.CauseTypeFieldValueInvalid:
		return fmt.Sprintf("Invalid value provided for field '%s': %s", cause.Field, cause.Message),
			fmt.Sprintf("Correct the value for field '%s' according to the validation requirements", cause.Field)

	case v1.CauseTypeUnexpectedServerResponse:
		return "The server returned an unexpected response",
			"This is likely a server-side issue. Contact your cluster administrator"

	default:
		return fmt.Sprintf("Issue with field '%s': %s", cause.Field, cause.Message),
			"Review and correct the specified field according to the error message."
	}
}

// ParseKubeStatusErr converts Kubernetes API errors into user-friendly messages
func ParseKubeStatusErr(err *kubeerror.StatusError) (shortDescription, longDescription, probableCause, remedy []string) {
	shortDescription = make([]string, 0)
	longDescription = make([]string, 0)
	probableCause = make([]string, 0)
	remedy = make([]string, 0)

	if err == nil {
		return
	}

	status := err.Status()

	// Add the high-level error message with status code to longDescription
	longDescription = append(longDescription, fmt.Sprintf("[Status Code: %d] %s", status.Code, status.Message))

	pc, rem := handleStatusReason(status.Reason)
	probableCause = append(probableCause, pc)
	remedy = append(remedy, rem)

	// Add specific field validation errors
	if status.Details != nil && len(status.Details.Causes) > 0 {
		for _, cause := range status.Details.Causes {
			longDescription = append(longDescription, fmt.Sprintf("Field '%s': %s", cause.Field, cause.Message))

			pc, rem := handleStatusCause(cause, status.Details.Kind)
			probableCause = append(probableCause, pc)
			remedy = append(remedy, rem)
		}
	} else {
		// If no specific causes are provided, add the general reason-based guidance
		pc, rem := handleStatusReason(status.Reason)
		probableCause = append(probableCause, pc)
		remedy = append(remedy, rem)
	}

	return
}
