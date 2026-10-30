package config

import (
	// Note(turkenh): we are importing this to embed provider schema document
	_ "embed"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/vatesfr/terraform-provider-xenorchestra/xoa"

	"github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/config/iam"
	"github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/config/network"
	"github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/config/storage"
	"github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider/config/vm"
)

const (
	resourcePrefix = "xenorchestra"
	modulePath     = "github.com/JeremyLARDENOIS/crossplane-xenorchestra-provider"
)

//go:embed schema.json
var providerSchema string

//go:embed provider-metadata.yaml
var providerMetadata string

// GetProvider returns provider configuration
func GetProvider() *ujconfig.Provider {
	pc := ujconfig.NewProvider([]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		ujconfig.WithRootGroup("xenorchestra.crossplane.io"),
		ujconfig.WithIncludeList([]string{}),
		ujconfig.WithTerraformPluginSDKIncludeList(ExternalNameConfigured()),
		ujconfig.WithTerraformProvider(xoa.Provider()),
		ujconfig.WithFeaturesPackage("internal/features"),
		ujconfig.WithDefaultResourceOptions(
			ExternalNameConfigurations(),
		))

	for _, configure := range []func(provider *ujconfig.Provider){
		// add custom config functions
		iam.Configure,
		network.Configure,
		storage.Configure,
		vm.Configure,
	} {
		configure(pc)
	}

	pc.ConfigureResources()
	return pc
}

// GetProviderNamespaced returns the namespaced provider configuration
func GetProviderNamespaced() *ujconfig.Provider {
	pc := ujconfig.NewProvider([]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		ujconfig.WithRootGroup("xenorchestra.m.crossplane.io"),
		ujconfig.WithIncludeList([]string{}),
		ujconfig.WithTerraformPluginSDKIncludeList(ExternalNameConfigured()),
		ujconfig.WithTerraformProvider(xoa.Provider()),
		ujconfig.WithFeaturesPackage("internal/features"),
		ujconfig.WithDefaultResourceOptions(
			ExternalNameConfigurations(),
		),
		ujconfig.WithExampleManifestConfiguration(ujconfig.ExampleManifestConfiguration{
			ManagedResourceNamespace: "crossplane-system",
		}))

	for _, configure := range []func(provider *ujconfig.Provider){
		// add custom config functions
		iam.Configure,
		network.Configure,
		storage.Configure,
		vm.Configure,
	} {
		configure(pc)
	}

	pc.ConfigureResources()
	return pc
}
