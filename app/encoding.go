package app

import (
	"github.com/cosmos/cosmos-sdk/std"

	ethcryptocodec "github.com/solva-solutions/neutron/v11/x/crypto/codec"

	"github.com/solva-solutions/neutron/v11/app/params"
)

// MakeEncodingConfig creates an EncodingConfig for testing
func MakeEncodingConfig() params.EncodingConfig {
	encodingConfig := params.MakeEncodingConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	ethcryptocodec.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ethcryptocodec.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ModuleBasics.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	return encodingConfig
}
