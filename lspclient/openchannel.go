package lspclient

import (
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
)

func (lc *LspClient) openChannel(pubkey string) error {
	req := &lspdrpc.OpenChannelRequest{
		Pubkey: pubkey,
	}
	_, err := lc.Client.OpenChannel(lc.Ctx, req)
	if err != nil {
		return errFormat(errOpenChannel, "OpenChannel", err)
	}
	return nil
}
