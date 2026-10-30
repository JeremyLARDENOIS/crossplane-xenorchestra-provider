// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package network

import "github.com/crossplane/upjet/v2/pkg/config"

// Configure configures the resources of the network group, for both the cluster
// and the namespaced providers.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("xenorchestra_network", func(r *config.Resource) {
		r.ShortGroup = "network"
		r.Kind = "Network"
		r.UseAsync = true
	})
	p.AddResourceConfigurator("xenorchestra_bonded_network", func(r *config.Resource) {
		r.ShortGroup = "network"
		r.Kind = "BondedNetwork"
		r.UseAsync = true
	})
}
