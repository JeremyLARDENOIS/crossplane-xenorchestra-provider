// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crossplane/upjet/v2/pkg/terraform"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// fakeClient is a provider client answering the health checks with user and
// err, after release is closed if it is not nil.
type fakeClient struct {
	url     string
	user    any
	err     error
	release chan struct{}
	closed  atomic.Bool
}

func (f *fakeClient) Call(_ string, _, result any) error {
	if f.release != nil {
		<-f.release
	}
	if f.err != nil {
		return f.err
	}
	*result.(*any) = f.user
	return nil
}

func (f *fakeClient) Close() error {
	f.closed.Store(true)
	return nil
}

// fakeProvider builds Terraform providers whose configuration calls
// configure with the configured url.
type fakeProvider struct {
	configure func(url string) (*fakeClient, error)
	count     atomic.Int64
}

func (f *fakeProvider) newProvider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"url": {Type: schema.TypeString, Optional: true},
		},
		ConfigureContextFunc: func(_ context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
			f.count.Add(1)
			c, err := f.configure(d.Get("url").(string))
			if err != nil {
				return nil, diag.FromErr(err)
			}
			return c, nil
		},
	}
}

func healthyClient(url string) (*fakeClient, error) {
	return &fakeClient{url: url, user: map[string]any{"id": "user"}}, nil
}

func setupFor(url string) terraform.Setup {
	return terraform.Setup{Configuration: terraform.ProviderConfiguration{"url": url}}
}

func tryConfigure(c *noForkSetupCache, url string) (*fakeClient, error) {
	ps, err := c.configure(context.Background(), setupFor(url), []byte(url))
	if err != nil {
		return nil, err
	}
	return ps.Meta.(*fakeClient), nil
}

func configureURL(t *testing.T, c *noForkSetupCache, url string) *fakeClient {
	t.Helper()
	client, err := tryConfigure(c, url)
	if err != nil {
		t.Fatalf("configure(%q): %v", url, err)
	}
	return client
}

// waitClosed waits for the asynchronous closing of an evicted client.
func waitClosed(t *testing.T, client *fakeClient) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !client.closed.Load() {
		if time.Now().After(deadline) {
			t.Error("the evicted client was not closed")
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func TestConfigureReusesClient(t *testing.T) {
	p := &fakeProvider{configure: healthyClient}
	c := newNoForkSetupCache(p.newProvider)

	var wg sync.WaitGroup
	clients := make([]*fakeClient, 20)
	for i := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			if clients[i], err = tryConfigure(c, "a"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got := p.count.Load(); got != 1 {
		t.Fatalf("provider configured %d times, want 1", got)
	}
	for _, client := range clients {
		if client != clients[0] {
			t.Fatal("configure returned different clients for the same credentials")
		}
	}
	if b := configureURL(t, c, "b"); b == clients[0] {
		t.Fatal("configure returned the same client for different credentials")
	}
}

func TestConfigureEvictsUnhealthyClient(t *testing.T) {
	cases := map[string]struct {
		client *fakeClient
	}{
		"CallFails": {
			client: &fakeClient{err: errors.New("connection closed")},
		},
		"NoSession": {
			client: &fakeClient{user: nil},
		},
		"NoAnswer": {
			client: &fakeClient{user: map[string]any{}, release: make(chan struct{})},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &fakeProvider{}
			p.configure = func(url string) (*fakeClient, error) {
				if p.count.Load() == 1 {
					return tc.client, nil
				}
				return healthyClient(url)
			}
			c := newNoForkSetupCache(p.newProvider)
			c.healthCheckInterval = 0
			c.healthCheckTimeout = 10 * time.Millisecond

			if first := configureURL(t, c, "a"); first != tc.client {
				t.Fatal("the first configure did not return the first client")
			}
			if second := configureURL(t, c, "a"); second == tc.client {
				t.Fatal("the unhealthy client was reused")
			}
			waitClosed(t, tc.client)
			if tc.client.release != nil {
				close(tc.client.release)
			}
		})
	}
}

func TestConfigureEvictsIdleClients(t *testing.T) {
	p := &fakeProvider{configure: healthyClient}
	c := newNoForkSetupCache(p.newProvider)
	c.idleTimeout = 20 * time.Millisecond

	a := configureURL(t, c, "a")
	time.Sleep(2 * c.idleTimeout)
	configureURL(t, c, "b")

	waitClosed(t, a)
	if again := configureURL(t, c, "a"); again == a {
		t.Error("the idle client was reused")
	}
}

func TestConfigureError(t *testing.T) {
	p := &fakeProvider{configure: func(string) (*fakeClient, error) {
		return nil, errors.New("cannot connect")
	}}
	c := newNoForkSetupCache(p.newProvider)

	for range 2 {
		if _, err := c.configure(context.Background(), setupFor("a"), []byte("a")); err == nil {
			t.Fatal("configure did not return the provider error")
		}
	}
	if got := p.count.Load(); got != 2 {
		t.Errorf("provider configured %d times, want 2: a failed configuration must not be cached", got)
	}
}

func TestConfigureDoesNotBlockCachedCredentials(t *testing.T) {
	release := make(chan struct{})
	p := &fakeProvider{configure: func(url string) (*fakeClient, error) {
		if url == "slow" {
			<-release
		}
		return healthyClient(url)
	}}
	c := newNoForkSetupCache(p.newProvider)
	configureURL(t, c, "a")

	go tryConfigure(c, "slow") //nolint:errcheck // only blocks the configurations
	for p.count.Load() < 2 {
		time.Sleep(time.Millisecond)
	}

	done := make(chan struct{})
	go func() {
		if _, err := tryConfigure(c, "a"); err != nil {
			t.Error(err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cached credentials were blocked by the configuration of other credentials")
	}
	close(release)
}

func TestConfigureSerializesConfigurations(t *testing.T) {
	var active, maxActive atomic.Int64
	p := &fakeProvider{configure: func(url string) (*fakeClient, error) {
		n := active.Add(1)
		for {
			m := maxActive.Load()
			if n <= m || maxActive.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return healthyClient(url)
	}}
	c := newNoForkSetupCache(p.newProvider)

	var wg sync.WaitGroup
	for _, url := range []string{"a", "b", "c", "d", "e"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tryConfigure(c, url); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got := maxActive.Load(); got != 1 {
		t.Errorf("%d configurations ran concurrently, want 1", got)
	}
}
