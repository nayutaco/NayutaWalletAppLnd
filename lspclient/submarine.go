package lspclient

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/protobuf/proto"
)

func newSubmarineScript(hash []byte, htlcPubkey []byte, repayPubkey []byte) []byte {
	const (
		OP_IF     = 0x63
		OP_ELSE   = 0x67
		OP_ENDIF  = 0x68
		OP_DROP   = 0x75
		OP_EQUAL  = 0x87
		OP_SHA256 = 0xa8
		OP_CHKSIG = 0xac
		OP_CSV    = 0xb2
	)

	// swap script
	//   OP_SHA256 <sha256(preimage)> OP_EQUAL
	//   OP_IF
	//      <htlcPubkey>
	//   OP_ELSE
	//      <csvHeight> OP_CSV OP_DROP <repayPubkey>
	//   OP_ENDIF
	//   OP_CHKSIG
	script := append([]byte{OP_SHA256, 0x20}, hash...)
	script = append(script, OP_EQUAL, OP_IF, 0x21)
	script = append(script, htlcPubkey...)
	script = append(script, OP_ELSE)
	if csvHeight <= 1 {
		log.Errorf("invalid csvHeight=%v", csvHeight)
		return nil
	} else if csvHeight <= 16 {
		script = append(script, 0x50+byte(csvHeight))
	} else if csvHeight <= 0x4b {
		script = append(script, byte(csvHeight))
	} else if csvHeight <= 0x7f {
		script = append(script, 0x01, byte(csvHeight))
	} else if csvHeight <= 0x7fff {
		script = append(script, 0x02, byte(csvHeight&0xff), byte(csvHeight>>8))
	} else {
		log.Errorf("invalid csvHeight=%v", csvHeight)
		return nil
	}
	script = append(script, OP_CSV, OP_DROP, 0x21)
	script = append(script, repayPubkey...)
	script = append(script, OP_ENDIF, OP_CHKSIG)
	return script
}

func (lc *LspClient) submarineRegister(lspPubkey, paymentHash, repayPubkey []byte) (*SubmarineRegisterResult, error) {
	log.Trace("submarineRegister")
	lspPubkeyBytes, err := btcec.ParsePubKey(lspPubkey)
	if err != nil {
		return nil, errFormat(errSubReg, "ParsePubKey", err)
	}
	// 返信用privkey
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		log.Errorf("submarineRegister: NewPrivateKey err: %v", err)
		return nil, errFormat(errSubReg, "NewPrivateKey", err)
	}

	rs := &lspdrpc.RegisterSubmarineRequest{
		EncryptPubkey:     priv.PubKey().SerializeCompressed(),
		PaymentHash:       paymentHash,
		RepayPubkey:       repayPubkey,
		Destination:       nodePubkeyBytes,
		SwapScriptVersion: scriptVersion,
	}
	data, _ := proto.Marshal(rs)

	// request
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		log.Errorf("submarineRegister: Encrypt err: %v", err)
		return nil, errFormat(errSubReg, "Encrypt", err)
	}
	req := &lspdrpc.Encrypted{
		Data: encrypted,
	}
	res, err := lc.Client.RegisterSubmarine(lc.Ctx, req)
	if err != nil {
		log.Errorf("submarineRegister: RegisterSubmarine err: %v", err)
		return nil, errFormat(errSubReg, "RegisterSubmarine", err)
	}
	// reply
	decoded, err := Decrypt(priv, res.Data)
	if err != nil {
		log.Errorf("submarineRegister: Decrypt err: %v", err)
		return nil, errFormat(errSubReg, "Decrypt", err)
	}
	var rsp lspdrpc.RegisterSubmarineReply
	err = proto.Unmarshal(decoded, &rsp)
	if err != nil {
		log.Errorf("submarineRegister: Unmarshal err: %v", err)
		return nil, errFormat(errSubReg, "Unmarshal", err)
	}

	info, err := lc.LndApi.GetInfo()
	if err != nil {
		log.Errorf("submarineRegister: getInfo err: %v", err)
		return nil, errFormat(errSubReg, "getInfo", err)
	}
	script := newSubmarineScript(rs.PaymentHash, rsp.HtlcPubkey, rs.RepayPubkey)
	scriptAddr, err := lc.LndApi.AddWatchScript(script, info.BlockHash, info.BlockHeight)
	if err != nil {
		log.Errorf("submarineRegister: addWatchScript err: %v", err)
		return nil, errFormat(errSubReg, "addWatchScript", err)
	}
	if scriptAddr != rsp.ScriptAddress {
		log.Errorf("submarineRegister: script address not match: me=%s(%x), hub=%s", scriptAddr, script, rsp.ScriptAddress)
		return nil, errFormat(errSubRegScript, "script address not match", nil)
	}
	result := &SubmarineRegisterResult{
		HtlcPubkey:    rsp.HtlcPubkey,
		Script:        script,
		ScriptAddress: rsp.ScriptAddress,
		Height:        info.BlockHeight,
	}
	log.Debugf("submarineRegister: HtlcPubkey=%x, script=%x, address=%s, height=%v", rsp.HtlcPubkey, script, rsp.ScriptAddress, info.BlockHeight)
	return result, nil
}

