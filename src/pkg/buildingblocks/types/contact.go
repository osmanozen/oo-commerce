package types

import (
	"errors"
	"regexp"
	"strings"
)

// emailRegex is compiled once at package level for performance.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Email is an immutable value object representing a validated email address.
type Email struct {
	value string
}

// NewEmail creates a validated Email value object.
func NewEmail(email string) (Email, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return Email{}, errors.New("email cannot be empty")
	}
	if !emailRegex.MatchString(trimmed) {
		return Email{}, errors.New("invalid email format")
	}
	return Email{value: strings.ToLower(trimmed)}, nil
}

// String returns the email address.
func (e Email) String() string {
	return e.value
}

// phoneRegex validates E.164 format: +[country code][number] (7-15 digits).
var phoneRegex = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

// PhoneNumber is an immutable value object representing a phone number in E.164 format.
type PhoneNumber struct {
	value string
}

// NewPhoneNumber creates a validated PhoneNumber value object.
func NewPhoneNumber(phone string) (PhoneNumber, error) {
	trimmed := strings.TrimSpace(phone)
	if trimmed == "" {
		return PhoneNumber{}, errors.New("phone number cannot be empty")
	}
	if !phoneRegex.MatchString(trimmed) {
		return PhoneNumber{}, errors.New("phone number must be in E.164 format (e.g., +1234567890)")
	}
	return PhoneNumber{value: trimmed}, nil
}

// String returns the phone number.
func (p PhoneNumber) String() string {
	return p.value
}
