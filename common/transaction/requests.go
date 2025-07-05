package transaction

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type EnqueueTransactionRequest struct {
	Host    string `json:"host"`
	Path    string `json:"path"`
	Method  string `json:"method"`
	Payload string `json:"payload"`
}

func ValidRequest(request EnqueueTransactionRequest) error {
	var errs []error
	if request.Host == "" {
		errs = append(errs, errors.New("host is required"))
	}
	if request.Path == "" {
		errs = append(errs, errors.New("path is required"))
	}
	if request.Method == "" {
		errs = append(errs, errors.New("method is required"))
	}
	if !isValidHTTPMethod(request.Method) {
		errs = append(errs, fmt.Errorf("method %s is invalid", request.Method))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func isValidHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodPatch, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return true
	default:
		return false
	}
}
