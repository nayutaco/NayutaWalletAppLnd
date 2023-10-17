package lspclient

import (
	"errors"
	"fmt"
)

const (
	errChanInfoNil = 1

	errNewLspClient        = 100
	errNewLspClientConnect = errNewLspClient + 1

	errPing = 200

	errPayReg           = 300
	errPayRegReceivable = errPayReg + 1

	errSubCreateKeys = 400

	errSubReg       = 500
	errSubRegScript = errSubReg + 1

	errSubRecv = 600

	errSubRepay = 700

	errSubRereg = 800

	errSelfRebalance = 900

	errQueryRoutes      = 1000
	errQueryRoutesRoute = errQueryRoutes + 1
	errQueryRoutesPay   = errQueryRoutes + 2

	errRegisterUserInfo = 1100

	errIntegrity       = 1200
	errIntegrityNonce  = errIntegrity + 1
	errIntegrityVerify = errIntegrity + 2

	errOpenChannel = 1300
)

var (
	errQueryRoutesExpired = errors.New("invoice expired")
)

func errFormat(num int, errStr string, err error) error {
	return fmt.Errorf("%d@%s@%w", num, errStr, err)
}
