package closechecker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/checksum0/go-electrum/electrum"
	"go.etcd.io/bbolt"
)

const (
	maxRetry       = 30
	connDuration   = 1 * time.Second
	methodDuration = 5 * time.Second

	dbFilename = "closecheck.db"
)

var (
	// database
	chanBucket = []byte("check-channel")

	// default coin port
	// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#server-peers-subscribe
	defaultPort = "8333"
)

// CloseChecker is close checker info.
type CloseChecker struct {
	lock    sync.RWMutex
	backend *bbolt.DB

	client *electrum.Client
}

// Open connect channel info DB.
func Open(dbPath string) (*CloseChecker, error) {
	path := fmt.Sprintf("%s/%s", dbPath, dbFilename)
	db, err := bbolt.Open(path, 0644, &bbolt.Options{ReadOnly: false})
	if err != nil {
		log.Errorf("Open: %w", err)
		return nil, err
	}
	return &CloseChecker{backend: db}, nil
}

func (cc *CloseChecker) Close() error {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	if cc.client != nil {
		cc.client.Shutdown()
		cc.client = nil
	}
	return cc.backend.Close()
}

// Connect connect to an Electrum server.
func (cc *CloseChecker) Connect(testNet bool) error {
	log.Debugf("Connect")

	cc.lock.Lock()
	defer cc.lock.Unlock()

	if cc.client != nil {
		cc.client.Shutdown()
	}
	if testNet {
		defaultPort = "18333"
	}

	err := cc.getElectrumClient(testNet)
	if err != nil {
		return err
	}
	return nil
}

// Disconnect disconnect Electrum server connection.
func (cc *CloseChecker) Disconnect() {
	log.Debugf("Disconnect")

	cc.lock.Lock()
	defer cc.lock.Unlock()

	if cc.client != nil {
		cc.client.Shutdown()
		cc.client = nil
	}
}

func (cc *CloseChecker) getElectrumClient(testNet bool) error {
	var err error
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	var servers []electrumServer
	var newServers []electrumServer
	var testTxid string // for GetTransaction support check
	if !testNet {
		servers = electrumServers
		testTxid = "7c02726b515fded8c416389235cf9a1a3ec1cf7436c95404abbc7a9d0a217ec9"
	} else {
		servers = electrumTestnetServers
		testTxid = "267008b36b9c05d8050f7bb2eed4d27f15b15617c273ae0778e3b66272bee4c5"
	}

	// first connect for get servers
	var client *electrum.Client
	for count := 0; count < maxRetry; count++ {
		serverIdx := rnd.Int31n(int32(len(servers)))
		server := servers[serverIdx]
		client, err = connectElectrumServer(server)
		if err != nil {
			// log.Warnf("getElectrumClient: fail connectElectrumServer: %w", err)
			continue
		}

		newServers, err = getServerPeers(client)
		client.Shutdown()
		client = nil
		if err != nil {
			log.Errorf("getElectrumClient: fail getServerPeers: %w", err)
			continue
		}
		break
	}
	if len(newServers) == 0 {
		return fmt.Errorf("getElectrumClient: no servers")
	}

	for count := 0; count < maxRetry; count++ {
		idx := rnd.Int31n(int32(len(servers) + len(newServers)))
		var node electrumServer
		if int(idx) < len(servers) {
			node = servers[idx]
		} else {
			node = newServers[int(idx)-len(servers)]
		}
		if len(node.Server) == 0 {
			continue
		}

		client, err = connectElectrumServer(node)
		if err != nil {
			// log.Warnf("  fail connectElectrumServer: %w", err)
			continue
		}
		_, err := getTransaction(client, testTxid)
		if err != nil {
			// GetTransaction(verbose) not supported
			client.Shutdown()
			client = nil
			continue
		}
		log.Debugf("getElectrumClient: connected: %s", node.Server)
		break
	}
	if client != nil {
		cc.client = client
	} else {
		log.Errorf("getElectrumClient: fail second connect")
	}
	return nil
}

// func (cc *CloseChecker) RegisteredChannelCount() (uint16, error) {
// 	if cc.backend == nil {
// 		return 0, fmt.Errorf("cc.backend: not initialized")
// 	}

// 	cc.lock.Lock()
// 	defer cc.lock.Unlock()

// 	var count uint16

// 	err := cc.backend.View(func(tx *bbolt.Tx) error {
// 		bucket := tx.Bucket(chanBucket)
// 		if bucket == nil {
// 			log.Errorf("RegisteredChannelCount: bbolt.Bucket: nil")
// 			return nil
// 		}
// 		cur := bucket.Cursor()
// 		for k, _ := cur.First(); k != nil; k, _ = cur.Next() {
// 			count++
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		log.Errorf("RegisteredChannelCount: bbolt.View: %w", err)
// 		return 0, err
// 	}
// 	return count, nil
// }

