package keylight

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveProduct(t *testing.T) {
	for _, tt := range []struct {
		name  string
		board int
		want  string
	}{
		{"Elgato Key Light", 0, "Elgato Key Light"},
		{"Elgato Key Light Air", 999, "Elgato Key Light Air"},
		{"Elgato Key Light MK.2", 0, "Elgato Key Light MK.2"},
		{"Elgato Ring Light", 0, "Elgato Ring Light"},
		{"Unknown model variant", 53, "Elgato Key Light"},
		{"", 200, "Elgato Key Light Air"},
		{"Elgato Key Light", 205, "Elgato Key Light MK.2"},
		{"", 201, "Elgato Ring Light"},
		{"Elgato Key Light Mini", 202, ""},
		{"Elgato Key Light Neo", 210, "Elgato Key Light Neo"},
		{"Elgato Light Strip", 70, ""},
		{"Elgato Light Strip Pro", 206, ""},
		{"Elgato Key Light Air MK.2", 214, ""},
		{"", 0, ""},
		{"Unknown", 999, ""},
	} {
		t.Run(tt.name+tt.want, func(t *testing.T) {
			product, ok := ResolveProduct("elgato", tt.board, tt.name)
			require.Equal(t, tt.want != "", ok)
			require.Equal(t, tt.want, product.Name)
			require.Equal(t, tt.board == 210, product.Experimental)
		})
	}
}

func TestRegistryMergesExperimental(t *testing.T) {
	registry := productRegistry{
		supported:    []Product{{Manufacturer: "elgato", Name: "Supported", BoardIDs: []int{1}, Names: []string{"Supported alias"}}},
		experimental: []Product{{Manufacturer: "elgato", Name: "Experimental", BoardIDs: []int{2}, Names: []string{"Experimental alias"}}},
	}
	for _, name := range []string{"Experimental", "Experimental alias"} {
		product, ok := registry.resolve("elgato", 0, name)
		require.True(t, ok)
		require.Equal(t, "Experimental", product.Name)
		require.True(t, product.Experimental)
	}
	product, ok := registry.resolve("elgato", 2, "Supported alias")
	require.True(t, ok)
	require.Equal(t, "Experimental", product.Name)
	require.True(t, product.Experimental)
	// Callers cannot mutate the registry through returned slices.
	product.BoardIDs[0] = 99
	product.Names[0] = "Changed"
	product, ok = registry.resolve("elgato", 2, "")
	require.True(t, ok)
	require.Equal(t, "Experimental alias", product.Names[0])
}

func TestValidateLightResolvesModelWithoutReplacingReportedName(t *testing.T) {
	server := httptest.NewServer(newAccessoryInfoHandler("Model-specific name", 205, "Desk", 0))
	defer server.Close()
	light, ok := validateLight(context.Background(), makeServiceEntry(t, server, "desk._elg._tcp.local."), discardLogger())
	require.True(t, ok)
	require.Equal(t, "Model-specific name", light.ProductName)
	require.Equal(t, "Elgato Key Light MK.2", light.Model)
	require.False(t, light.Experimental)
}

func TestUpdateLightInfoRefreshesModel(t *testing.T) {
	m := NewManager(discardLogger())
	m.lights["test"] = Light{ID: "test", Model: "Old", Experimental: true}
	light, err := m.updateLightInfo("test", &AccessoryInfo{ProductName: "Variant", HardwareBoardType: 205})
	require.NoError(t, err)
	require.Equal(t, "Elgato Key Light MK.2", light.Model)
	require.False(t, light.Experimental)
	light, err = m.updateLightInfo("test", &AccessoryInfo{ProductName: "Unknown", HardwareBoardType: 999})
	require.NoError(t, err)
	require.Empty(t, light.Model)
}

func TestRegistryScopesIdentifiersToManufacturer(t *testing.T) {
	registry := productRegistry{supported: []Product{
		{Manufacturer: "elgato", Name: "Shared name", BoardIDs: []int{53}},
		{Manufacturer: "other", Name: "Other light", BoardIDs: []int{53}, Names: []string{"Shared name"}},
	}}
	product, ok := registry.resolve("other", 53, "")
	require.True(t, ok)
	require.Equal(t, "Other light", product.Name)
	product, ok = registry.resolve("other", 0, "Shared name")
	require.True(t, ok)
	require.Equal(t, "other", product.Manufacturer)
	_, ok = registry.resolve("unknown", 53, "Shared name")
	require.False(t, ok)
	_, ok = registry.resolve("", 53, "Shared name")
	require.False(t, ok)
}

func TestRegistryParityAndUniqueIdentifiers(t *testing.T) {
	require.Len(t, products.supported, 4)
	require.Len(t, products.experimental, 1)
	boards := map[string]map[int]bool{}
	names := map[string]map[string]bool{}
	for _, group := range [][]Product{products.supported, products.experimental} {
		for _, product := range group {
			require.NotEmpty(t, product.Manufacturer)
			require.NotEmpty(t, product.Name)
			if boards[product.Manufacturer] == nil {
				boards[product.Manufacturer] = map[int]bool{}
				names[product.Manufacturer] = map[string]bool{}
			}
			for _, id := range product.BoardIDs {
				require.Positive(t, id)
				require.False(t, boards[product.Manufacturer][id], "duplicate board: %d", id)
				boards[product.Manufacturer][id] = true
			}
			for _, name := range append([]string{product.Name}, product.Names...) {
				require.False(t, names[product.Manufacturer][name], "duplicate name: %s", name)
				names[product.Manufacturer][name] = true
			}
		}
	}
}

func TestResolveModelPreservesManufacturerScope(t *testing.T) {
	light := Light{Manufacturer: "other", ProductName: "Elgato Key Light", HardwareBoardType: 53}
	light.resolveModel()
	require.Equal(t, "other", light.Manufacturer)
	require.Empty(t, light.Model)
	require.False(t, light.Experimental)
}
