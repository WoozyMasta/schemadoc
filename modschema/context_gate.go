// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"context"
	"sync"
)

// contextGate is a lazily initialized mutex whose wait can be cancelled.
type contextGate struct {
	token chan struct{} // Serializes owners without blocking cancellation.
	once  sync.Once     // Initializes token exactly once, including for zero values.
}

// lock acquires exclusive ownership unless the caller's context expires.
func (gate *contextGate) lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	gate.once.Do(func() { gate.token = make(chan struct{}, 1) })

	select {
	case gate.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			gate.unlock()
			return err
		}

		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// unlock releases ownership acquired by lock.
func (gate *contextGate) unlock() {
	<-gate.token
}
