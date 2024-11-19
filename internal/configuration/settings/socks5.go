package settings

import (
	"fmt"
	"os"
	"time"

	"github.com/qdm12/gosettings"
	"github.com/qdm12/gosettings/reader"
	"github.com/qdm12/gosettings/validate"
	"github.com/qdm12/gotree"
)

// Socks5 contains settings to configure the Socks5 proxy.
type Socks5 struct {
	// User is the username to use for the Socks5 proxy.
	// It cannot be nil in the internal state.
	User *string
	// Password is the password to use for the Socks5 proxy.
	// It cannot be nil in the internal state.
	Password *string
	// ListeningAddress is the listening address
	// of the Socks5 proxy server.
	// It cannot be the empty string in the internal state.
	ListeningAddress string
	// Enabled is true if the Socks5 proxy server should run,
	// and false otherwise. It cannot be nil in the
	// internal state.
	Enabled *bool
	// ReadTimeout is the Socks5 read timeout duration
	// of the Socks5 server. It defaults to 3 seconds if left unset.
	ReadTimeout time.Duration
}

func (h Socks5) validate() (err error) {
	// Do not validate user and password
	err = validate.ListeningAddress(h.ListeningAddress, os.Getuid())
	if err != nil {
		return fmt.Errorf("%w: %s", ErrServerAddressNotValid, h.ListeningAddress)
	}

	return nil
}

func (h *Socks5) copy() (copied Socks5) {
	return Socks5{
		User:              gosettings.CopyPointer(h.User),
		Password:          gosettings.CopyPointer(h.Password),
		ListeningAddress:  h.ListeningAddress,
		Enabled:           gosettings.CopyPointer(h.Enabled),
		ReadTimeout:       h.ReadTimeout,
	}
}

// overrideWith overrides fields of the receiver
// settings object with any field set in the other
// settings.
func (h *Socks5) overrideWith(other Socks5) {
	h.User = gosettings.OverrideWithPointer(h.User, other.User)
	h.Password = gosettings.OverrideWithPointer(h.Password, other.Password)
	h.ListeningAddress = gosettings.OverrideWithComparable(h.ListeningAddress, other.ListeningAddress)
	h.Enabled = gosettings.OverrideWithPointer(h.Enabled, other.Enabled)
	h.ReadTimeout = gosettings.OverrideWithComparable(h.ReadTimeout, other.ReadTimeout)
}

func (h *Socks5) setDefaults() {
	h.User = gosettings.DefaultPointer(h.User, "")
	h.Password = gosettings.DefaultPointer(h.Password, "")
	h.ListeningAddress = gosettings.DefaultComparable(h.ListeningAddress, "0.0.0.0:1080")
	h.Enabled = gosettings.DefaultPointer(h.Enabled, false)
	const defaultReadTimeout = 3 * time.Second
	h.ReadTimeout = gosettings.DefaultComparable(h.ReadTimeout, defaultReadTimeout)
}

func (h Socks5) String() string {
	return h.toLinesNode().String()
}

func (h Socks5) toLinesNode() (node *gotree.Node) {
	node = gotree.New("Socks5server settings:")
	node.Appendf("Enabled: %s", gosettings.BoolToYesNo(h.Enabled))
	if !*h.Enabled {
		return node
	}

	node.Appendf("Listening address: %s", h.ListeningAddress)
	node.Appendf("User: %s", *h.User)
	node.Appendf("Password: %s", gosettings.ObfuscateKey(*h.Password))
	node.Appendf("Read timeout: %s", h.ReadTimeout)

	return node
}

func (h *Socks5) read(r *reader.Reader) (err error) {
	h.User = r.Get("SOCKS5SERVER_USER",
		// reader.RetroKeys("PROXY_USER", "TINYPROXY_USER"),
		reader.ForceLowercase(false))

	h.Password = r.Get("SOCKS5SERVER_PASSWORD",
		// reader.RetroKeys("PROXY_PASSWORD", "TINYPROXY_PASSWORD"),
		reader.ForceLowercase(false))

	h.ListeningAddress, err = readSocks5ProxyListeningAddress(r)
	if err != nil {
		return err
	}

	h.Enabled, err = r.BoolPtr("SOCKS5SERVER")
	if err != nil {
		return err
	}

	return nil
}

func readSocks5ProxyListeningAddress(r *reader.Reader) (listeningAddress string, err error) {
	// Retro-compatible keys using a port only
	port, err := r.Uint16Ptr("",
		reader.RetroKeys("SOCKS5_PROXY_PORT"),
		reader.IsRetro("SOCKS5_PROXY_LISTENING_ADDRESS"))
	if err != nil {
		return "", err
	} else if port != nil {
		return fmt.Sprintf(":%d", *port), nil
	}
	const currentKey = "SOCKS5_LISTENING_ADDRESS"
	return r.String(currentKey), nil
}
