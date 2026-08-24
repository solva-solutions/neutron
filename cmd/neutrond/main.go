package main

import (
	"os"

	"github.com/solva-solutions/neutron/v11/app/config"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"github.com/solva-solutions/neutron/v11/app"
)

func main() {
	config := config.GetDefaultConfig()
	config.Seal()

	rootCmd, _ := NewRootCmd()

	if err := svrcmd.Execute(rootCmd, "", app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
