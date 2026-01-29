package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thanhnp/chain-apis/internal/api"
	"github.com/thanhnp/chain-apis/internal/config"
	"github.com/thanhnp/chain-apis/internal/notifier"
	"github.com/thanhnp/chain-apis/internal/rpc"
	"github.com/thanhnp/chain-apis/internal/storage"
	"github.com/thanhnp/chain-apis/internal/sync"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Println("Starting Chain APIs server...")

	// Initialize multi-chain stores
	multiBlockStore := storage.NewMultiChainBlockStore()
	multiTxStore := storage.NewMultiChainTxStore()
	multiVinStore := storage.NewMultiChainVinStore()
	multiVoutStore := storage.NewMultiChainVoutStore()
	multiAddressStore := storage.NewMultiChainAddressStore()
	multiSyncStore := storage.NewMultiChainSyncStore()

	// Track chain stores for cleanup
	var chainStores []*storage.ChainStores

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize syncers for enabled chains
	var syncers []*sync.Syncer

	// Bitcoin syncer
	if cfg.Bitcoin.Enabled {
		log.Println("Initializing Bitcoin notifier...")

		// Create separate database for Bitcoin
		btcDBPath := cfg.Pebble.Path + "/btc"
		log.Printf("Opening Bitcoin Pebble database at %s", btcDBPath)
		btcDB, err := storage.NewPebbleDB(btcDBPath)
		if err != nil {
			log.Fatalf("Failed to open Bitcoin Pebble database: %v", err)
		}
		btcStores := storage.NewChainStores(btcDB)
		chainStores = append(chainStores, btcStores)

		// Register Bitcoin stores in multi-chain stores
		multiBlockStore.RegisterChain("btc", btcStores.BlockStore)
		multiTxStore.RegisterChain("btc", btcStores.TxStore)
		multiVinStore.RegisterChain("btc", btcStores.VinStore)
		multiVoutStore.RegisterChain("btc", btcStores.VoutStore)
		multiAddressStore.RegisterChain("btc", btcStores.AddressStore)
		multiSyncStore.RegisterChain("btc", btcStores.SyncStore)

		// Create the notifier first
		btcNotifier := notifier.NewBTCNotifier()

		// Connect to btcd using the notifier's handlers
		btcdClient, btcNodeVer, btcConnectErr := rpc.ConnectNodeRPC(
			cfg.Bitcoin.Host,
			cfg.Bitcoin.User,
			cfg.Bitcoin.Pass,
			cfg.Bitcoin.Cert,
			cfg.Bitcoin.DisableTLS,
			false, // disableReconnect
			btcNotifier.BtcdHandlers(),
		)

		if btcConnectErr != nil {
			log.Printf("Warning: Failed to connect to Bitcoin node: %v", btcConnectErr)
		} else {
			log.Printf("Connected to btcd, API version: %s", btcNodeVer)

			// Set the client on the notifier
			btcNotifier.SetClient(btcdClient)

			btcSyncer := sync.NewSyncer(
				btcNotifier,
				btcStores.BlockStore,
				btcStores.TxStore,
				btcStores.VinStore,
				btcStores.VoutStore,
				btcStores.AddressStore,
				btcStores.SyncStore,
				cfg.Bitcoin.StartHeight,
			)
			// Save the client to the syncer for normal RPC calls
			btcSyncer.SetClient(btcdClient)
			syncers = append(syncers, btcSyncer)

			if err := btcSyncer.Start(ctx); err != nil {
				log.Printf("Warning: Failed to start Bitcoin syncer: %v", err)
			} else {
				log.Println("Bitcoin syncer started")
			}
		}
	}

	// Litecoin syncer
	if cfg.Litecoin.Enabled {
		log.Println("Initializing Litecoin notifier...")

		// Create separate database for Litecoin
		ltcDBPath := cfg.Pebble.Path + "/ltc"
		log.Printf("Opening Litecoin Pebble database at %s", ltcDBPath)
		ltcDB, err := storage.NewPebbleDB(ltcDBPath)
		if err != nil {
			log.Fatalf("Failed to open Litecoin Pebble database: %v", err)
		}
		ltcStores := storage.NewChainStores(ltcDB)
		chainStores = append(chainStores, ltcStores)

		// Register Litecoin stores in multi-chain stores
		multiBlockStore.RegisterChain("ltc", ltcStores.BlockStore)
		multiTxStore.RegisterChain("ltc", ltcStores.TxStore)
		multiVinStore.RegisterChain("ltc", ltcStores.VinStore)
		multiVoutStore.RegisterChain("ltc", ltcStores.VoutStore)
		multiAddressStore.RegisterChain("ltc", ltcStores.AddressStore)
		multiSyncStore.RegisterChain("ltc", ltcStores.SyncStore)

		// Create the notifier first
		ltcNotifier := notifier.NewLTCNotifier()

		// Connect to ltcd using the notifier's handlers
		ltcdClient, ltcNodeVer, ltcConnectErr := rpc.ConnectLTCNodeRPC(
			cfg.Litecoin.Host,
			cfg.Litecoin.User,
			cfg.Litecoin.Pass,
			cfg.Litecoin.Cert,
			cfg.Litecoin.DisableTLS,
			false, // disableReconnect
			ltcNotifier.LtcdHandlers(),
		)

		if ltcConnectErr != nil {
			log.Printf("Warning: Failed to connect to Litecoin node: %v", ltcConnectErr)
		} else {
			log.Printf("Connected to ltcd, API version: %s", ltcNodeVer)

			// Set the client on the notifier
			ltcNotifier.SetClient(ltcdClient)

			ltcSyncer := sync.NewSyncer(
				ltcNotifier,
				ltcStores.BlockStore,
				ltcStores.TxStore,
				ltcStores.VinStore,
				ltcStores.VoutStore,
				ltcStores.AddressStore,
				ltcStores.SyncStore,
				cfg.Litecoin.StartHeight,
			)
			// Save the client to the syncer for normal RPC calls
			ltcSyncer.SetLTCClient(ltcdClient)
			syncers = append(syncers, ltcSyncer)

			if err := ltcSyncer.Start(ctx); err != nil {
				log.Printf("Warning: Failed to start Litecoin syncer: %v", err)
			} else {
				log.Println("Litecoin syncer started")
			}
		}
	}

	// Initialize API router with multi-chain stores
	router := api.NewRouter(
		multiBlockStore,
		multiTxStore,
		multiVinStore,
		multiVoutStore,
		multiAddressStore,
		multiSyncStore,
	)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router.Engine(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Printf("HTTP server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	// Cancel context to stop syncers
	cancel()

	// Stop all syncers
	for _, s := range syncers {
		if err := s.Stop(); err != nil {
			log.Printf("Error stopping syncer: %v", err)
		}
	}

	// Close all chain databases
	for _, cs := range chainStores {
		if err := cs.Close(); err != nil {
			log.Printf("Error closing chain database: %v", err)
		}
	}

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
