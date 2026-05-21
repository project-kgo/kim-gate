package etcd

import (
	"context"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	etcdresolver "go.etcd.io/etcd/client/v3/naming/resolver"
	gresolver "google.golang.org/grpc/resolver"

	"github.com/project-kgo/kim-gate/internal/discovery"
)

// Config holds etcd connection and registration parameters.
type Config struct {
	Endpoints   []string
	ServiceName string
	Username    string
	Password    string
	TTL         time.Duration
}

// Registry implements discovery.ServiceRegistry using etcd.
type Registry struct {
	client  *clientv3.Client
	manager endpoints.Manager
	cfg     Config

	mu         sync.Mutex
	registered bool
	instance   discovery.ServiceInstance
	leaseID    clientv3.LeaseID
	ctx        context.Context
	cancel     context.CancelFunc
}

// New creates an etcd-backed Registry.
func New(cfg Config) (*Registry, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: cfg.Endpoints,
		Username:  cfg.Username,
		Password:  cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("create etcd client: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Registry{
		client: cli,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (r *Registry) Register(ctx context.Context, instance discovery.ServiceInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.registered {
		return nil
	}

	manager, err := endpoints.NewManager(r.client, instance.Name)
	if err != nil {
		return fmt.Errorf("etcd new manager: %w", err)
	}
	r.manager = manager

	ttl := r.cfg.TTL
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	lresp, err := r.client.Grant(ctx, int64(ttl.Seconds()))
	if err != nil {
		return fmt.Errorf("etcd grant lease: %w", err)
	}
	r.leaseID = lresp.ID

	kaCh, err := r.client.KeepAlive(r.ctx, lresp.ID)
	if err != nil {
		return fmt.Errorf("etcd keepalive: %w", err)
	}
	go func() {
		for range kaCh {
		}
	}()

	key := fmt.Sprintf("%s/%s", instance.Name, instance.ID)
	ep := endpoints.Endpoint{Addr: instance.Address}
	if err := r.manager.AddEndpoint(ctx, key, ep, clientv3.WithLease(lresp.ID)); err != nil {
		_, _ = r.client.Revoke(ctx, lresp.ID)
		return fmt.Errorf("etcd add endpoint: %w", err)
	}

	r.instance = instance
	r.registered = true
	return nil
}

func (r *Registry) Deregister(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.registered || r.manager == nil {
		return nil
	}

	key := fmt.Sprintf("%s/%s", r.instance.Name, r.instance.ID)
	_ = r.manager.DeleteEndpoint(ctx, key)
	r.registered = false
	return nil
}

func (r *Registry) Close() error {
	r.cancel()
	return r.client.Close()
}

// ResolverBuilder returns a gRPC resolver.Builder that resolves targets
// via etcd using the "etcd" scheme.
//
// Usage:
//
//	builder := etcd.ResolverBuilder(client)
//	conn, _ := grpc.Dial("etcd:///kim-gate",
//	    grpc.WithResolvers(builder),
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//
// ResolverBuilder returns a gRPC resolver.Builder that resolves targets
// via etcd using the "etcd" scheme.
func ResolverBuilder(cli *clientv3.Client) gresolver.Builder {
	b, _ := etcdresolver.NewBuilder(cli)
	return b
}
