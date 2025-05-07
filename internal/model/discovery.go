package model

import (
	"context"
)

type Discovery interface {
	Register(ctx context.Context, hostname string, updCb func(Node) error, meta map[string]string) error
	Deregister(ctx context.Context) error
	GetNodes(ctx context.Context) ([]Node, error)
}

type Node struct {
	ID          string
	Hostname    string
	ServiceName string
	State       string
	Meta        map[string]string
}

const (
	StateDown = "down"
	StateUp   = "up"
)
