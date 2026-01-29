package rpc

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"

	"github.com/thanhnp/chain-apis/internal/config"
	"github.com/thanhnp/chain-apis/internal/models"
	"github.com/thanhnp/chain-apis/pkg/semver"
)

// Compatible btcd JSON-RPC API versions
var compatibleChainServerAPIs = []semver.Semver{
	semver.NewSemver(1, 0, 0),
	semver.NewSemver(2, 0, 0),
	semver.NewSemver(3, 0, 0),
	semver.NewSemver(4, 0, 0),
	semver.NewSemver(5, 0, 0),
	semver.NewSemver(6, 0, 0),
	semver.NewSemver(7, 0, 0),
	semver.NewSemver(8, 0, 0),
}

// ConnectNodeRPC connects to a btcd node via WebSocket RPC
func ConnectNodeRPC(host, user, pass, cert string, disableTLS, disableReconnect bool,
	ntfnHandlers ...*rpcclient.NotificationHandlers) (*rpcclient.Client, semver.Semver, error) {
	var btcdCerts []byte
	var err error
	var nodeVer semver.Semver
	if !disableTLS {
		btcdCerts, err = os.ReadFile(cert)
		if err != nil {
			log.Printf("Failed to read btcd cert file at %s: %s\n",
				cert, err.Error())
			return nil, nodeVer, err
		}
		log.Printf("Attempting to connect to btcd RPC %s as user %s "+
			"using certificate located in %s",
			host, user, cert)
	} else {
		log.Printf("Attempting to connect to btcd RPC %s as user %s (no TLS)",
			host, user)
	}

	// connect with btcd
	connCfgDaemon := &rpcclient.ConnConfig{
		Host:                 host,
		Endpoint:             "ws", // websocket
		User:                 user,
		Pass:                 pass,
		Certificates:         btcdCerts,
		DisableTLS:           disableTLS,
		DisableAutoReconnect: disableReconnect,
	}
	var ntfnHdlrs *rpcclient.NotificationHandlers
	if len(ntfnHandlers) > 0 {
		if len(ntfnHandlers) > 1 {
			return nil, nodeVer, fmt.Errorf("invalid notification handler argument")
		}
		ntfnHdlrs = ntfnHandlers[0]
	}
	btcdClient, err := rpcclient.New(connCfgDaemon, ntfnHdlrs)
	if err != nil {
		return nil, nodeVer, fmt.Errorf("Failed to start btcd RPC client: %s", err.Error())
	}

	// Ensure the RPC server has a compatible API version.
	ver, err := btcdClient.Version()
	if err != nil {
		log.Println("Unable to get RPC version: ", err)
		return nil, nodeVer, fmt.Errorf("unable to get node RPC version")
	}

	btcdVer := ver["btcdjsonrpcapi"]
	nodeVer = semver.NewSemver(btcdVer.Major, btcdVer.Minor, btcdVer.Patch)

	// Check if the btcd RPC API version is compatible.
	isAPICompat := semver.AnyCompatible(compatibleChainServerAPIs, nodeVer)
	if !isAPICompat {
		return nil, nodeVer, fmt.Errorf("Node JSON-RPC server does not have "+
			"a compatible API version. Advertises %v but requires one of: %v",
			nodeVer, compatibleChainServerAPIs)
	}

	return btcdClient, nodeVer, nil
}

// BTCClient wraps the Bitcoin RPC client
type BTCClient struct {
	client *rpcclient.Client
	config *config.ChainConfig
}

// NewBTCClient creates a new Bitcoin RPC client
func NewBTCClient(cfg *config.ChainConfig, ntfnHandlers *rpcclient.NotificationHandlers) (*BTCClient, error) {
	var certs []byte
	var err error

	if !cfg.DisableTLS && cfg.Cert != "" {
		certs, err = os.ReadFile(cfg.Cert)
		if err != nil {
			return nil, fmt.Errorf("failed to read certificate: %w", err)
		}
	}

	var connCfg *rpcclient.ConnConfig
	var handlers *rpcclient.NotificationHandlers

	if cfg.HTTPMode {
		// HTTP POST mode for bitcoind
		connCfg = &rpcclient.ConnConfig{
			Host:         cfg.Host,
			User:         cfg.User,
			Pass:         cfg.Pass,
			HTTPPostMode: true,
			DisableTLS:   cfg.DisableTLS,
			Certificates: certs,
		}
	} else {
		// WebSocket mode for btcd
		connCfg = &rpcclient.ConnConfig{
			Host:                 cfg.Host,
			Endpoint:             "ws",
			User:                 cfg.User,
			Pass:                 cfg.Pass,
			Certificates:         certs,
			DisableTLS:           cfg.DisableTLS,
			DisableAutoReconnect: false,
		}
		handlers = ntfnHandlers
	}

	client, err := rpcclient.New(connCfg, handlers)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	return &BTCClient{
		client: client,
		config: cfg,
	}, nil
}

// Close closes the RPC client connection
func (c *BTCClient) Close() {
	c.client.Shutdown()
}

// GetBlockCount returns the current block height
func (c *BTCClient) GetBlockCount() (int64, error) {
	return c.client.GetBlockCount()
}

