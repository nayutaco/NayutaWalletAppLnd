package lspclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	reflect "reflect"
	sync "sync"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/lntypes"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const (
	rebalanceRetryClose = 3

	// from lnd.conf
	lndConfBitcoinTimeLockDelay = 40 // bitcoin.timelockdelay

	// submarine
	scriptVersion   = 2
	csvHeight       = 144 // submarine swap可能期間(block)
	scriptLength    = 110 // waitHeightが1byte分の場合
	feeRateSatPerKw = int64(300)
)

type FeeHint struct {
	FeeBaseMsat               int64
	FeeProportionalMillionths int64
	TimeLockDelta             uint32
}

type LspClient struct {
	Conn   *grpc.ClientConn
	Client lspdrpc.LightningServiceClient
	Ctx    context.Context
	Cancel context.CancelFunc
	LndApi LndApiConnector
}

type LockChanInfo struct {
	lock sync.RWMutex

	// LSP ChannelInformation
	//	set:   newLspClient() if chanInfo is nil
	//	clear: fail Ping()
	chanInfo *lspdrpc.ChannelInformationReply
}

var (
	lspCert     string
	lndCert     string
	lndAddr     string
	lndMacaroon string
	lspAddr     string
	lspToken    string

	// local LND node_id
	nodePubkeyBytes []byte
	lockChanInfo    LockChanInfo
)

func newLspClient() (*LspClient, error) {
	lnd, err := newLndApi(lndCert, lndAddr, lndMacaroon)
	if err != nil {
		return nil, err
	}
	if nodePubkeyBytes == nil {
		info, err := lnd.GetInfo()
		if err != nil {
			lnd.Disconnect()
			return nil, errFormat(errNewLspClient, "getInfo", err)
		}
		nodePubkeyBytes, err = hex.DecodeString(info.IdentityPubkey)
		if err != nil {
			lnd.Disconnect()
			return nil, errFormat(errNewLspClient, "DecodeString", err)
		}
	}
	lspClient := LspClient{
		LndApi: lnd,
	}
	err = lspClient.connecToLsp(lspAddr, lspToken, lspCert)
	if err != nil {
		lnd.Disconnect()
		return nil, errFormat(errNewLspClient, "connecToLsp", err)
	}
	lockChanInfo.lock.Lock()
	defer lockChanInfo.lock.Unlock()
	if lockChanInfo.chanInfo == nil {
		lockChanInfo.chanInfo, err = lspClient.channelInfo()
		if err != nil {
			lspClient.disconnect()
			return nil, errFormat(errNewLspClient, "channelInfo", err)
		}
	}
	log.Tracef("Connect done: %v", lockChanInfo.chanInfo)
	return &lspClient, nil
}

// for invoicerpc.AddInvoice
func LspFeeHint(peerNode []byte) *FeeHint {
	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	if lockChanInfo.chanInfo == nil {
		log.Error("LspFeeHint: chanInfo is nil")
		return nil
	}
	pubkey, err := hex.DecodeString(lockChanInfo.chanInfo.GetPubkey())
	if err != nil || !reflect.DeepEqual(pubkey, peerNode) {
		// not LSP
		return nil
	}
	return &FeeHint{
		FeeBaseMsat:               lockChanInfo.chanInfo.BaseFeeMsat,
		FeeProportionalMillionths: int64(lockChanInfo.chanInfo.FeeRate * 1000000),
		TimeLockDelta:             lockChanInfo.chanInfo.TimeLockDelta,
	}
}

func Init(lspCert_, lndCert_, lndAddr_, macaroon, lspAddr_, token string) {
	lspCert = lspCert_
	lndCert = lndCert_
	lndAddr = lndAddr_
	lndMacaroon = macaroon
	lspAddr = lspAddr_
	lspToken = token
	nodePubkeyBytes = nil
}

func GetLspVersion() string {
	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	if lockChanInfo.chanInfo == nil {
		log.Error("GetVersion: chanInfo is nil")
		return ""
	}
	return lockChanInfo.chanInfo.Version
}

func Ping(nonce int32) (int32, error) {
	lsp, err := newLspClient()
	if err != nil {
		return 0, err
	}
	defer lsp.disconnect()

	reply, err := lsp.ping(nonce)
	if err != nil {
		log.Info("Ping: clear chanInfo")
		lockChanInfo.lock.Lock()
		defer lockChanInfo.lock.Unlock()
		lockChanInfo.chanInfo = nil
	}
	return reply, err
}

func GetHubLnNode() string {
	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	if lockChanInfo.chanInfo == nil {
		log.Error("GetHubLnNode: chanInfo is nil")
		return ""
	}
	return lockChanInfo.chanInfo.Pubkey
}

func GetHubLnNodeString() (string, error) {
	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	if lockChanInfo.chanInfo == nil {
		log.Error("GetHubLnNodeString: chanInfo is nil")
		return "", errFormat(errChanInfoNil, "GetHubLnNodeString", nil)
	}
	return fmt.Sprintf("%s@%s", lockChanInfo.chanInfo.Pubkey, lockChanInfo.chanInfo.Host), nil
}

