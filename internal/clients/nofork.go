// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"crypto/sha256"
	"io"
	"sync"
	"time"

	"github.com/crossplane/upjet/v2/pkg/terraform"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	tfsdk "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/pkg/errors"
	"github.com/vatesfr/terraform-provider-xenorchestra/xoa"
	"golang.org/x/sync/singleflight"
)

const (
	// noForkHealthCheckMethod is the JSON-RPC method called to check that a
	// cached client still has a working session: it returns the signed in
	// user, or null when the connection has no session.
	noForkHealthCheckMethod = "session.getUser"
	// noForkHealthCheckInterval is the minimum time between two health checks
	// of a cached client.
	noForkHealthCheckInterval = time.Minute
	// noForkHealthCheckTimeout bounds a health check, so that a connection
	// that stopped answering is considered dead.
	noForkHealthCheckTimeout = 10 * time.Second
	// noForkIdleTimeout is how long an unused client stays cached, e.g. after
	// the credentials of a ProviderConfig were rotated.
	noForkIdleTimeout = time.Hour
)

// noForkSetups is shared by the cluster and namespaced controllers.
var noForkSetups = newNoForkSetupCache(xoa.Provider)

// rpcCaller is implemented by the Xen Orchestra SDK client.
type rpcCaller interface {
	Call(method string, params, result any) error
}

type noForkSetup struct {
	meta      any
	checkedAt time.Time
	usedAt    time.Time
}

// noForkSetupCache configures the Terraform provider in-process and caches
// the resulting client per set of credentials: configuring the provider opens
// a new session, which must not happen on every reconciliation. A cached
// client is kept as long as it passes its health checks.
type noForkSetupCache struct {
	newProvider         func() *schema.Provider
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	idleTimeout         time.Duration

	// configureMu serializes the configurations of the provider: the Xen
	// Orchestra SDK sets the TLS configuration (insecure or not) of a global
	// websocket dialer right before dialing, so concurrent configurations
	// could use the TLS configuration of each other.
	configureMu sync.Mutex

	// group makes the callers sharing the same credentials wait for a single
	// health check or configuration, without blocking the other credentials.
	group singleflight.Group

	mu     sync.Mutex
	setups map[string]*noForkSetup
}

func newNoForkSetupCache(newProvider func() *schema.Provider) *noForkSetupCache {
	return &noForkSetupCache{
		newProvider:         newProvider,
		healthCheckInterval: noForkHealthCheckInterval,
		healthCheckTimeout:  noForkHealthCheckTimeout,
		idleTimeout:         noForkIdleTimeout,
		setups:              map[string]*noForkSetup{},
	}
}

// configure sets the Meta of ps to a Terraform provider client configured
// with ps.Configuration, reusing the client cached for credentials if any.
func (c *noForkSetupCache) configure(ctx context.Context, ps terraform.Setup, credentials []byte) (terraform.Setup, error) {
	sum := sha256.Sum256(credentials)
	key := string(sum[:])
	if meta, ok := c.get(key, false); ok {
		ps.Meta = meta
		return ps, nil
	}

	meta, err, _ := c.group.Do(key, func() (any, error) {
		if meta, ok := c.get(key, true); ok {
			return meta, nil
		}
		return c.create(ctx, key, ps.Configuration)
	})
	if err != nil {
		return ps, err
	}
	ps.Meta = meta
	return ps, nil
}

// get returns the client cached for key. A client that was not checked
// recently is returned only when check is true and its health check passes;
// it is evicted otherwise.
func (c *noForkSetupCache) get(key string, check bool) (any, bool) {
	now := time.Now()
	c.mu.Lock()
	c.evictIdle(now)
	s, ok := c.setups[key]
	if !ok {
		c.mu.Unlock()
		return nil, false
	}
	s.usedAt = now
	meta, checkedAt := s.meta, s.checkedAt
	c.mu.Unlock()

	if now.Sub(checkedAt) < c.healthCheckInterval {
		return meta, true
	}
	if !check {
		return nil, false
	}
	if err := c.healthCheck(meta); err != nil {
		c.mu.Lock()
		c.evict(key)
		c.mu.Unlock()
		return nil, false
	}
	c.mu.Lock()
	if s, ok := c.setups[key]; ok {
		s.checkedAt = time.Now()
	}
	c.mu.Unlock()
	return meta, true
}

func (c *noForkSetupCache) create(ctx context.Context, key string, cfg terraform.ProviderConfiguration) (any, error) {
	// A new provider instance is configured each time since the provider
	// stores the configured client in itself.
	p := c.newProvider()
	c.configureMu.Lock()
	diags := p.Configure(context.WithoutCancel(ctx), &tfsdk.ResourceConfig{Config: cfg})
	c.configureMu.Unlock()
	if diags.HasError() {
		return nil, errors.Errorf("cannot configure the Terraform provider: %v", diags)
	}
	now := time.Now()
	c.mu.Lock()
	c.setups[key] = &noForkSetup{meta: p.Meta(), checkedAt: now, usedAt: now}
	c.mu.Unlock()
	return p.Meta(), nil
}

// evictIdle evicts the clients unused for c.idleTimeout. c.mu must be held.
func (c *noForkSetupCache) evictIdle(now time.Time) {
	for key, s := range c.setups {
		if now.Sub(s.usedAt) > c.idleTimeout {
			c.evict(key)
		}
	}
}

// evict removes the client cached for key and closes it if it can be closed.
// c.mu must be held.
func (c *noForkSetupCache) evict(key string) {
	s, ok := c.setups[key]
	if !ok {
		return
	}
	delete(c.setups, key)
	if closer, ok := s.meta.(io.Closer); ok {
		go closer.Close() //nolint:errcheck // nothing to do with a failed close
	}
}

// healthCheck calls noForkHealthCheckMethod on the client, if it supports it,
// and fails if the call fails or the client has no session.
func (c *noForkSetupCache) healthCheck(meta any) error {
	caller, ok := meta.(rpcCaller)
	if !ok {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		var user any
		if err := caller.Call(noForkHealthCheckMethod, map[string]any{"dummy": "dummy"}, &user); err != nil {
			done <- err
			return
		}
		if user == nil {
			done <- errors.New("the connection has no session")
			return
		}
		done <- nil
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(c.healthCheckTimeout):
		return errors.New("health check timed out")
	}
}
