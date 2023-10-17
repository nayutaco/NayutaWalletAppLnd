//go:build mobile
// +build mobile

package lndmobile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/btcsuite/btclog"

	"github.com/lightningnetwork/lnd"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/closechecker"
	"github.com/lightningnetwork/lnd/lspclient"
	"github.com/lightningnetwork/lnd/signal"
)

const (
	maxLogFileSize = 5
	maxLogFiles    = 3
)

var (
	// lndStarted will be used atomically to ensure only a single lnd instance is
	// attempted to be started at once.
	lndStarted int32

	logWriter *build.RotatingLogWriter

	shutdownInterceptor signal.Interceptor

	closeChecker *closechecker.CloseChecker
)

func cleanupGlobalVar() {
	atomic.StoreInt32(&lndStarted, 0)
	shutdownInterceptor = signal.Interceptor{}
	if closeChecker != nil {
		closeChecker.Close()
		closeChecker = nil
	}
	logWriter = nil
}

func IsRunning() bool {
	return lndStarted == 1
}

// genSubLogger creates a logger for a subsystem. We provide an instance of
// a signal.Interceptor to be able to shutdown in the case of a critical error.
func genSubLogger(root *build.RotatingLogWriter,
	interceptor signal.Interceptor) func(string) btclog.Logger {

	// Create a shutdown function which will request shutdown from our
	// interceptor if it is listening.
	shutdown := func() {
		if !interceptor.Listening() {
			return
		}

		interceptor.RequestShutdown()
	}

	// Return a function which will create a sublogger from our root
	// logger without shutdown fn.
	return func(tag string) btclog.Logger {
		return root.GenSubLogger(tag, shutdown)
	}
}

// addSubLogger is a helper method to conveniently create and register the
// logger of one or more sub systems.
func addSubLogger(root *build.RotatingLogWriter, subsystem string,
	interceptor signal.Interceptor, useLoggers ...func(btclog.Logger)) {

	// genSubLogger will return a callback for creating a logger instance,
	// which we will give to the root logger.
	genLogger := genSubLogger(root, interceptor)

	// Create and register just a single logger to prevent them from
	// overwriting each other internally.
	logger := build.NewSubLogger(subsystem, genLogger)
	root.RegisterSubLogger(subsystem, logger)
	for _, useLogger := range useLoggers {
		useLogger(logger)
	}
}

func Init(filePath string, logPath string) error {
	var err error

	fmt.Printf("mobile.Init(%s)\n", filePath)
	if len(filePath) == 0 || len(logPath) == 0 {
		return fmt.Errorf("mobile.Init: bad argument")
	}

	if logWriter == nil {
		// Hook interceptor for os signals.
		shutdownInterceptor, err = signal.Intercept()
		if err != nil {
			fmt.Printf("mobile.Init: Intercept :%v\n", err)
			return err
		}

		logWriter = build.NewRotatingLogWriter()
		err = logWriter.InitLogRotator(
			filepath.Join(logPath, "lndmobile.log"),
			maxLogFileSize, maxLogFiles,
		)
		if err != nil {
			return fmt.Errorf("mobile.Init: lndmobile.log rotation setup failed: %w", err)
		}
		addSubLogger(logWriter, "LMBL", shutdownInterceptor, UseLogger)
		addSubLogger(logWriter, "LSPC", shutdownInterceptor, lspclient.UseLogger)
		addSubLogger(logWriter, "CCHK", shutdownInterceptor, closechecker.UseLogger)
		log.Infof("mobile.Init: log init")
	}

	if closeChecker == nil {
		closeChecker, err = closechecker.Open(filePath)
		if err != nil {
			return fmt.Errorf("mobile.Init: closeChecker.Open: %w", err)
		}
		log.Infof("mobile.Init: closechecker init")
	}
	log.Infof("mobile.Init: done")
	return nil
}

