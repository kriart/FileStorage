package repository

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrLimitExceeded = errors.New("limit exceeded")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrInvalidInput  = errors.New("invalid input")
)

type MessageError struct {
	cause   error
	message string
}

func (e MessageError) Error() string {
	if e.message == "" {
		return e.cause.Error()
	}
	return e.message
}

func (e MessageError) Unwrap() error {
	return e.cause
}

func InvalidInput(message string) error {
	return MessageError{cause: ErrInvalidInput, message: message}
}

func LimitExceeded(message string) error {
	return MessageError{cause: ErrLimitExceeded, message: message}
}

func Conflict(message string) error {
	return MessageError{cause: ErrConflict, message: message}
}

func PublicMessage(err error) string {
	var messageErr MessageError
	if errors.As(err, &messageErr) && messageErr.message != "" {
		return messageErr.message
	}

	switch {
	case errors.Is(err, ErrInvalidInput):
		return "Некорректные данные"
	case errors.Is(err, ErrUnauthorized):
		return "Нужно войти в аккаунт"
	case errors.Is(err, ErrForbidden):
		return "Недостаточно прав"
	case errors.Is(err, ErrNotFound):
		return "Объект не найден"
	case errors.Is(err, ErrConflict):
		return "Конфликт данных"
	case errors.Is(err, ErrLimitExceeded):
		return "Превышен лимит"
	default:
		return "Внутренняя ошибка сервера"
	}
}
