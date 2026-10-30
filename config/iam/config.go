// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package iam

import "github.com/crossplane/upjet/v2/pkg/config"

// Configure configures the resources of the iam group, for both the cluster
// and the namespaced providers.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("xenorchestra_acl", func(r *config.Resource) {
		r.ShortGroup = "iam"
		r.Kind = "ACL"
		r.UseAsync = true
	})
	p.AddResourceConfigurator("xenorchestra_resource_set", func(r *config.Resource) {
		r.ShortGroup = "iam"
		r.Kind = "ResourceSet"
		r.UseAsync = true
	})
}
