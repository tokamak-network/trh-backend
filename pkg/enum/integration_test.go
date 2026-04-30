package enum_test

import (
	"testing"

	"github.com/tokamak-network/trh-backend/pkg/enum"
)

func TestIntegrationTypeValues(t *testing.T) {
	if got := enum.IntegrationTypeCrossTrade.String(); got != "cross-trade" {
		t.Errorf("IntegrationTypeCrossTrade = %q, want %q", got, "cross-trade")
	}
	if got := enum.IntegrationTypeDRB.String(); got != "drb" {
		t.Errorf("IntegrationTypeDRB = %q, want %q", got, "drb")
	}
	_ = enum.IntegrationTypeCrossTrade
	_ = enum.IntegrationTypeDRB
}
