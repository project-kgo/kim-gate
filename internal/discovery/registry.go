package discovery

import "context"

// ServiceInstance identifies a server instance for registration.
type ServiceInstance struct {
	ID      string
	Name    string
	Address string
}

// ServiceRegistry abstracts service registration for gRPC.
// Implementations must be safe for concurrent use.
type ServiceRegistry interface {
	Register(ctx context.Context, instance ServiceInstance) error
	Deregister(ctx context.Context) error
	Close() error
}
