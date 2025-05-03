package dkv

import (
	"context"
)

type Discovery interface {
	Register(ctx context.Context, hostname string, updCb func(Node) error) error
	Deregister(ctx context.Context) error
	GetNodes(ctx context.Context) ([]Node, error)
}

type Node struct {
	ID          string
	Hostname    string
	ServiceName string
	State       string
}
