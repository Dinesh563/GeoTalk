package main

import "math"

func RoundTo4Decimal(f float64) float64 {
	return math.Round(f*10000) / 10000
}