package lspclient

import (
	"github.com/btcsuite/btcd/btcec/v2"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/protobuf/proto"
)

func (lc *LspClient) registerUserInfo(chanInfo *lspdrpc.ChannelInformationReply, mailAddress string) error {
	lspPubkey := chanInfo.LspPubkey

	// encrypt
	lspPubkeyBytes, err := btcec.ParsePubKey(lspPubkey)
	if err != nil {
		log.Errorf("btcec.ParsePubKey(%x) error: %v", lspPubkey, err)
		return errFormat(errRegisterUserInfo, "registerUserInfo", err)
	}
	reg := &lspdrpc.RegisterUserInfoRequest{
		MailAddress: mailAddress,
	}
	data, _ := proto.Marshal(reg)
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		return errFormat(errRegisterUserInfo, "registerUserInfo:Encrypt", err)
	}
	req := &lspdrpc.Encrypted{
		Data: encrypted,
	}
	_, err = lc.Client.RegisterUserInfo(lc.Ctx, req)
	return err
}
