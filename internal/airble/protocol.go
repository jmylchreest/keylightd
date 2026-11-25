package airble

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const certificatePayload = 242

type characteristicIO interface {
	Write([]byte) error
	Read([]byte) (int, error)
}

func crc16(data []byte) uint16 {
	var crc uint16
	for _, value := range data {
		crc ^= uint16(value)
		for range 8 {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x8408
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func certificateFrames(kind string, pem []byte) ([][]byte, error) {
	if len(kind) != 4 || len(pem) == 0 || len(pem) > 128*certificatePayload-12 {
		return nil, errors.New("invalid certificate transfer size or type")
	}
	block := make([]byte, 12+len(pem))
	copy(block, kind)
	binary.LittleEndian.PutUint32(block[4:], uint32(len(pem)))
	binary.LittleEndian.PutUint16(block[8:], crc16(pem))
	binary.LittleEndian.PutUint16(block[10:], crc16(block[:10]))
	copy(block[12:], pem)

	var frames [][]byte
	for len(block) > 0 {
		size := min(len(block), certificatePayload)
		sequence := byte(len(frames))
		if size == len(block) {
			sequence |= 0x80
		}
		frame := []byte{byte(size + 2), sequence}
		frames = append(frames, append(frame, block[:size]...))
		block = block[size:]
	}
	return frames, nil
}

func wifiFrame(security uint16, ssid, password string) ([]byte, error) {
	if len(ssid) == 0 || len(ssid) > 32 || len(password) > 64 {
		return nil, errors.New("invalid Wi-Fi credential length")
	}
	frame := make([]byte, 9+len(ssid)+len(password))
	frame[0] = byte(len(frame))
	binary.LittleEndian.PutUint16(frame[1:], security)
	binary.LittleEndian.PutUint16(frame[3:], uint16(len(ssid)))
	binary.LittleEndian.PutUint16(frame[5:], uint16(len(password)))
	copy(frame[7:], ssid)
	copy(frame[7+len(ssid):], password)
	end := len(frame) - 2
	binary.LittleEndian.PutUint16(frame[end:], crc16(frame[1:end]))
	return frame, nil
}

func certificatesInstalled(response string) bool {
	fields := strings.Split(response, "_")
	if fields[0] != "ACK" || len(fields)%2 != 1 {
		return false
	}
	installed := make(map[string]bool, 3)
	for i := 1; i < len(fields); i += 2 {
		installed[fields[i]] = fields[i+1] == "1"
	}
	return installed["CACR"] && installed["SRVC"] && installed["TLSK"]
}

func transferCertificates(ctx context.Context, command, install characteristicIO, ca, cert, key []byte) error {
	transferCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := waitForCommand(transferCtx, command, "CMD_INIT", func(s string) bool { return s == "ACK_READY" }); err != nil {
		return fmt.Errorf("initialize certificate transfer: %w", err)
	}
	for _, item := range []struct {
		kind string
		pem  []byte
	}{{"CACR", ca}, {"SRVC", cert}, {"TLSK", key}} {
		frames, err := certificateFrames(item.kind, item.pem)
		if err != nil {
			return err
		}
		for _, frame := range frames {
			if err := transferCtx.Err(); err != nil {
				return err
			}
			if err := install.Write(frame); err != nil {
				return fmt.Errorf("write %s certificate frame: %w", item.kind, err)
			}
			if err := pause(transferCtx, 100*time.Millisecond); err != nil {
				return err
			}
		}
	}
	if err := waitForCommand(transferCtx, command, "CMD_INST_STATE", certificatesInstalled); err != nil {
		return fmt.Errorf("verify certificate installation: %w", err)
	}
	return nil
}

func waitForCommand(ctx context.Context, characteristic characteristicIO, request string, accepted func(string) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := characteristic.Write([]byte(request)); err != nil {
		return err
	}
	var value [128]byte
	for {
		n, err := characteristic.Read(value[:])
		if err != nil {
			return err
		}
		response := strings.Trim(string(value[:n]), "\x00 \r\n")
		if strings.HasPrefix(response, "NAK") {
			return errors.New("light rejected certificate command")
		}
		if accepted(response) {
			return nil
		}
		if err := pause(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
}

func pause(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type notifyingCharacteristic interface {
	Notifications(func([]byte)) error
}

func joinWiFi(ctx context.Context, credentials, state characteristicIO, address notifyingCharacteristic, frame []byte) (string, error) {
	joinCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := joinCtx.Err(); err != nil {
		return "", err
	}
	addresses := make(chan string, 1)
	if err := address.Notifications(func(value []byte) {
		if len(value) < 4 {
			return
		}
		ip := net.IP(value[:4])
		if ip.IsGlobalUnicast() {
			select {
			case addresses <- ip.String():
			default:
			}
		}
	}); err != nil {
		return "", fmt.Errorf("subscribe to Wi-Fi address: %w", err)
	}
	defer func() { _ = address.Notifications(nil) }()

	if err := credentials.Write(frame); err != nil {
		return "", fmt.Errorf("write Wi-Fi credentials: %w", err)
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var value [128]byte
	for {
		select {
		case <-joinCtx.Done():
			return "", joinCtx.Err()
		case ip := <-addresses:
			return ip, nil
		case <-ticker.C:
			n, err := state.Read(value[:])
			if err != nil {
				return "", fmt.Errorf("read Wi-Fi state: %w", err)
			}
			if n > 0 && value[0] >= 100 {
				return "", fmt.Errorf("light reported Wi-Fi setup failure (state %d)", value[0])
			}
		}
	}
}
