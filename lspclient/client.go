package lspclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"

	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

func (lc *LspClient) connecToLsp(grpcAddr string, token string, cert string) error {
	log.Tracef("connecToLsp: %s", grpcAddr)
	var option grpc.DialOption
	if len(cert) > 0 {
		log.Trace("connecToLsp: TLS")
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM([]byte(cert)) {
			log.Error("credentials: failed to append certificates")
			return errFormat(errNewLspClientConnect, "connecToLsp:Pem", nil)
		}
		creds := credentials.NewTLS(&tls.Config{
			ServerName: "localhost",
			RootCAs:    cp,
		})
		option = grpc.WithTransportCredentials(creds)
	} else {
		log.Warn("connecToLsp: NoTLS")
		option = grpc.WithInsecure()
	}
	var err error
	lc.Conn, err = grpc.Dial(grpcAddr, option)
	if err != nil {
		return errFormat(errNewLspClientConnect, "connecToLsp:Dial", err)
	}
	lc.Ctx, lc.Cancel = context.WithCancel(
		metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token),
	)
	lc.Client = lspdrpc.NewLightningServiceClient(lc.Conn)
	return nil
}

func (lc *LspClient) disconnect() {
	lc.Conn.Close()
	lc.LndApi.Disconnect()
	lc.Cancel()
	log.Trace("Disonnect done")
}
