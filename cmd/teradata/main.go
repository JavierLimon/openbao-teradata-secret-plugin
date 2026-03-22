package main

import (
	"fmt"
	"os"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/logging"
	teradata "github.com/JavierLimon/openbao-teradata-secret-plugin/plugin"
	"github.com/openbao/openbao/api/v2"
	"github.com/openbao/openbao/sdk/v2/plugin"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("Teradata Secret Plugin\n")
		fmt.Printf("Version: %s\n", teradata.Version)
		os.Exit(0)
	}

	logging.Init()
	logging.LogStartup(nil, teradata.Version)

	apiClientMeta := &api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()
	flags.Parse(os.Args[1:])

	tlsConfig := apiClientMeta.GetTLSConfig()
	tlsProviderFunc := api.VaultPluginTLSProvider(tlsConfig)

	err := plugin.ServeMultiplex(&plugin.ServeOpts{
		BackendFactoryFunc: teradata.Factory,
		TLSProviderFunc:    tlsProviderFunc,
	})
	if err != nil {
		logging.LogError(nil, "plugin_serve_error", err, nil)
		os.Exit(1)
	}

	logging.LogShutdown(nil)
}
