// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// A healthy remote refresh may take longer than five seconds. It must use
// the caller's acquisition budget, including an explicitly unbounded wait.
func TestResourceWaitSlowProbeUsesAcquisitionBudget(t *testing.T) {
	for _, wait := range []time.Duration{20 * time.Second, 0} {
		t.Run(wait.String(), func(t *testing.T) {
			fastHostLockPolling(t)
			holder := holdHostLockInProcess(t, "/holder", "holder")
			defer holder.release()
			parent, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			ctx := withResourceWaitObserver(parent, func(ctx context.Context) error {
				timer := time.NewTimer(6 * time.Second)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timer.C:
					holder.release()
					return nil
				}
			})
			lock, err := acquireHostLockWithSource(ctx, hostLockHolder{Branch: "waiter"}, wait, "flag", io.Discard)
			if err != nil {
				t.Fatalf("healthy slow probe ended acquisition: %v", err)
			}
			lock.release()
		})
	}
}

func TestResourceWaitProbePreservesErrors(t *testing.T) {
	for _, want := range []error{errTargetMovedWhileWaiting, context.Canceled, errors.New("refresh failed")} {
		t.Run(want.Error(), func(t *testing.T) {
			ctx := withResourceWaitObserver(context.Background(), func(context.Context) error { return want })
			if got := observeResourceWait(ctx, time.Minute, true); !errors.Is(got, want) {
				t.Fatalf("probe error = %v, want %v", got, want)
			}
		})
	}
}
