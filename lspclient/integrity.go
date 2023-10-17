package lspclient

import (
	"github.com/btcsuite/btcd/btcec/v2"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/protobuf/proto"
)

func (lc *LspClient) integrityNonce(chanInfo *lspdrpc.ChannelInformationReply, id string) (string, error) {
	log.Trace("integrityNonce")
	// 返信用privkey
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		return "", errFormat(errIntegrityNonce, "NewPrivateKey", err)
	}

	// encrypt
	lspPubkeyBytes, err := btcec.ParsePubKey(chanInfo.LspPubkey)
	if err != nil {
		return "", errFormat(errIntegrityNonce, "ParsePubKey", err)
	}
	reg := &lspdrpc.IntegrityNonceRequest{
		EncryptPubkey: priv.PubKey().SerializeCompressed(),
		Pubkey:        nodePubkeyBytes,
		Id:            id,
	}
	data, _ := proto.Marshal(reg)

	// request
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		return "", errFormat(errIntegrityNonce, "integrityNonce:Encrypt", err)
	}
	req := &lspdrpc.Encrypted{
		Data: encrypted,
	}
	res, err := lc.Client.IntegrityNonce(lc.Ctx, req)
	if err != nil {
		return "", errFormat(errIntegrityNonce, "integrityNonce:call gRPC", err)
	}
	// reply
	decoded, err := Decrypt(priv, res.Data)
	if err != nil {
		return "", errFormat(errIntegrityNonce, "integrityNonce:Decrypt", err)
	}
	var rsp lspdrpc.IntegrityNonceReply
	err = proto.Unmarshal(decoded, &rsp)
	if err != nil {
		return "", errFormat(errIntegrityNonce, "integrityNonce:Unmarshal", err)
	}
	return rsp.Nonce, nil
}

func (lc *LspClient) integrityVerify(chanInfo *lspdrpc.ChannelInformationReply, id string, token string) (lspdrpc.IntegrityResult, error) {
	log.Trace("integrityVerify")

	// encrypt
	lspPubkeyBytes, err := btcec.ParsePubKey(chanInfo.LspPubkey)
	if err != nil {
		log.Errorf("btcec.ParsePubKey(%x) error: %v", chanInfo.LspPubkey, err)
		return lspdrpc.IntegrityResult_INTEGRITYRESULT_NONE, errFormat(errIntegrityVerify, "integrityVerify", err)
	}
	reg := &lspdrpc.IntegrityVerifyRequest{
		Pubkey: nodePubkeyBytes,
		Token:  token,
		Id:     id,
	}
	data, _ := proto.Marshal(reg)

	// request
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		return lspdrpc.IntegrityResult_INTEGRITYRESULT_NONE, errFormat(errIntegrityVerify, "integrityVerify:Encrypt", err)
	}
	req := &lspdrpc.Encrypted{
		Data: encrypted,
	}
	response, err := lc.Client.IntegrityVerify(lc.Ctx, req)
	if err != nil {
		return lspdrpc.IntegrityResult_INTEGRITYRESULT_NONE, errFormat(errIntegrityVerify, "integrityVerify:call gRPC", err)
	}
	return response.Result, nil
}