// Start starts lnd in a new goroutine.
//
// extraArgs can be used to pass command line arguments to lnd that will
// override what is found in the config file. Example:
//
//	extraArgs = "--bitcoin.testnet --lnddir=\"/tmp/folder name/\" --profile=5050"
//
// The rpcReady is called lnd is ready to accept RPC calls.
//
// NOTE: On mobile platforms the '--lnddir` argument should be set to the
// current app directory in order to ensure lnd has the permissions needed to
// write to it.
func Start(extraArgs string, rpcReady Callback) {
	log.Infof("mobile.Start")

	// We only support a single lnd instance at a time (singleton) for now,
	// so we make sure to return immediately if it has already been
	// started.
	if !atomic.CompareAndSwapInt32(&lndStarted, 0, 1) {
		log.Errorf("mobile.Start: lnd already started")
		err := errors.New("lnd already started")
		rpcReady.OnError(err)
		return
	}

	// (Re-)initialize the in-mem gRPC listeners we're going to give to lnd.
	// This is required each time lnd is started, because when lnd shuts
	// down, the in-mem listeners are closed.
	RecreateListeners()

	// Split the argument string on "--" to get separated command line
	// arguments.
	var splitArgs []string
	for _, a := range strings.Split(extraArgs, "--") {
		// Trim any whitespace space, and ignore empty params.
		a := strings.TrimSpace(a)
		if a == "" {
			continue
		}

		// Finally we prefix any non-empty string with -- to mimic the
		// regular command line arguments.
		splitArgs = append(splitArgs, "--"+a)
	}

	// Add the extra arguments to os.Args, as that will be parsed in
	// LoadConfig below.
	os.Args = append(os.Args, splitArgs...)

	// Load the configuration, and parse the extra arguments as command
	// line options. This function will also set up logging properly.
	loadedConfig, err := lnd.LoadConfig(shutdownInterceptor)
	if err != nil {
		cleanupGlobalVar()
		log.Errorf("mobile.Start: LoadConfig :%v", err)
		rpcReady.OnError(err)
		return
	}

	// Set a channel that will be notified when the RPC server is ready to
	// accept calls.
	var (
		rpcListening = make(chan struct{})
		quit         = make(chan struct{})
	)

	// Nayuta Core needs default gRPC server, not in-memory socket, and
	// channels to be notified that State RPC is ready.
	cfg := lnd.ListenerCfg{
		RPCListeners: []*lnd.ListenerWithSignal{{
			Listener: nil,
			Ready:    rpcListening,
		}},
	}
	implCfg := loadedConfig.ImplementationConfig(shutdownInterceptor)

	// Call the "real" main in a nested manner so the defers will properly
	// be executed in the case of a graceful shutdown.
	go func() {
		defer cleanupGlobalVar()
		defer close(quit)

		log.Infof("mobile.Start: lnd.Main")
		if err := lnd.Main(
			loadedConfig, cfg, implCfg, shutdownInterceptor,
		); err != nil {
			log.Errorf("mobile.Start: lnd.Main: %v", err)
			rpcReady.OnError(err)
			return
		}
		log.Infof("mobile.Start: gracefully stopped")
		rpcReady.OnError(nil) // gracefully stopped
	}()

	// // By default we'll apply the admin auth options, which will include
	// // macaroons.
	// setDefaultDialOption(
	// 	func() ([]grpc.DialOption, error) {
	// 		return lnd.AdminAuthOptions(loadedConfig, false)
	// 	},
	// )

	// // For the WalletUnlocker and StateService, the macaroons might not be
	// // available yet when called, so we use a more restricted set of
	// // options that don't include them.
	// setWalletUnlockerDialOption(
	// 	func() ([]grpc.DialOption, error) {
	// 		return lnd.AdminAuthOptions(loadedConfig, true)
	// 	},
	// )
	// setStateDialOption(
	// 	func() ([]grpc.DialOption, error) {
	// 		return lnd.AdminAuthOptions(loadedConfig, true)
	// 	},
	// )

	// Finally we start a go routine that will call the provided callback
	// when the RPC server is ready to accept calls.
	go func() {
		select {
		case <-rpcListening:
		case <-quit:
			return
		}

		log.Infof("mobile.Start: ready")
		rpcReady.OnResponse([]byte{})
	}()
}

func Shutdown() {
	log.Infof("mobile.Shutdown")
	shutdownInterceptor.RequestShutdown()
}

/*
 * Close Checker without LND
 */
func CcStart(testNet bool) error {
	if closeChecker == nil {
		return fmt.Errorf("CcStart: not started")
	}
	err := closeChecker.Connect(testNet)
	if err != nil {
		return fmt.Errorf("CcStart: Connect: %w", err)
	}
	return nil
}

func CcEnd() error {
	if closeChecker == nil {
		return fmt.Errorf("CcEnd: not started")
	}
	closeChecker.Disconnect()
	return nil
}

func CcAddChannelList(channelPoint string) error {
	if closeChecker == nil {
		return fmt.Errorf("CcAddChannelList: not started")
	}
	err := closeChecker.AddChannelPoint(channelPoint)
	log.Infof("CcAddChannelList: %v", err)
	return err
}

func CcRemoveChannelList(channelPoint string) error {
	if closeChecker == nil {
		return fmt.Errorf("CcRemoveChannelList: not started")
	}
	err := closeChecker.RemoveChannelPoint(channelPoint)
	log.Infof("CcRemoveChannelList: %v", err)
	return err
}

func CcRemoveChannelListAll() error {
	if closeChecker == nil {
		return fmt.Errorf("CcRemoveChannelListAll: not started")
	}
	err := closeChecker.RemoveChannelPointAll()
	log.Infof("CcRemoveChannelListAll: %v", err)
	return err
}

// CheckClosedChannels checks channelpoint is UTXO or not.
func CcCheckClosedChannels() (bool, error) {
	if closeChecker == nil {
		return false, fmt.Errorf("CcCheckClosedChannels: not started")
	}

	closed, err := closeChecker.CheckClosedChannels()
	if err != nil {
		return false, fmt.Errorf("CcCheckClosedChannels: CheckClosedChannels: %v", err)
	}
	log.Infof("CcCheckClosedChannels: closed: %d", closed)
	return closed > 0, nil
}
