package dkv

import (
	"context"

	"github.com/horockey/dkv/internal/model"
)

type Controller interface {
	model.MetricsProvider
	Start(context.Context) error
}
