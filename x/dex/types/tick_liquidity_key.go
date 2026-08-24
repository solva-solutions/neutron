package types

import math_utils "github.com/solva-solutions/neutron/v11/utils/math"

type TickLiquidityKey interface {
	KeyMarshal() []byte
	Price() (priceTakerToMaker math_utils.PrecDec, err error)
}
