// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package vm

import "github.com/crossplane/upjet/v2/pkg/config"

// Configure configures the resources of the vm group, for both the cluster
// and the namespaced providers.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("xenorchestra_vm", func(r *config.Resource) {
		r.ShortGroup = "vm"
		r.Kind = "VM"
		r.UseAsync = true
		r.References["network.network_id"] = config.Reference{
			TerraformName: "xenorchestra_network",
		}
		r.References["cdrom.id"] = config.Reference{
			TerraformName: "xenorchestra_vdi",
		}
	})
	p.AddResourceConfigurator("xenorchestra_cloud_config", func(r *config.Resource) {
		r.ShortGroup = "vm"
		r.Kind = "CloudConfig"
		r.UseAsync = true
	})
}
