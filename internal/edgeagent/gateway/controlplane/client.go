package controlplane

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/bryanbaek/mission/gen/go/agent/v1"
	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
)

type Client struct {
	client agentv1connect.AgentServiceClient
}

func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		client: agentv1connect.NewAgentServiceClient(
			httpClient,
			baseURL,
			connect.WithInterceptors(&bearerInterceptor{token: token}),
		),
	}
}

func (c *Client) OpenCommandStream(
	ctx context.Context,
	req edgecontroller.OpenCommandStreamRequest,
) (edgecontroller.CommandStream, error) {
	stream, err := c.client.OpenCommandStream(
		ctx,
		connect.NewRequest(&agentv1.OpenCommandStreamRequest{
			SessionId:    req.SessionID,
			Hostname:     req.Hostname,
			AgentVersion: req.AgentVersion,
			StartedAt:    timestamppb.New(req.StartedAt),
		}),
	)
	if err != nil {
		return nil, err
	}
	return &commandStream{stream: stream}, nil
}

func (c *Client) Heartbeat(
	ctx context.Context,
	req edgecontroller.HeartbeatRequest,
) error {
	_, err := c.client.Heartbeat(
		ctx,
		connect.NewRequest(&agentv1.HeartbeatRequest{
			SessionId: req.SessionID,
			SentAt:    timestamppb.New(req.SentAt),
		}),
	)
	return err
}

func (c *Client) SubmitPingResult(
	ctx context.Context,
	req edgecontroller.SubmitPingResultRequest,
) error {
	_, err := c.client.SubmitCommandResult(
		ctx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   req.SessionID,
			CommandId:   req.CommandID,
			CompletedAt: timestamppb.New(req.CompletedAt),
			Result: &agentv1.SubmitCommandResultRequest_Ping{
				Ping: &agentv1.PingResult{RoundTripMs: req.RoundTripMS},
			},
		}),
	)
	return err
}

func (c *Client) SubmitExecuteQueryResult(
	ctx context.Context,
	req edgecontroller.SubmitExecuteQueryResultRequest,
) error {
	result, err := queryResultToProto(req)
	if err != nil {
		return err
	}

	_, err = c.client.SubmitCommandResult(
		ctx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   req.SessionID,
			CommandId:   req.CommandID,
			CompletedAt: timestamppb.New(req.CompletedAt),
			Result: &agentv1.SubmitCommandResultRequest_ExecuteQuery{
				ExecuteQuery: result,
			},
		}),
	)
	return err
}

type commandStream struct {
	stream *connect.ServerStreamForClient[agentv1.ControlMessage]
}

func (s *commandStream) Receive() bool {
	return s.stream.Receive()
}

func (s *commandStream) Message() edgecontroller.ControlMessage {
	msg := s.stream.Msg()
	command := edgecontroller.ControlMessage{
		SessionID: msg.GetSessionId(),
		CommandID: msg.GetCommandId(),
		Kind:      commandKind(msg),
		SQL:       commandSQL(msg),
	}
	if msg.GetIssuedAt() != nil {
		command.IssuedAt = msg.GetIssuedAt().AsTime().UTC()
	}
	return command
}

func (s *commandStream) Err() error {
	return s.stream.Err()
}

type bearerInterceptor struct {
	token string
}

func (b *bearerInterceptor) WrapUnary(
	next connect.UnaryFunc,
) connect.UnaryFunc {
	return func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", "Bearer "+b.token)
		return next(ctx, req)
	}
}

func (b *bearerInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", "Bearer "+b.token)
		return conn
	}
}

func (b *bearerInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return next
}

func commandKind(msg *agentv1.ControlMessage) edgecontroller.CommandKind {
	switch msg.Payload.(type) {
	case *agentv1.ControlMessage_Ping:
		return edgecontroller.CommandKindPing
	case *agentv1.ControlMessage_ExecuteQuery:
		return edgecontroller.CommandKindExecuteQuery
	default:
		return ""
	}
}

func commandSQL(msg *agentv1.ControlMessage) string {
	switch payload := msg.Payload.(type) {
	case *agentv1.ControlMessage_ExecuteQuery:
		return payload.ExecuteQuery.GetSql()
	default:
		return ""
	}
}

func queryResultToProto(
	req edgecontroller.SubmitExecuteQueryResultRequest,
) (*agentv1.ExecuteQueryResult, error) {
	rows := make([]*agentv1.ExecuteQueryRow, 0, len(req.Rows))
	for _, row := range req.Rows {
		values := make(map[string]*structpb.Value, len(row))
		for key, value := range row {
			pbValue, err := structpb.NewValue(value)
			if err != nil {
				return nil, fmt.Errorf("convert row value %q: %w", key, err)
			}
			values[key] = pbValue
		}
		rows = append(rows, &agentv1.ExecuteQueryRow{Values: values})
	}

	return &agentv1.ExecuteQueryResult{
		Columns:      append([]string(nil), req.Columns...),
		Rows:         rows,
		ElapsedMs:    req.ElapsedMS,
		DatabaseUser: req.DatabaseUser,
		DatabaseName: req.DatabaseName,
		Error:        req.Error,
	}, nil
}
