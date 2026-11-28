package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/keylightd/pkg/client"
)

// HTTPS administration always uses the local socket, even when normal light
// commands select a remote profile.
func newHTTPSCommand(logger *slog.Logger) *cobra.Command {
	root := &cobra.Command{Use: "https", Short: "Manage daemon HTTPS and client pairing"}
	for _, action := range []string{"status", "enable", "disable", "delete-identity", "invite"} {
		cmd := &cobra.Command{Use: action, Short: map[string]string{"status": "Show listener mode and addresses", "enable": "Activate the saved or newly generated identity", "disable": "Restore the previous listener mode", "delete-identity": "Forget the disabled identity", "invite": "Create a single-use invitation"}[action], Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				socket, _ := cmd.Flags().GetString("socket")
				rpc := "https." + action
				if action == "invite" {
					rpc = "pair.create"
				}
				url, _ := cmd.Flags().GetString("url")
				result, err := client.New(logger, socket).HTTPS(rpc, url)
				if err != nil {
					return err
				}
				if action == "invite" {
					fmt.Fprintln(cmd.OutOrStdout(), result["invitation"])
					return nil
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}}
		if action == "invite" {
			cmd.Flags().String("url", "", "Advertised HTTPS address to include")
		}
		root.AddCommand(cmd)
	}
	pair := &cobra.Command{Use: "pair", Short: "Read an invitation from stdin and save a verified HTTPS profile", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		path, _ := cmd.Flags().GetString("profile")
		if path == "" {
			return errors.New("set --profile to the file for this connection")
		}
		raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 8193))
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		profile, err := client.Redeem(ctx, logger, strings.TrimSpace(string(raw)))
		if err != nil {
			return err
		}
		c, err := client.NewPaired(logger, profile)
		if err != nil {
			return err
		}
		_, err = c.GetLights()
		if err == nil {
			_, err = c.Watch(ctx)
		}
		if err == nil {
			err = saveProfile(path, profile)
		}
		if err != nil {
			_ = c.RevokePairing()
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Paired with", profile.URL)
		return nil
	}}
	root.AddCommand(pair)
	return root
}
func saveProfile(path string, profile client.PairedProfile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".paired-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(profile) //nolint:gosec // The credential belongs in this owner-only client profile.
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
