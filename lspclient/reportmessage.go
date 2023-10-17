package lspclient

import (
	"github.com/btcsuite/btcd/btcec/v2"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/protobuf/proto"
)

const (
	reportCategory = "QueryRoutes"
)

func (lc *LspClient) reportMessage(chanInfo *lspdrpc.ChannelInformationReply, category string, level lspdrpc.ReportRequest_ReportLevel, message string) error {
	lspPubkey := chanInfo.LspPubkey

	// encrypt
	lspPubkeyBytes, err := btcec.ParsePubKey(lspPubkey)
	if err != nil {
		log.Errorf("btcec.ParsePubKey(%x) error: %v", lspPubkey, err)
		return errFormat(errRegisterUserInfo, "reportMessage", err)
	}
	reg := &lspdrpc.ReportRequest{
		Category: category,
		Level:    level,
		Message:  message,
	}
	data, _ := proto.Marshal(reg)
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		return errFormat(errRegisterUserInfo, "reportMessage:Encrypt", err)
	}
	req := &lspdrpc.Encrypted{
		Data: encrypted,
	}
	_, err = lc.Client.ReportMessage(lc.Ctx, req)
	return err
}
