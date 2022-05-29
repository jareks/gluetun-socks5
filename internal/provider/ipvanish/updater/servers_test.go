package ipvanish

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/qdm12/gluetun/internal/constants/vpn"
	"github.com/qdm12/gluetun/internal/models"
	"github.com/qdm12/gluetun/internal/updater/resolver"
	"github.com/qdm12/gluetun/internal/updater/resolver/mock_resolver"
	"github.com/qdm12/gluetun/internal/updater/unzip/mock_unzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Updater_GetServers(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		// Inputs
		minServers int

		// Mocks
		warnerBuilder func(ctrl *gomock.Controller) Warner

		// Unzip
		unzipContents map[string][]byte
		unzipErr      error

		// Resolution
		expectResolve   bool
		hostsToResolve  []string
		resolveSettings resolver.ParallelSettings
		hostToIPs       map[string][]net.IP
		resolveWarnings []string
		resolveErr      error

		// Output
		servers []models.Server
		err     error
	}{
		"unzipper error": {
			warnerBuilder: func(ctrl *gomock.Controller) Warner { return nil },
			unzipErr:      errors.New("dummy"),
			err:           errors.New("dummy"),
		},
		"not enough unzip contents": {
			minServers:    1,
			warnerBuilder: func(ctrl *gomock.Controller) Warner { return nil },
			unzipContents: map[string][]byte{},
			err:           errors.New("not enough servers found: 0 and expected at least 1"),
		},
		"no openvpn file": {
			minServers:    1,
			warnerBuilder: func(ctrl *gomock.Controller) Warner { return nil },
			unzipContents: map[string][]byte{"somefile.txt": {}},
			err:           errors.New("not enough servers found: 0 and expected at least 1"),
		},
		"invalid proto": {
			minServers: 1,
			warnerBuilder: func(ctrl *gomock.Controller) Warner {
				warner := NewMockWarner(ctrl)
				warner.EXPECT().Warn("unknown protocol: invalid in badproto.ovpn")
				return warner
			},
			unzipContents: map[string][]byte{"badproto.ovpn": []byte(`proto invalid`)},
			err:           errors.New("not enough servers found: 0 and expected at least 1"),
		},
		"no host": {
			minServers: 1,
			warnerBuilder: func(ctrl *gomock.Controller) Warner {
				warner := NewMockWarner(ctrl)
				warner.EXPECT().Warn("remote host not found in nohost.ovpn")
				return warner
			},
			unzipContents: map[string][]byte{"nohost.ovpn": []byte(``)},
			err:           errors.New("not enough servers found: 0 and expected at least 1"),
		},
		"multiple hosts": {
			minServers: 1,
			warnerBuilder: func(ctrl *gomock.Controller) Warner {
				warner := NewMockWarner(ctrl)
				warner.EXPECT().Warn("only using the first host \"hosta\" and discarding 1 other hosts")
				return warner
			},
			unzipContents: map[string][]byte{
				"ipvanish-CA-City-A-hosta.ovpn": []byte("remote hosta\nremote hostb"),
			},
			expectResolve:   true,
			hostsToResolve:  []string{"hosta"},
			resolveSettings: getResolveSettings(1),
			err:             errors.New("not enough servers found: 0 and expected at least 1"),
		},
		"resolve error": {
			warnerBuilder: func(ctrl *gomock.Controller) Warner {
				warner := NewMockWarner(ctrl)
				warner.EXPECT().Warn("resolve warning")
				return warner
			},
			unzipContents: map[string][]byte{
				"ipvanish-CA-City-A-hosta.ovpn": []byte("remote hosta"),
			},
			expectResolve:   true,
			hostsToResolve:  []string{"hosta"},
			resolveSettings: getResolveSettings(0),
			resolveWarnings: []string{"resolve warning"},
			resolveErr:      errors.New("dummy"),
			err:             errors.New("dummy"),
		},
		"filename parsing error": {
			minServers: 1,
			warnerBuilder: func(ctrl *gomock.Controller) Warner {
				warner := NewMockWarner(ctrl)
				warner.EXPECT().Warn("country code is unknown: unknown in ipvanish-unknown-City-A-hosta.ovpn")
				return warner
			},
			unzipContents: map[string][]byte{
				"ipvanish-unknown-City-A-hosta.ovpn": []byte("remote hosta"),
			},
			err: errors.New("not enough servers found: 0 and expected at least 1"),
		},
		"success": {
			minServers: 1,
			warnerBuilder: func(ctrl *gomock.Controller) Warner {
				warner := NewMockWarner(ctrl)
				warner.EXPECT().Warn("resolve warning")
				return warner
			},
			unzipContents: map[string][]byte{
				"ipvanish-CA-City-A-hosta.ovpn": []byte("remote hosta"),
				"ipvanish-LU-City-B-hostb.ovpn": []byte("remote hostb"),
			},
			expectResolve:   true,
			hostsToResolve:  []string{"hosta", "hostb"},
			resolveSettings: getResolveSettings(1),
			hostToIPs: map[string][]net.IP{
				"hosta": {{1, 1, 1, 1}, {2, 2, 2, 2}},
				"hostb": {{3, 3, 3, 3}, {4, 4, 4, 4}},
			},
			resolveWarnings: []string{"resolve warning"},
			servers: []models.Server{
				{
					VPN:      vpn.OpenVPN,
					Country:  "Canada",
					City:     "City A",
					Hostname: "hosta",
					UDP:      true,
					IPs:      []net.IP{{1, 1, 1, 1}, {2, 2, 2, 2}},
				},
				{
					VPN:      vpn.OpenVPN,
					Country:  "Luxembourg",
					City:     "City B",
					Hostname: "hostb",
					UDP:      true,
					IPs:      []net.IP{{3, 3, 3, 3}, {4, 4, 4, 4}},
				},
			},
		},
	}
	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			ctx := context.Background()

			unzipper := mock_unzip.NewMockUnzipper(ctrl)
			const zipURL = "https://www.ipvanish.com/software/configs/configs.zip"
			unzipper.EXPECT().FetchAndExtract(ctx, zipURL).
				Return(testCase.unzipContents, testCase.unzipErr)

			presolver := mock_resolver.NewMockParallel(ctrl)
			if testCase.expectResolve {
				presolver.EXPECT().Resolve(ctx, testCase.hostsToResolve, testCase.resolveSettings).
					Return(testCase.hostToIPs, testCase.resolveWarnings, testCase.resolveErr)
			}

			warner := testCase.warnerBuilder(ctrl)

			updater := New(unzipper, presolver, warner)

			servers, err := updater.GetServers(ctx, testCase.minServers)

			assert.Equal(t, testCase.servers, servers)
			if testCase.err != nil {
				require.Error(t, err)
				assert.Equal(t, testCase.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}