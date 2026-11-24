package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLightTableDataIncludesOptionalMetadata(t *testing.T) {
	rows := LightTableData("desk", map[string]any{
		"vendor_info": map[string]any{"enabled": false, "limit": 0},
		"absent":      nil,
		"available":   false,
		"state":       map[string]any{},
	})
	require.Contains(t, rows, []string{"Product", "<nil>"})
	require.Equal(t, []string{"available", "false"}, rows[len(rows)-2])
	require.Equal(t, []string{"vendor_info", `{"enabled":false,"limit":0}`}, rows[len(rows)-1])
}
