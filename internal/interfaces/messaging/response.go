package messaging

import (
	"BookShareWriteApi/internal/exception"
	"errors"
	"log"
)

type Response struct {
	OK    bool       `json:"ok"`
	Data  any        `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func fail(code, msg string) Response {
	return Response{Error: &ErrorBody{Code: code, Message: msg}}
}

func failFrom(err error) Response {
	switch {
	case errors.Is(err, exception.ErrBookAlreadyExists):
		return fail("BOOK_ALREADY_EXISTS", err.Error())
	case errors.Is(err, exception.ErrValidation): // pega todas as regras do domain
		return fail("INVALID_INPUT", err.Error())
	default:
		log.Printf("internal error: %v", err)
		return fail("INTERNAL", "internal error")
	}
}