// AddChannelPoint add channel info to DB.
func (cc *CloseChecker) AddChannelPoint(channelPoint string) error {
	if cc.backend == nil {
		return fmt.Errorf("cc.backend: not initialized")
	}
	chanPnt := strings.Split(channelPoint, ":")
	if len(chanPnt) != 2 {
		return fmt.Errorf("AddChannelPoint: bad argument(%s)", channelPoint)
	}
	_, err := strconv.Atoi(chanPnt[1])
	if err != nil {
		return fmt.Errorf("strconv.Atoi: %w", err)
	}

	cc.lock.Lock()
	defer cc.lock.Unlock()

	var gotValue []byte
	cc.backend.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(chanBucket)
		if bucket == nil {
			return nil
		}
		gotValue = bucket.Get([]byte(channelPoint))
		return nil
	})
	if gotValue != nil {
		log.Warnf("AddChannelPoint: already registered: %s", channelPoint)
		return nil
	}

	err = cc.backend.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(chanBucket)
		if err != nil {
			log.Errorf("AddChannelPoint: bbolt.CreateBucketIfNotExists: %w", err)
			return err
		}

		return bucket.Put([]byte(channelPoint), []byte(""))
	})
	if err != nil {
		log.Errorf("AddChannelPoint: bbolt.Update: %w", err)
		return err
	}

	return nil
}

// RemoveChannelPoint remove a channel info from DB.
func (cc *CloseChecker) RemoveChannelPoint(channelPoint string) error {
	if cc.backend == nil {
		return fmt.Errorf("cc.backend: not initialized")
	}

	cc.lock.Lock()
	defer cc.lock.Unlock()

	err := cc.backend.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(chanBucket)
		if bucket == nil {
			log.Errorf("RemoveChannelPoint: bbolt.Bucket: nil")
			return nil
		}

		return bucket.Delete([]byte(channelPoint))
	})
	if err != nil {
		log.Errorf("RemoveChannelPoint: bbolt.Update: %w", err)
		return err
	}

	return nil
}

// RemoveChannelPointAll remove all channel info from DB.
func (cc *CloseChecker) RemoveChannelPointAll() error {
	if cc.backend == nil {
		return fmt.Errorf("cc.backend: not initialized")
	}

	cc.lock.Lock()
	defer cc.lock.Unlock()

	err := cc.backend.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(chanBucket)
		if bucket == nil {
			log.Errorf("RemoveChannelPointAll: bbolt.Bucket: nil")
			return nil
		}
		bucket.ForEach(func(k, v []byte) error {
			log.Debugf("RemoveChannelPointAll: k=%s, v=%s", string(k), string(v))
			err := bucket.Delete(k)
			if err != nil {
				log.Errorf("RemoveChannelPointAll: bbolt.Delete: %w", err)
			}
			return nil
		})

		return nil
	})
	if err != nil {
		log.Errorf("RemoveChannelPointAll: bbolt.Update: %w", err)
		return err
	}

	return nil
}

func (cc *CloseChecker) checkClosedChannels(client *electrum.Client) (uint16, error) {
	var closed uint16

	err := cc.backend.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(chanBucket)
		if bucket == nil {
			log.Errorf("checkClosedChannels: bbolt.Bucket: nil")
			return nil
		}
		cur := bucket.Cursor()
		for k, v := cur.First(); k != nil; k, v = cur.Next() {
			channelPoint := string(k)
			scriptHash := string(v)
			log.Debugf("checkClosedChannels: k=%s, v=%s", channelPoint, scriptHash)
			if len(string(v)) == 0 {
				var err error
				scriptHash, err = getElectrumAddress(client, channelPoint)
				if err != nil {
					log.Errorf("getElectrumAddress: %w", err)
					return err
				}
				bucket.Put([]byte(channelPoint), []byte(scriptHash))
			}

			// アドレスへの送金履歴取得
			//    --> confirmed TXID取得
			//    --> Vin[0] == channelPoint --> CLOSED!
			history, err := getHistory(client, scriptHash)
			if err != nil {
				continue
			}
			var hasClosed = false
			for _, hist := range history {
				if hist.Height != 0 {
					// confirmed
					log.Debugf("checkClosedChannels: checking...: %s", hist.Hash)
					txResult, err := getTransaction(client, hist.Hash)
					if err != nil {
						log.Errorf("checkClosedChannels: getTransaction: %v", err)
						continue
					}
					log.Debugf("txResult: %v", txResult)
					chanPntTxid := strings.Split(channelPoint, ":")[0]
					if txResult.Confirmations > 1 && len(txResult.Vin) == 1 && txResult.Vin[0].TxID == chanPntTxid {
						log.Debugf("checkClosedChannels: closed!: %s(conf=%d)", channelPoint, txResult.Confirmations)
						hasClosed = true
						break
					}
				}
			}

			if hasClosed {
				log.Debugf("checkClosedChannels: closed: %s", channelPoint)
				closed++
			}
		}

		return nil
	})
	if err != nil {
		log.Errorf("checkClosedChannels: bbolt.View: %w", err)
		return 0, err
	}

	return closed, nil
}

