package keylight

import "slices"

// Product describes an accepted model independently of a device's display name.
// Experimental marks entries awaiting the same validation as supported models.
type Product struct {
	Manufacturer string
	Name         string
	BoardIDs     []int
	Names        []string
	Experimental bool
}

type productRegistry struct {
	supported    []Product
	experimental []Product
}

var products = productRegistry{
	supported: []Product{
		{Manufacturer: "elgato", Name: "Elgato Key Light", BoardIDs: []int{53}},
		{Manufacturer: "elgato", Name: "Elgato Key Light Air", BoardIDs: []int{200}},
		{Manufacturer: "elgato", Name: "Elgato Key Light MK.2", BoardIDs: []int{205}},
		{Manufacturer: "elgato", Name: "Elgato Ring Light", BoardIDs: []int{201}},
	},
	experimental: []Product{
		{Manufacturer: "elgato", Name: "Elgato Key Light Mini", BoardIDs: []int{202}},
		{Manufacturer: "elgato", Name: "Elgato Key Light Neo", BoardIDs: []int{210}},
	},
}

// ExperimentalProducts returns a snapshot of the experimental device registry.
func ExperimentalProducts() []Product {
	result := slices.Clone(products.experimental)
	for i := range result {
		result[i].BoardIDs = slices.Clone(result[i].BoardIDs)
		result[i].Names = slices.Clone(result[i].Names)
		result[i].Experimental = true
	}
	return result
}

// ResolveProduct scopes identifiers to a manufacturer, matching a known board
// before falling back to an exact product name or alias. The returned value owns its slices, so callers cannot change
// the registry. An unknown product is not implicitly experimental.
func ResolveProduct(manufacturer string, boardID int, name string) (Product, bool) {
	return products.resolve(manufacturer, boardID, name)
}

func (r productRegistry) resolve(manufacturer string, boardID int, name string) (Product, bool) {
	groups := [][]Product{r.supported, r.experimental}
	// Search both lists by board first: a generic name must not hide a more
	// specific model, including one in the experimental list.
	for _, byBoard := range []bool{true, false} {
		for group, entries := range groups {
			for _, product := range entries {
				if product.Manufacturer != manufacturer {
					continue
				}
				match := boardID != 0 && slices.Contains(product.BoardIDs, boardID)
				if !byBoard {
					match = name != "" && (name == product.Name || slices.Contains(product.Names, name))
				}
				if match {
					product.Experimental = group == 1
					product.BoardIDs = slices.Clone(product.BoardIDs)
					product.Names = slices.Clone(product.Names)
					return product, true
				}
			}
		}
	}
	return Product{}, false
}

func (l *Light) resolveModel() {
	// Existing callers construct Elgato lights without manufacturer metadata.
	if l.Manufacturer == "" {
		l.Manufacturer = "elgato"
	}
	product, _ := ResolveProduct(l.Manufacturer, l.HardwareBoardType, l.ProductName)
	l.Model = product.Name
	l.Experimental = product.Experimental
}
