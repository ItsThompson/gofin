package service

import (
	"fmt"
	"math"
	"math/big"
	"strings"
)

func parsePositiveRat(value string) (*big.Rat, error) {
	rat := new(big.Rat)
	if _, ok := rat.SetString(value); !ok {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	if rat.Sign() <= 0 {
		return nil, fmt.Errorf("decimal must be positive")
	}
	return rat, nil
}

func decimalString(value *big.Rat) string {
	out := value.FloatString(18)
	out = strings.TrimRight(out, "0")
	out = strings.TrimRight(out, ".")
	if out == "" || out == "-" {
		return "0"
	}
	return out
}

func scaleFactor(digits int) *big.Int {
	factor := big.NewInt(1)
	for range digits {
		factor.Mul(factor, big.NewInt(10))
	}
	return factor
}

func minorToMajor(amount int64, digits int) *big.Rat {
	return new(big.Rat).SetFrac(big.NewInt(amount), scaleFactor(digits))
}

func majorToRoundedMinor(amount *big.Rat, digits int) int64 {
	scaled := new(big.Rat).Mul(amount, new(big.Rat).SetInt(scaleFactor(digits)))
	return roundHalfAwayFromZero(scaled)
}

func roundHalfAwayFromZero(value *big.Rat) int64 {
	numerator := new(big.Int).Set(value.Num())
	denominator := new(big.Int).Set(value.Denom())
	quotient, remainder := new(big.Int).QuoRem(numerator, denominator, new(big.Int))
	if remainder.Sign() < 0 {
		remainder.Abs(remainder)
	}
	doubled := new(big.Int).Mul(remainder, big.NewInt(2))
	if doubled.Cmp(denominator) >= 0 {
		if value.Sign() >= 0 {
			quotient.Add(quotient, big.NewInt(1))
		} else {
			quotient.Sub(quotient, big.NewInt(1))
		}
	}
	if !quotient.IsInt64() {
		if quotient.Sign() >= 0 {
			return math.MaxInt64
		}
		return math.MinInt64
	}
	return quotient.Int64()
}
