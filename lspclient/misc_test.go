package lspclient

import (
	"fmt"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
)

type mockLndApi struct {
	LndApiConnector

	fakeListChannels func() (*lnrpc.ListChannelsResponse, error)
}

func (m *mockLndApi) ListChannels() (*lnrpc.ListChannelsResponse, error) {
	return m.fakeListChannels()
}

func TestReceiveMaxError(t *testing.T) {
	fakeApi := &mockLndApi{
		fakeListChannels: func() (*lnrpc.ListChannelsResponse, error) {
			return nil, fmt.Errorf("something error")
		},
	}
	_, err := receivableMax(fakeApi)
	if err == nil {
		t.Errorf("Error must occur")
	}
}

func TestReceiveMax(t *testing.T) {
	data := []struct {
		name     string
		channels []*lnrpc.Channel
		expected int64
	}{
		{
			name: "1 channel",
			channels: []*lnrpc.Channel{
				{
					RemoteBalance: 5000,
					CommitFee:     10,
					RemoteConstraints: &lnrpc.ChannelConstraints{
						ChanReserveSat: 1000,
					},
				},
			},
			expected: 5000 - 1000 - 10/2,
		},
		{
			name: "2 channels",
			channels: []*lnrpc.Channel{
				{
					RemoteBalance: 5000,
					CommitFee:     10,
					RemoteConstraints: &lnrpc.ChannelConstraints{
						ChanReserveSat: 1000,
					},
				},
				{
					RemoteBalance: 10000,
					CommitFee:     100,
					RemoteConstraints: &lnrpc.ChannelConstraints{
						ChanReserveSat: 2000,
					},
				},
			},
			expected: 10000 - 2000 - 100/2,
		},
	}

	var channels []*lnrpc.Channel
	fakeApi := &mockLndApi{
		fakeListChannels: func() (*lnrpc.ListChannelsResponse, error) {
			return &lnrpc.ListChannelsResponse{Channels: channels}, nil
		},
	}
	for _, tt := range data {
		t.Run(tt.name, func(t *testing.T) {
			channels = tt.channels
			result, err := receivableMax(fakeApi)
			if err != nil {
				t.Errorf("err: %v", err)
			}
			if result != tt.expected {
				t.Errorf("[%s]expected=%v, result=%v", tt.name, tt.expected, result)
			}
		})
	}
}