// GetBlockHash returns the block hash for a given height
func (c *BTCClient) GetBlockHash(height int64) (*chainhash.Hash, error) {
	return c.client.GetBlockHash(height)
}

// GetBlock returns the block for a given hash
func (c *BTCClient) GetBlock(hash *chainhash.Hash) (*wire.MsgBlock, error) {
	return c.client.GetBlock(hash)
}

// GetBlockVerbose returns verbose block info for a given hash
func (c *BTCClient) GetBlockVerbose(hash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error) {
	return c.client.GetBlockVerbose(hash)
}

// GetBlockVerboseTx returns verbose block info with transactions
func (c *BTCClient) GetBlockVerboseTx(hash *chainhash.Hash) (*btcjson.GetBlockVerboseTxResult, error) {
	return c.client.GetBlockVerboseTx(hash)
}

// GetRawTransaction returns the raw transaction for a given hash
func (c *BTCClient) GetRawTransaction(hash *chainhash.Hash) (*btcjson.TxRawResult, error) {
	return c.client.GetRawTransactionVerbose(hash)
}

// NotifyBlocks registers for block notifications
func (c *BTCClient) NotifyBlocks() error {
	return c.client.NotifyBlocks()
}

// ParseBlock converts btcd block data to our Block model
func (c *BTCClient) ParseBlock(blockVerbose *btcjson.GetBlockVerboseResult) *models.Block {
	return &models.Block{
		Hash:         blockVerbose.Hash,
		Height:       blockVerbose.Height,
		Version:      blockVerbose.Version,
		PreviousHash: blockVerbose.PreviousHash,
		MerkleRoot:   blockVerbose.MerkleRoot,
		Timestamp:    time.Unix(blockVerbose.Time, 0),
		Bits:         blockVerbose.Bits,
		Nonce:        blockVerbose.Nonce,
		TxCount:      len(blockVerbose.Tx),
		Size:         int(blockVerbose.Size),
		Weight:       int(blockVerbose.Weight),
		Chain:        "btc",
	}
}

// ParseBlockWithTx converts btcd block data with transactions to our models
func (c *BTCClient) ParseBlockWithTx(blockVerbose *btcjson.GetBlockVerboseTxResult) (*models.Block, []*models.Transaction, []*models.Vin, []*models.Vout) {
	block := &models.Block{
		Hash:         blockVerbose.Hash,
		Height:       blockVerbose.Height,
		Version:      blockVerbose.Version,
		PreviousHash: blockVerbose.PreviousHash,
		MerkleRoot:   blockVerbose.MerkleRoot,
		Timestamp:    time.Unix(blockVerbose.Time, 0),
		Bits:         blockVerbose.Bits,
		Nonce:        blockVerbose.Nonce,
		TxCount:      len(blockVerbose.Tx),
		Size:         int(blockVerbose.Size),
		Weight:       int(blockVerbose.Weight),
		Chain:        "btc",
	}

	var txs []*models.Transaction
	var vins []*models.Vin
	var vouts []*models.Vout

	for _, rawTx := range blockVerbose.Tx {
		// Check if this is a coinbase transaction
		// Coinbase tx has a single vin with empty Txid
		isCoinbase := len(rawTx.Vin) > 0 && rawTx.Vin[0].Txid == ""

		tx := &models.Transaction{
			TxID:        rawTx.Txid,
			BlockHash:   blockVerbose.Hash,
			BlockHeight: blockVerbose.Height,
			Version:     int32(rawTx.Version),
			LockTime:    rawTx.LockTime,
			Size:        int(rawTx.Size),
			VSize:       int(rawTx.Vsize),
			Weight:      int(rawTx.Weight),
			IsCoinbase:  isCoinbase,
			Timestamp:   time.Unix(blockVerbose.Time, 0),
			Chain:       "btc",
		}
		txs = append(txs, tx)

		// Parse vins
		for i, rawVin := range rawTx.Vin {
			vin := &models.Vin{
				TxID:        rawTx.Txid,
				VinIndex:    i,
				PrevTxID:    rawVin.Txid,
				PrevVoutIdx: int(rawVin.Vout),
				Sequence:    rawVin.Sequence,
				Chain:       "btc",
			}
			if rawVin.ScriptSig != nil {
				vin.ScriptSig = rawVin.ScriptSig.Hex
			}
			if len(rawVin.Witness) > 0 {
				vin.Witness = rawVin.Witness
			}
			vins = append(vins, vin)
		}

		// Parse vouts
		for _, rawVout := range rawTx.Vout {
			vout := &models.Vout{
				TxID:         rawTx.Txid,
				VoutIndex:    int(rawVout.N), // Use actual vout index from RPC, not loop index
				Value:        int64(rawVout.Value * 1e8), // Convert BTC to satoshis
				ScriptPubKey: rawVout.ScriptPubKey.Hex,
				Type:         rawVout.ScriptPubKey.Type,
				Addresses:    rawVout.ScriptPubKey.Addresses,
				Spent:        false,
				Chain:        "btc",
			}
			vouts = append(vouts, vout)
		}
	}

	return block, txs, vins, vouts
}

// GetClient returns the underlying RPC client for advanced operations
func (c *BTCClient) GetClient() *rpcclient.Client {
	return c.client
}