// CheckClosedChannels checks closed channels in DB.
func (cc *CloseChecker) CheckClosedChannels() (uint16, error) {
	if cc.backend == nil {
		return 0, fmt.Errorf("cc.backend: not initialized")
	}
	if cc.client == nil {
		return 0, fmt.Errorf("client1: not initialized")
	}
	cc.lock.Lock()
	defer cc.lock.Unlock()

	closed, err := cc.checkClosedChannels(cc.client)
	if err != nil {
		return 0, err
	}
	return closed, nil
}

func connectElectrumServer(server electrumServer) (*electrum.Client, error) {
	var client *electrum.Client
	var err error

	if len(server.TLS) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), connDuration)
		defer cancel()

		url := fmt.Sprintf("%s:%s", server.Server, server.TLS)
		client, err = electrum.NewClientSSL(ctx, url, nil)
		if err == nil {
			return client, nil
		}
	}
	if len(server.TCP) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), connDuration)
		defer cancel()

		url := fmt.Sprintf("%s:%s", server.Server, server.TCP)
		client, err = electrum.NewClientTCP(ctx, url)
		if err == nil {
			return client, nil
		}
	}
	return nil, err
}

func getServerPeers(client *electrum.Client) ([]electrumServer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), methodDuration)
	defer cancel()

	list, err := client.ServerPeers(ctx)
	if err != nil {
		return nil, fmt.Errorf("getServerPeers: ServerPeers: %w", err)
	}

	listArray, ok := list.([][]interface{})
	if !ok {
		return nil, fmt.Errorf("getServerPeers: fail type assertion: listArray")
	}

	newServers, err := convertServers(listArray)
	if err != nil {
		return nil, err
	}
	return newServers, nil
}

func getTransaction(client *electrum.Client, txid string) (*electrum.GetTransactionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), methodDuration)
	defer cancel()

	txResult, err := client.GetTransaction(ctx, txid)
	if err != nil {
		return nil, fmt.Errorf("GetTransaction(%s): %w", txid, err)
	}
	return txResult, nil
}

func getHistory(client *electrum.Client, scriptHash string) ([]*electrum.GetMempoolResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), methodDuration)
	defer cancel()

	list, err := client.GetHistory(ctx, scriptHash)
	if err != nil {
		return nil, fmt.Errorf("GetHistory: %w", err)
	}
	return list, err
}

func getElectrumAddress(client *electrum.Client, channelPoint string) (string, error) {
	// already checked on AddChannelPoint()
	chanPnt := strings.Split(channelPoint, ":")
	chanPntIdx, _ := strconv.Atoi(chanPnt[1])
	txResult, err := getTransaction(client, chanPnt[0])
	if err != nil {
		return "", fmt.Errorf("getTransaction: %w", err)
	}
	if len(txResult.Vout) < chanPntIdx {
		return "", fmt.Errorf("AddChannelPoint: less index: %d < %d", len(txResult.Vout), chanPntIdx)
	}
	log.Debugf("txResult.Vout[chanPntIdx].ScriptPubkey=%s", txResult.Vout[chanPntIdx].ScriptPubkey.Hex)
	scriptHash, err := hex.DecodeString(txResult.Vout[chanPntIdx].ScriptPubkey.Hex)
	if err != nil {
		return "", fmt.Errorf("DecodeString: %w", err)
	}
	hashSum := sha256.Sum256(scriptHash)
	for i, j := 0, len(hashSum)-1; i < j; i, j = i+1, j-1 {
		hashSum[i], hashSum[j] = hashSum[j], hashSum[i]
	}
	return hex.EncodeToString((hashSum[:])), nil
}

// convertServers convert servers list from Electrum Server.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#server-peers-subscribe
// list = [
//   ["IP address", "hostname",
//     [
//       "v...max protocol version,
//       "p...prune",
//       "t...TCP port number",
//       "s...SSL port number"
//     ],
//   ],
//   ...
// ]
//
func convertServers(listArray [][]interface{}) ([]electrumServer, error) {
	var newServers []electrumServer
	for idx, l := range listArray {
		features, ok := l[2].([]interface{})
		if !ok {
			continue
		}
		server := electrumServer{
			Server: l[1].(string),
		}
		if strings.HasSuffix(server.Server, ".onion") {
			// onion not support
			continue
		}

		// log.Debugf("[%d]listArray: ipaddr: %s", idx, l[0].(string))
		log.Debugf("[%d]listArray: name: %s", idx, l[1].(string))
		setFeatures := false
		for _, f := range features {
			// log.Debugf("  listArray: features: %s", f.(string))

			feature, ok := f.(string)
			if !ok || len(feature) == 0 {
				continue
			}
			if feature[0] == 's' {
				if len(feature) > 1 {
					server.TLS = feature[1:]
				} else {
					server.TLS = defaultPort
				}
				setFeatures = true
			} else if feature[0] == 't' {
				if len(feature) > 1 {
					server.TCP = feature[1:]
				} else {
					server.TCP = defaultPort
				}
				setFeatures = true
			}
		}
		if setFeatures {
			newServers = append(newServers, server)
		}
	}
	return newServers, nil
}
