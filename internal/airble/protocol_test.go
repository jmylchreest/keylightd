package airble

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

type scriptedCharacteristic struct {
	write func([]byte)
	read  func() []byte
}

func (s scriptedCharacteristic) Write(value []byte) error {
	s.write(value)
	return nil
}

func (s scriptedCharacteristic) Read(dst []byte) (int, error) {
	return copy(dst, s.read()), nil
}

type scriptedNotifications struct {
	receive func([]byte)
}

func (s *scriptedNotifications) Notifications(receive func([]byte)) error {
	s.receive = receive
	return nil
}

func TestCertificateAndWiFiSetup(t *testing.T) {
	var command string
	var installed []byte
	var kinds []string
	control := scriptedCharacteristic{
		write: func(value []byte) { command = string(value) },
		read: func() []byte {
			switch command {
			case "CMD_INIT":
				return []byte("ACK_READY\x00")
			case "CMD_INST_STATE":
				require.Equal(t, []string{"CACR", "SRVC", "TLSK"}, kinds)
				return []byte("ACK_CACR_1_SRVC_1_TLSK_1")
			default:
				t.Fatalf("unexpected command %q", command)
				return nil
			}
		},
	}
	var sequence byte
	upload := scriptedCharacteristic{write: func(frame []byte) {
		require.Equal(t, "CMD_INIT", command)
		require.Equal(t, len(frame), int(frame[0]))
		require.Equal(t, sequence, frame[1]&0x7f)
		installed = append(installed, frame[2:]...)
		sequence++
		if frame[1]&0x80 == 0 {
			require.Len(t, frame, 244)
			return
		}
		require.Equal(t, len(installed)-12, int(binary.LittleEndian.Uint32(installed[4:8])))
		require.Equal(t, crc16(installed[:10]), binary.LittleEndian.Uint16(installed[10:12]))
		require.Equal(t, crc16(installed[12:]), binary.LittleEndian.Uint16(installed[8:10]))
		kinds = append(kinds, string(installed[:4]))
		installed = nil
		sequence = 0
	}}
	require.NoError(t, transferCertificates(t.Context(), control, upload, bytes.Repeat([]byte("ca"), 200), []byte("server"), []byte("key")))

	frame, err := wifiFrame(200, "test", "password")
	require.NoError(t, err)
	address := &scriptedNotifications{}
	wifi := scriptedCharacteristic{write: func(data []byte) {
		require.Equal(t, "CMD_INST_STATE", command)
		require.Equal(t, "15c800040008007465737470617373776f7264", hex.EncodeToString(data[:len(data)-2]))
		require.Equal(t, crc16(data[1:len(data)-2]), binary.LittleEndian.Uint16(data[len(data)-2:]))
		address.receive([]byte{192, 0, 2, 3})
	}}
	ip, err := joinWiFi(t.Context(), wifi, nil, address, frame)
	require.NoError(t, err)
	require.Equal(t, "192.0.2.3", ip)
	require.Nil(t, address.receive)
}

func TestPairingRejectionAndCancellation(t *testing.T) {
	require.Equal(t, uint16(0x2189), crc16([]byte("123456789")))
	require.False(t, certificatesInstalled("ACK_CACR_1_SRVC_1_TLSK_0"))
	require.False(t, certificatesInstalled("ACK_READY"))

	control := scriptedCharacteristic{write: func([]byte) {}, read: func() []byte { return []byte("NAK") }}
	require.ErrorContains(t, transferCertificates(t.Context(), control, nil, nil, nil, nil), "light rejected certificate command")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := joinWiFi(ctx, nil, nil, nil, nil)
	require.ErrorIs(t, err, context.Canceled)
}