func (lc *LspClient) submarineReceive(lspPubkey, paymentHash []byte, invoice string) error {
	log.Trace("submarineReceive")
	lspPubkeyBytes, err := btcec.ParsePubKey(lspPubkey)
	if err != nil {
		log.Errorf("submarineReceive: ParsePubKey err: %v", err)
		return errFormat(errSubRecv, "ParsePubKey", err)
	}

	rs := &lspdrpc.ReceiveSubmarineRequest{
		PaymentHash: paymentHash,
		Invoice:     invoice,
	}
	data, _ := proto.Marshal(rs)

	// request
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		log.Errorf("submarineReceive: Encrypt err: %v", err)
		return errFormat(errSubRecv, "Encrypt", err)
	}
	req := &lspdrpc.Encrypted{
		Data: encrypted,
	}
	_, err = lc.Client.ReceiveSubmarine(lc.Ctx, req)
	if err != nil {
		log.Errorf("submarineReceive: ReceiveSubmarine err: %v", err)
		return errFormat(errSubRecv, "ReceiveSubmarine", err)
	}

	return nil
}

func (lc *LspClient) submarineRepayment(repayParam []byte, repayAddress string, label string) (string, error) {
	log.Trace("submarineRepayment")

	// <witness>
	//   1 + 73(max signature)
	//   1
	//   1 + (script length)
	const SZ_WITNESS = 1 + 73 + 1 + 1 + scriptLength

	log.Debugf("submarineRepayment: SZ_WITNESS=%v, scriptVersion=%v, csvHeight=%v", SZ_WITNESS, scriptVersion, csvHeight)

	decoded := &SubmarineRepayRequest{}
	err := proto.Unmarshal(repayParam, decoded)
	if err != nil {
		log.Errorf("submarineRepayment: Unmarshal err: %v", err)
		return "", errFormat(errSubRepay, "Unmarshal", err)
	}

	info, err := lc.LndApi.GetInfo()
	if err != nil {
		log.Errorf("submarineRepayment: getInfo err: %v", err)
		return "", errFormat(errSubRepay, "getInfo", err)
	}
	chainParams := getChainParams(info.Chains[0].Network)
	if chainParams == nil {
		log.Error("submarineRepayment: getChainParams")
		return "", errFormat(errSubRepay, "getChainParams", nil)
	}
	payAddr, _ := btcutil.DecodeAddress(repayAddress, chainParams)
	payPkScript, _ := txscript.PayToAddrScript(payAddr)
	txHash, err := lc.repaymentWitnessScript(
		decoded.Data,
		payPkScript,
		feeRateSatPerKw,
		int64(SZ_WITNESS*len(decoded.Data)),
		label,
	)
	if err != nil {
		log.Errorf("submarineRepayment: repaymentWitnessScript err: %v", err)
		return "", errFormat(errSubRepay, "repaymentWitnessScript", err)
	}

	return txHash.String(), nil
}

