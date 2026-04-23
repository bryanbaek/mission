package handler

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/auth"
)

func requireUser(ctx context.Context) (auth.User, error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return auth.User{}, unauthenticatedError()
	}
	return user, nil
}

func unauthenticatedError() error {
	return connect.NewError(
		connect.CodeUnauthenticated,
		errors.New("unauthenticated"),
	)
}

func parseConnectUUID(value, field string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid %s", field),
		)
	}
	return parsed, nil
}
