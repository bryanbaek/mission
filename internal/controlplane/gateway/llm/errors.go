package llm

import (
	"errors"
	"fmt"
)

type ProviderError struct {
	Provider  string
	Err       error
	Transient bool
}

func (e *ProviderError) Error() string {
	switch {
	case e.Provider == "":
		return e.Err.Error()
	case e.Err == nil:
		return fmt.Sprintf("%s provider error", e.Provider)
	default:
		return fmt.Sprintf("%s provider error: %v", e.Provider, e.Err)
	}
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

func NewProviderError(provider string, err error) error {
	if err == nil {
		return nil
	}
	return &ProviderError{
		Provider: provider,
		Err:      err,
	}
}

func NewTransientProviderError(provider string, err error) error {
	if err == nil {
		return nil
	}
	return &ProviderError{
		Provider:  provider,
		Err:       err,
		Transient: true,
	}
}

func WrapProviderError(provider string, err error) error {
	if err == nil {
		return nil
	}

	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		if providerErr.Provider == "" {
			return &ProviderError{
				Provider:  provider,
				Err:       providerErr.Err,
				Transient: providerErr.Transient,
			}
		}
		return err
	}

	return NewProviderError(provider, err)
}

func IsTransientProviderError(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Transient
}

type UnavailableError struct {
	Providers []string
	Err       error
}

func (e *UnavailableError) Error() string {
	return UserUnavailableMessage
}

func (e *UnavailableError) Unwrap() error {
	return e.Err
}

func NewUnavailableError(providers []string, err error) error {
	return &UnavailableError{
		Providers: append([]string(nil), providers...),
		Err:       err,
	}
}

func IsUnavailableError(err error) bool {
	var unavailableErr *UnavailableError
	return errors.As(err, &unavailableErr)
}
