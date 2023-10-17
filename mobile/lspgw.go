//go:build mobile
// +build mobile

package lndmobile

import (
	"github.com/lightningnetwork/lnd/lspclient"
)

// LcGetLspVersion return the LSP version.
func LcGetLspVersion() string {
	return lspclient.GetLspVersion()
}

// LcConnect connect to LSPd gRPC and local LND gRPC.
func LcInit(lspCert, lndCert, lndGrpc, macaroon string, lspGrpc, lspToken string) {
	lspclient.Init(lspCert, lndCert, lndGrpc, macaroon, lspGrpc, lspToken)
}

// LcPing send ping to LSP and receive pong from LSP.
func LcPing(nonce int32) (int32, error) {
	return lspclient.Ping(nonce)
}

// LcGetHubLnNodeString return the HUB LN node connect string(<node_id>@ipaddr:port).
func LcGetHubLnNodeString() (string, error) {
	return lspclient.GetHubLnNodeString()
}

// LcGetLcFeePermyriad return feerate from LSP.
func LcGetLcFeePermyriad() (int64, error) {
	return lspclient.GetLcFeePermyriad()
}

// LcGetLcFeePermyriad return maximum receivable amount.
func LcReceiveMax() (int64, error) {
	return lspclient.ReceiveMax()
}

// LcPaymentFee return fee sats.
func LcPaymentFee(requestAmountSat int64) (int64, error) {
	return lspclient.PaymentFee(requestAmountSat)
}

// LcPaymentRegister register on-the-fly channel creation info and return invoice.
func LcPaymentRegister(requestAmountSat int64, invoiceMemo string) (string, error) {
	return lspclient.PaymentRegister(requestAmountSat, invoiceMemo)
}

// LcSubmarineRefundBlock returns the block number of redeem swap script(OP_CSV value).
func LcSubmarineRefundBlock() int32 {
	return lspclient.SubmarineRefundBlock()
}

// LcSubmarineCreateKeys create keys for submarine swap.
func LcSubmarineCreateKeys() ([]byte, error) {
	return lspclient.SubmarineCreateKeys()
}

// LcSubmarineRegister register submarine swap info.
//
//	@return protobuf(SubmarineRegisterResult)
func LcSubmarineRegister(paymentHash, repayPubkey []byte) ([]byte, error) {
	return lspclient.SubmarineRegister(paymentHash, repayPubkey)
}

// LcSubmarineReceive start submarine swap.
func LcSubmarineReceive(paymentHash []byte, invoice string) error {
	return lspclient.SubmarineReceive(paymentHash, invoice)
}

// LcSubmarineRepayment refund from swap script.
//
//	@param repayParam protobuf(SubmarineRepayRequest)
//	@return TXID
func LcSubmarineRepayment(
	repayParam []byte,
	repayAddress string,
	label string,
) (string, error) {
	return lspclient.SubmarineRepayment(repayParam, repayAddress, label)
}

// LcSubmarineReregister register swap script(recovery).
func LcSubmarineReregister(script []byte) (string, error) {
	return lspclient.SubmarineReregister(script)
}

// LcSelfRebalance aggregate balances from channels and close unused channels.
func LcSelfRebalance() error {
	return lspclient.SelfRebalance()
}

// LcQueryRoutePayment pay from routing info created by HUB.
func LcQueryRoutePayment(invoice string, feeLimitSat int32, amtSat int64) ([]byte, error) {
	return lspclient.QueryRoutePayment(invoice, feeLimitSat, amtSat)
}

// LcRegisterUserInfo register a user data to LSP database.
func LcRegisterUserInfo(mailAddress string) error {
	return lspclient.RegisterUserInfo(mailAddress)
}

// LcRequestOpenChannel request OpenChannel from Hub.
func LcRequestOpenChannel() error {
	return lspclient.RequestOpenChannel()
}

// LcIntegrityNonce return nonce value for Integrity API.
func LcIntegrityNonce(id string) (string, error) {
	return lspclient.IntegrityNonce(id)
}

// LcIntegrityVerify verify Integrity token.
func LcIntegrityVerify(id string, token string) (int32, error) {
	verify, err := lspclient.IntegrityVerify(id, token)
	return int32(verify), err
}
