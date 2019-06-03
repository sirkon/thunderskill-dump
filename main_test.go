package main

import (
	"testing"

	"github.com/sanity-io/litter"
)

func TestGetVehicleStats(t *testing.T) {
	stats := getVehicleStats(*createLogger(), "/en/vehicle/us_m103")
	t.Log(litter.Sdump(stats))
}
