package lspclient

import (
	"encoding/hex"

	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
)

func (lc *LspClient) channelInfo() (*lspdrpc.ChannelInformationReply, error) {
	log.Trace("channelInfo")
	in := &lspdrpc.ChannelInformationRequest{
		Pubkey: hex.EncodeToString(nodePubkeyBytes),
	}
	info, err := lc.Client.ChannelInformation(lc.Ctx, in)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (lc *LspClient) ping(nonce int32) (int32, error) {
	log.Trace("ping")
	in := &lspdrpc.PingRequest{
		Nonce: nonce,
	}
	res, err := lc.Client.Ping(lc.Ctx, in)
	if err != nil {
		return 0, errFormat(errPing, "ping", err)
	}
	return res.Nonce, nil
}
