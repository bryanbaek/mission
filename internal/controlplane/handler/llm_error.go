package handler

import (
	"errors"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

func publicLLMUnavailableError() error {
	return errors.New(llm.UserUnavailableMessage)
}
