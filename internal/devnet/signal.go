package devnet

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// SignalHandler manages graceful shutdown on OS signals.
type SignalHandler struct {
	ctx        context.Context
	cancel     context.CancelFunc
	signalChan chan os.Signal
	callbacks  []func()
}

// NewSignalHandler creates a new SignalHandler that listens for
// SIGINT and SIGTERM signals.
func NewSignalHandler() *SignalHandler {
	ctx, cancel := context.WithCancel(context.Background())
	h := &SignalHandler{
		ctx:        ctx,
		cancel:     cancel,
		signalChan: make(chan os.Signal, 1),
		callbacks:  make([]func(), 0),
	}

	signal.Notify(h.signalChan, syscall.SIGINT, syscall.SIGTERM)

	go h.listen()

	return h
}

// listen waits for signals and triggers shutdown.
func (h *SignalHandler) listen() {
	<-h.signalChan
	h.Shutdown()
}

// Context returns the context that will be canceled on shutdown.
func (h *SignalHandler) Context() context.Context {
	return h.ctx
}

// OnShutdown registers a callback to be called on shutdown.
// Callbacks are executed in reverse order (LIFO).
func (h *SignalHandler) OnShutdown(callback func()) {
	h.callbacks = append(h.callbacks, callback)
}

// Shutdown triggers the shutdown sequence.
// It cancels the context and calls all registered callbacks.
func (h *SignalHandler) Shutdown() {
	// Cancel context first
	h.cancel()

	// Execute callbacks in reverse order (LIFO)
	for i := len(h.callbacks) - 1; i >= 0; i-- {
		h.callbacks[i]()
	}
}

// Stop stops listening for signals and cleans up.
func (h *SignalHandler) Stop() {
	signal.Stop(h.signalChan)
	close(h.signalChan)
}

// Done returns a channel that is closed when shutdown is triggered.
func (h *SignalHandler) Done() <-chan struct{} {
	return h.ctx.Done()
}

// IsShutdown returns true if shutdown has been triggered.
func (h *SignalHandler) IsShutdown() bool {
	select {
	case <-h.ctx.Done():
		return true
	default:
		return false
	}
}

// WaitForSignal blocks until a shutdown signal is received.
func (h *SignalHandler) WaitForSignal() {
	<-h.ctx.Done()
}

// WithTimeout creates a new context with timeout that will also be
// canceled on shutdown.
func (h *SignalHandler) WithTimeout(timeout context.Context) context.Context {
	ctx, cancel := context.WithCancel(timeout)

	go func() {
		select {
		case <-h.ctx.Done():
			cancel()
		case <-timeout.Done():
			cancel()
		}
	}()

	return ctx
}
