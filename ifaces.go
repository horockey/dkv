package dkv

import (
	"context"
	"fmt"

	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/processor"
)

type (
	Processor[K fmt.Stringer, V any] = processor.Processor[K, V]
	Merger[K fmt.Stringer, V any]    = model.Merger[K, V]
)

type Controller[K fmt.Stringer, V any] interface {
	model.MetricsProvider
	Start(ctx context.Context, proc *Processor[K, V]) error
}
