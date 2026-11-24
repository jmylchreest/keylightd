// Command keylight-device-docs updates the generated registry section of the device guide.
package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

const guidePath = "docs/docs/supported-devices/index.md"

func main() {
	check := flag.Bool("check", false, "fail if the guide needs regeneration")
	flag.Parse()
	if err := updateGuide(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func updateGuide(check bool) error {
	const begin = "{/* BEGIN GENERATED EXPERIMENTAL DEVICES */}"
	const end = "{/* END GENERATED EXPERIMENTAL DEVICES */}"
	data, err := os.ReadFile(guidePath)
	if err != nil {
		return err
	}
	head, content, ok := strings.Cut(string(data), begin)
	if !ok {
		return fmt.Errorf("missing opening marker in %s", guidePath)
	}
	_, tail, ok := strings.Cut(content, end)
	if !ok {
		return fmt.Errorf("missing closing marker in %s", guidePath)
	}

	products := keylight.ExperimentalProducts()
	slices.SortFunc(products, func(a, b keylight.Product) int {
		return strings.Compare(a.Name, b.Name)
	})
	var rows strings.Builder
	for _, product := range products {
		fmt.Fprintf(&rows, "- **%s**", product.Name)
		ids := make([]string, 0, len(product.BoardIDs))
		for _, id := range product.BoardIDs {
			ids = append(ids, strconv.Itoa(id))
		}
		if len(ids) > 0 {
			fmt.Fprintf(&rows, " (board %s)", strings.Join(ids, ", "))
		}
		rows.WriteByte('\n')
	}
	if len(products) == 0 {
		rows.WriteString("There are currently no experimental devices.\n")
	}
	updated := head + begin + "\n" + rows.String() + end + tail
	if updated == string(data) {
		return nil
	}
	if check {
		return fmt.Errorf("%s is stale: run go run ./cmd/keylight-device-docs", guidePath)
	}
	return os.WriteFile(guidePath, []byte(updated), 0o644)
}
