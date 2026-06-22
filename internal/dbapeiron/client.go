package dbapeiron

import (
	"context"
	"fmt"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn

	Cache         apeironv1.CacheServiceClient
	Creatures     apeironv1.CreatureDataServiceClient
	Inventory     apeironv1.InventoryDataServiceClient
	Observability apeironv1.ObservabilityServiceClient
	Players       apeironv1.PlayerDataServiceClient
	Profiles      apeironv1.ProfileDataServiceClient
	Skills        apeironv1.SkillDataServiceClient
	World         apeironv1.WorldDataServiceClient
}

func Connect(ctx context.Context, cfg config.DBApeironConfig) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("db-apeiron endpoint is required")
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	conn, err := grpc.DialContext(
		connectCtx,
		cfg.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect db-apeiron: %w", err)
	}

	return NewClient(conn), nil
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		conn:          conn,
		Cache:         apeironv1.NewCacheServiceClient(conn),
		Creatures:     apeironv1.NewCreatureDataServiceClient(conn),
		Inventory:     apeironv1.NewInventoryDataServiceClient(conn),
		Observability: apeironv1.NewObservabilityServiceClient(conn),
		Players:       apeironv1.NewPlayerDataServiceClient(conn),
		Profiles:      apeironv1.NewProfileDataServiceClient(conn),
		Skills:        apeironv1.NewSkillDataServiceClient(conn),
		World:         apeironv1.NewWorldDataServiceClient(conn),
	}
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) CheckReadiness(ctx context.Context, cfg config.DBApeironConfig) error {
	if c == nil || c.Observability == nil {
		return fmt.Errorf("db-apeiron observability client is not initialized")
	}

	reqCtx, cancel := requestContext(ctx, cfg)
	defer cancel()

	resp, err := c.Observability.GetReadiness(reqCtx, &apeironv1.Empty{})
	if err != nil {
		return fmt.Errorf("db-apeiron readiness request failed: %w", err)
	}

	if resp.GetReadiness().GetStatus() != "READY" {
		return fmt.Errorf("db-apeiron not ready: %s", resp.GetReadiness().GetMessage())
	}

	return nil
}
