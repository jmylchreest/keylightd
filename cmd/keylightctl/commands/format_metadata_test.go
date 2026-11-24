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
	require.Equal(t, [][]string{
		{"available", "false"},
		{"vendor_info.enabled", "false"},
		{"vendor_info.limit", "0"},
	}, [][]string(rows[len(rows)-3:]))
}
