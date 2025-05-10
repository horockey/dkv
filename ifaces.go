package dkv

import (
	"context"

	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/processor"
)

type (
	Processor[V any] = processor.Processor[V]
	Merger[V any]    = model.Merger[V]
	Discovery        = model.Discovery
	Node             = model.Node
)

type Controller[V any] interface {
	model.MetricsProvider
	Start(ctx context.Context, proc *Processor[V]) error
}
