package lspclient

import (
	"bytes"
	"crypto/rand"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/protobuf/proto"
)

func (lc *LspClient) paymentRegister(chanInfo *lspdrpc.ChannelInformationReply, requestAmountSat int64, invoiceMemo string) (string, error) {
	log.Trace("paymentRegister")
	maxReceive, err := receivableMax(lc.LndApi)
	if err != nil {
		log.Errorf("receivableMax error: %v", err)
		return "", errFormat(errPayReg, "receivableMax", err)
	}
	if maxReceive >= requestAmountSat {
		err = fmt.Errorf("there is receivable channel(%v >= %v)", maxReceive, requestAmountSat)
		return "", errFormat(errPayRegReceivable, "paymentRegister", err)
	}

	lspPubkey := chanInfo.LspPubkey
	remotePubkey := chanInfo.Pubkey

	// lnd addinvoice
	paymentPreimage := &lntypes.Preimage{}
	if _, err := rand.Read(paymentPreimage[:]); err != nil {
		return "", errFormat(errPayReg, "preimage", err)
	}

	// invoice for LSP
	feeSat := paymentFee(chanInfo, requestAmountSat)
	incomingSat := requestAmountSat
	outgoingSat := requestAmountSat - feeSat
	routeHint := &lnrpc.HopHint{
		NodeId:                    remotePubkey,
		ChanId:                    333333,
		FeeBaseMsat:               uint32(chanInfo.BaseFeeMsat),
		FeeProportionalMillionths: uint32(chanInfo.FeeRate * 1000000),
		CltvExpiryDelta:           chanInfo.TimeLockDelta,
	}
	invoiceStr, _, paymentAddr, err := lc.LndApi.AddInvoice(outgoingSat, paymentPreimage[:], invoiceMemo, routeHint, nil)
	if err != nil {
		log.Error("paymentRegister: invoice create")
		return "", errFormat(errPayReg, "AddInvoice", err)
	}
	log.Tracef("paymentRegister: invoice for LSP: %s", invoiceStr)
	// invoice for payer
	invoiceStr, paymentHash, paymentAddr2, err := lc.LndApi.AddInvoiceOnlyCreate(incomingSat, paymentPreimage[:], invoiceMemo, routeHint, paymentAddr)
	if err != nil {
		log.Error("paymentRegister: invoice only create")
		return "", errFormat(errPayReg, "AddInvoiceOnlyCreate", err)
	}
	log.Debugf("paymentRegister: invoice for User: %s", invoiceStr)
	if !bytes.Equal(paymentAddr, paymentAddr2) {
		log.Errorf("paymentRegister: paymentAddr not same(%x, %x)", paymentAddr, paymentAddr2)
		return "", errFormat(errPayReg, "payment addr not same", nil)
	}

	// payment register
	info := &lspdrpc.PaymentInformation{
		PaymentHash:        paymentHash,
		PaymentSecret:      paymentAddr,
		Destination:        nodePubkeyBytes,
		IncomingAmountMsat: incomingSat * 1000,
		OutgoingAmountMsat: outgoingSat * 1000,
	}
	data, _ := proto.Marshal(info)

	// encrypt
	lspPubkeyBytes, err := btcec.ParsePubKey(lspPubkey)
	if err != nil {
		log.Errorf("btcec.ParsePubKey(%x) error: %v", lspPubkey, err)
		return "", errFormat(errPayReg, "paymentRegister", err)
	}
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		return "", errFormat(errPayReg, "paymentRegister:Encrypt", err)
	}

	req := &lspdrpc.RegisterPaymentRequest{
		Blob: encrypted,
	}
	_, err = lc.Client.RegisterPayment(lc.Ctx, req)
	if err != nil {
		return "", errFormat(errPayReg, "RegisterPayment", err)
	}
	return invoiceStr, nil
}
