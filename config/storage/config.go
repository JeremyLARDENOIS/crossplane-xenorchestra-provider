// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package storage

import "github.com/crossplane/upjet/v2/pkg/config"

// Configure configures the resources of the storage group, for both the cluster
// and the namespaced providers.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("xenorchestra_vdi", func(r *config.Resource) {
		r.ShortGroup = "storage"
		r.Kind = "VDI"
		r.UseAsync = true
	})
}
