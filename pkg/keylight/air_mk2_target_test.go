package keylight

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAirMK2AdvertisedIdentity(t *testing.T) {
	record := []string{
		"mf=Elgato", "dt=214", "id=001122aAbBcC",
		"md=Elgato Key Light Air MK.2 20LAM9901", airMK2TLSText,
	}
	require.Equal(t, "00:11:22:aa:bb:cc", airMK2AdvertisedMAC(record))
	for _, invalid := range [][]string{
		{airMK2TLSText},
		{airMK2TLSText, "mf=Elgato", "dt=213", "id=001122aabbcc"},
		{airMK2TLSText, "mf=Elgato", "dt=214", "id=broken"},
		append(append([]string(nil), record...), "id=001122ffffff"),
	} {
		require.Empty(t, airMK2AdvertisedMAC(invalid))
	}
}

func TestAirMK2BluetoothIdentityMatch(t *testing.T) {
	for _, data := range [][]byte{
		{0, 17, 34, 170, 187, 204}, []byte("00:11:22:AA:BB:CC\x00"), []byte("001122AABBCC"),
	} {
		require.True(t, AirMK2BluetoothIdentityMatches("00:11:22:aa:bb:cc", data))
	}
	require.False(t, AirMK2BluetoothIdentityMatches("00:11:22:aa:bb:cc", []byte("00:11:22:ff:ff:ff")))
	require.False(t, AirMK2BluetoothIdentityMatches("", []byte{0, 17, 34, 170, 187, 204}))
}
