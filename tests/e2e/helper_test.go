package e2e_test

import (
	"context"
	"time"

	"github.com/puppetlabs/leg/timeutil/pkg/backoff"
	"github.com/puppetlabs/leg/timeutil/pkg/retry"
)

var backoffFactory = backoff.Build(
	backoff.Exponential(250*time.Millisecond, 2.0),
	backoff.MaxBound(5*time.Second),
	backoff.FullJitter(),
	backoff.NonSliding,
)

func Wait(ctx context.Context, work retry.WorkFunc) error {
	return retry.Wait(ctx, work, retry.WithBackoffFactory(backoffFactory))
}