func (lc *LspClient) repaymentWitnessScript(
	repayData []*SubmarineRepayData,
	pkScript []byte,
	satPerKw int64,
	szWitness int64,
	label string,
) (*chainhash.Hash, error) {

	feeRate := chainfee.SatPerKWeight(satPerKw)

	// tx
	//	version: 2
	//	input num: len(repayData)
	//	output num: 1
	outTx := wire.NewMsgTx(2)

	// input
	var totalAmount int64
	wit := make(wire.TxWitness, 1)
	prevOutputFetcher := txscript.NewMultiPrevOutFetcher(nil)
	for idx := 0; idx < len(repayData); idx++ {
		script := repayData[idx].Script
		txid := repayData[idx].Txid
		index := repayData[idx].Index
		amount := repayData[idx].Amount
		log.Debugf("idx: %d", idx)
		log.Debugf("  script: %x", script)
		log.Debugf("  txid: %s", txid)
		log.Debugf("  index: %d", index)
		log.Debugf("  amount: %d", amount)

		totalAmount += amount
		hash, err := chainhash.NewHashFromStr(txid)
		if err != nil {
			log.Errorf("repaymentWitnessScript: NewHashFromStr err: %v", err)
			return nil, fmt.Errorf("repaymentWitnessScript:NewHash %v", err)
		}
		outPoint := wire.NewOutPoint(hash, uint32(index))
		var sig []byte
		var witness [][]byte
		txIn := wire.NewTxIn(outPoint, sig, witness)
		txIn.Sequence = uint32(csvHeight)
		outTx.AddTxIn(txIn)

		// ComputePkScript() use last element of TxWitness[] if TxWitness.length != 2.
		wit[0] = script
		prevPkScript, err := txscript.ComputePkScript(nil, wit)
		if err != nil {
			log.Errorf("fail ComputePkScript: %v", err)
			return nil, err
		}
		prevOutputFetcher.AddPrevOut(*outPoint, &wire.TxOut{
			Value:    amount,
			PkScript: prevPkScript.Script(),
		})
	}

	// output
	txOut := wire.NewTxOut(0, pkScript)
	outTx.AddTxOut(txOut)

	// fee
	weight := int64(4*outTx.SerializeSizeStripped()) + szWitness
	fee := feeRate.FeeForWeight(weight)
	outTx.TxOut[0].Value = int64(totalAmount - int64(fee))

	// witness
	for idx := 0; idx < len(repayData); idx++ {
		repayPrivkey := repayData[idx].Privkey
		script := repayData[idx].Script
		amount := repayData[idx].Amount

		sigHashes := txscript.NewTxSigHashes(outTx, prevOutputFetcher)
		privateKey, _ := btcec.PrivKeyFromBytes(repayPrivkey)
		scriptSig, err := txscript.RawTxInWitnessSignature(outTx, sigHashes, idx, amount, script, txscript.SigHashAll, privateKey)
		if err != nil {
			log.Errorf("repaymentWitnessScript: RawTxInWitnessSignature err: %v", err)
			return nil, fmt.Errorf("repaymentWitnessScript:RawTxInWitnessSignature %v", err)
		}
		outTx.TxIn[idx].Witness = [][]byte{scriptSig, nil, script}
	}

	var buf bytes.Buffer
	err := outTx.Serialize(&buf)
	if err != nil {
		log.Errorf("repaymentWitnessScript: Serialize err: %v", err)
		return nil, fmt.Errorf("repaymentWitnessScript:Serialize %v", err)
	}
	log.Debugf("rawtx: %x", buf.Bytes())
	err = lc.LndApi.PublishTransaction(buf.Bytes(), label)
	if err != nil {
		log.Errorf("repaymentWitnessScript: PublishTransaction err: %v", err)
		return nil, fmt.Errorf("repaymentWitnessScript:PublishTransaction %v", err)
	}
	txHash := outTx.TxHash()
	return &txHash, nil
}

func (lc *LspClient) submarineReregister(script []byte) (string, error) {
	log.Trace("submarineReregister")

	info, err := lc.LndApi.GetInfo()
	if err != nil {
		log.Errorf("submarineReregister: getInfo err: %v", err)
		return "", errFormat(errSubRereg, "getInfo", err)
	}
	scriptAddr, err := lc.LndApi.AddWatchScript(script, info.BlockHash, info.BlockHeight)
	if err != nil {
		log.Errorf("submarineReregister: addWatchScript err: %v", err)
		return "", errFormat(errSubRereg, "addWatchScript", err)
	}
	return scriptAddr, nil
}
