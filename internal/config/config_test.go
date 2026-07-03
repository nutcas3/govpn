package config_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/govpn/govpn/internal/config"
)

func TestValidateServer(t *testing.T) {
	t.Parallel()
	c := config.ExampleServer()
	if err := c.Validate(); err != nil {
		t.Fatalf("valid server config failed validation: %v", err)
	}
}

func TestValidateClient(t *testing.T) {
	t.Parallel()
	c := config.ExampleClient()
	if err := c.Validate(); err != nil {
		t.Fatalf("valid client config failed validation: %v", err)
	}
}

func TestValidateSentinels(t *testing.T) {
	t.Parallel()

	base := config.ExampleServer()

	cases := []struct {
		name    string
		mutate  func(*config.Config)
		wantErr error
	}{
		{
			name:    "unknown mode",
			mutate:  func(c *config.Config) { c.Mode = "proxy" },
			wantErr: config.ErrUnknownMode,
		},
		{
			name:    "empty passphrase",
			mutate:  func(c *config.Config) { c.Passphrase = "" },
			wantErr: config.ErrEmptyPassphrase,
		},
		{
			name:    "empty local_ip",
			mutate:  func(c *config.Config) { c.LocalIP = "" },
			wantErr: config.ErrEmptyLocalIP,
		},
		{
			name:    "server missing listen_addr",
			mutate:  func(c *config.Config) { c.ListenAddr = "" },
			wantErr: config.ErrMissingListenAddr,
		},
		{
			name:    "invalid mtu too low",
			mutate:  func(c *config.Config) { c.MTU = 100 },
			wantErr: config.ErrInvalidMTU,
		},
		{
			name:    "invalid mtu too high",
			mutate:  func(c *config.Config) { c.MTU = 70000 },
			wantErr: config.ErrInvalidMTU,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := json.Marshal(base)
			var c config.Config
			_ = json.Unmarshal(b, &c)
			tc.mutate(&c)
			err := c.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Validate() = %v; want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestLoadRoundTrip(t *testing.T) {
	t.Parallel()

	src := config.ExampleServer()
	data, _ := json.MarshalIndent(src, "", "  ")

	f, err := os.CreateTemp(t.TempDir(), "govpn-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := config.Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Passphrase != src.Passphrase {
		t.Errorf("passphrase: got %q, want %q", got.Passphrase, src.Passphrase)
	}
	if got.MTU != src.MTU {
		t.Errorf("mtu: got %d, want %d", got.MTU, src.MTU)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := config.Load("/nonexistent/path.json"); err == nil {
		t.Error("Load of nonexistent file should return an error")
	}
}

// TestDurationJSON validates that Duration round-trips through JSON correctly.
// We compare the parsed time.Duration values, not the raw strings, because
// time.Duration.String() normalises "2m" → "2m0s".
func TestDurationJSON(t *testing.T) {
	t.Parallel()

	cases := []time.Duration{
		time.Second,
		30 * time.Second,
		2 * time.Minute,
		90 * time.Minute,
	}

	for _, want := range cases {
		want := want
		t.Run(want.String(), func(t *testing.T) {
			t.Parallel()

			d := config.Duration{Duration: want}
			b, err := d.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON: %v", err)
			}

			var got config.Duration
			if err := got.UnmarshalJSON(b); err != nil {
				t.Fatalf("UnmarshalJSON: %v", err)
			}

			if got.Duration != want {
				t.Errorf("round-trip: got %v, want %v", got.Duration, want)
			}
		})
	}
}
