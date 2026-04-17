package service

import (
	"context"
)

type TxManagerStub struct{}

var _ TxManager = TxManagerStub{}

func (TxManagerStub) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
