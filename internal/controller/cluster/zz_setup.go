// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/v2/pkg/controller"

	acl "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/iam/acl"
	resourceset "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/iam/resourceset"
	bondednetwork "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/network/bondednetwork"
	network "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/network/network"
	providerconfig "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/providerconfig"
	vdi "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/storage/vdi"
	cloudconfig "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/vm/cloudconfig"
	vm "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/internal/controller/cluster/vm/vm"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		acl.Setup,
		resourceset.Setup,
		bondednetwork.Setup,
		network.Setup,
		providerconfig.Setup,
		vdi.Setup,
		cloudconfig.Setup,
		vm.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated creates all controllers with the supplied logger and adds them to
// the supplied manager gated.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		acl.SetupGated,
		resourceset.SetupGated,
		bondednetwork.SetupGated,
		network.SetupGated,
		providerconfig.SetupGated,
		vdi.SetupGated,
		cloudconfig.SetupGated,
		vm.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupWebhookWithManager registers conversion webhooks for all resource kinds in the group.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	for _, setup := range []func(ctrl.Manager) error{
		acl.SetupWebhookWithManager,
		resourceset.SetupWebhookWithManager,
		bondednetwork.SetupWebhookWithManager,
		network.SetupWebhookWithManager,
		providerconfig.SetupWebhookWithManager,
		vdi.SetupWebhookWithManager,
		cloudconfig.SetupWebhookWithManager,
		vm.SetupWebhookWithManager,
	} {
		if err := setup(mgr); err != nil {
			return err
		}
	}
	return nil
}