func GetLcFeePermyriad() (int64, error) {
	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	if lockChanInfo.chanInfo == nil {
		log.Error("GetLcFeePermyriad: chanInfo is nil")
		return 0, errFormat(errChanInfoNil, "GetLcFeePermyriad", nil)
	}
	return lockChanInfo.chanInfo.ChannelFeePermyriad, nil
}

func ReceiveMax() (int64, error) {
	lnd, err := newLndApi(lndCert, lndAddr, lndMacaroon)
	if err != nil {
		return 0, err
	}
	defer lnd.Disconnect()
	maxReceive, err := receivableMax(lnd)
	if err != nil {
		return 0, errFormat(errPayReg, "receivableMax", err)
	}
	return maxReceive, nil
}

func PaymentFee(requestAmountSat int64) (int64, error) {
	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	if lockChanInfo.chanInfo == nil {
		log.Error("PaymentFee: chanInfo is nil")
		return 0, errFormat(errChanInfoNil, "paymentFee", nil)
	}

	return paymentFee(lockChanInfo.chanInfo, requestAmountSat), nil
}

func PaymentRegister(requestAmountSat int64, invoiceMemo string) (string, error) {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("PaymentRegister: newLspClient err=%v", err)
		return "", err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.paymentRegister(lockChanInfo.chanInfo, requestAmountSat, invoiceMemo)
}

func SubmarineRefundBlock() int32 {
	return int32(csvHeight)
}

func SubmarineCreateKeys() ([]byte, error) {
	preimage := &lntypes.Preimage{}
	if _, err := rand.Read(preimage[:]); err != nil {
		return nil, errFormat(errSubCreateKeys, "Read", err)
	}
	paymentHash := preimage.Hash()

	repayPriv, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, errFormat(errSubCreateKeys, "NewPrivateKey", err)
	}
	repayPubkey := repayPriv.PubKey().SerializeCompressed()

	res := &SubmarineCreateKeysResult{
		Preimage:     preimage[:],
		PaymentHash:  paymentHash[:],
		RepayPrivkey: repayPriv.Serialize(),
		RepayPubkey:  repayPubkey,
	}
	resPb, _ := proto.Marshal(res)
	return resPb, nil
}

func SubmarineRegister(paymentHash, repayPubkey []byte) ([]byte, error) {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("SubmarineRegister: newLspClient err=%v", err)
		return nil, err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	res, err := lsp.submarineRegister(lockChanInfo.chanInfo.LspPubkey, paymentHash, repayPubkey)
	if err != nil {
		return nil, err
	}
	resPb, _ := proto.Marshal(res)
	return resPb, nil
}

func SubmarineReceive(paymentHash []byte, invoice string) error {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("SubmarineReceive: newLspClient err=%v", err)
		return err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.submarineReceive(lockChanInfo.chanInfo.LspPubkey, paymentHash, invoice)
}

func SubmarineRepayment(
	repayParam []byte,
	repayAddress string,
	label string,
) (string, error) {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("SubmarineRepayment: newLspClient err=%v", err)
		return "", err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.submarineRepayment(repayParam, repayAddress, label)
}

func SubmarineReregister(script []byte) (string, error) {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("SubmarineReregister: newLspClient err=%v", err)
		return "", err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.submarineReregister(script)
}

func SelfRebalance() error {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("SelfRebalance: newLspClient err=%v", err)
		return err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.selfRebalance(lockChanInfo.chanInfo)
}

func QueryRoutePayment(invoice string, feeLimitSat int32, amtSat int64) ([]byte, error) {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("QueryRoutePayment: newLspClient err=%v", err)
		return nil, err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	paymentHash, status, failure, err := lsp.queryRoutePayment(lockChanInfo.chanInfo, invoice, feeLimitSat, amtSat)
	if err != nil {
		return nil, err
	}
	res := &QueryRoutePaymentResult{
		PaymentHash: paymentHash,
		Status:      int32(status),
		Failure:     int32(failure),
	}
	resPb, _ := proto.Marshal(res)
	return resPb, nil
}

func RegisterUserInfo(mailAddress string) error {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("RegisterUserInfo: newLspClient err=%v", err)
		return err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.registerUserInfo(lockChanInfo.chanInfo, mailAddress)
}

func ReportMessage(category string, level lspdrpc.ReportRequest_ReportLevel, message string) error {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("ReportMessage: newLspClient err=%v", err)
		return err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.reportMessage(lockChanInfo.chanInfo, category, level, message)
}

func RequestOpenChannel() error {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("RegisterUserInfo: newLspClient err=%v", err)
		return err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.openChannel(hex.EncodeToString(nodePubkeyBytes))
}

func IntegrityNonce(id string) (string, error) {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("IntegrityNonce: newLspClient err=%v", err)
		return "", err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.integrityNonce(lockChanInfo.chanInfo, id)
}

func IntegrityVerify(id string, token string) (lspdrpc.IntegrityResult, error) {
	lsp, err := newLspClient()
	if err != nil {
		log.Errorf("IntegrityVerify: newLspClient err=%v", err)
		return lspdrpc.IntegrityResult_INTEGRITYRESULT_NONE, err
	}
	defer lsp.disconnect()

	lockChanInfo.lock.RLock()
	defer lockChanInfo.lock.RUnlock()
	return lsp.integrityVerify(lockChanInfo.chanInfo, id, token)
}
