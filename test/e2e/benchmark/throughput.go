package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v2/test/e2e/testnet"
	"github.com/celestiaorg/celestia-app/v2/test/util/testnode"
	"github.com/tendermint/tendermint/pkg/trace"
)

const (
	seed = 42

	toMiB = 1024 * 1024
)

var bigBlockManifest = Manifest{
	TestnetName: "big-block",
	ChainID:     "test",
	Validators:  2,
	ValidatorResource: testnet.Resources{
		MemoryRequest: "12Gi",
		MemoryLimit:   "12Gi",
		CPU:           "8",
		Volume:        "20Gi",
	},
	TxClientsResource: testnet.Resources{
		MemoryRequest: "1Gi",
		MemoryLimit:   "3Gi",
		CPU:           "2",
		Volume:        "1Gi",
	},
	SelfDelegation:     10000000,
	CelestiaAppVersion: "pr-3261",
	TxClientVersion:    "pr-3261",
	EnableLatency:      false,
	LatencyParams:      LatencyParams{150, 150}, // in  milliseconds
	BandwidthParams:    0,                       // in kbps
	BlobsPerSeq:        6,
	BlobSequences:      50,
	BlobSizes:          "200000",
	PerPeerBandwidth:   100 * toMiB,
	UpgradeHeight:      0,
	TimeoutCommit:      11 * time.Second,
	TimeoutPropose:     10 * time.Second,
	Mempool:            "v1", // ineffective as it always defaults to v1
	BroadcastTxs:       true,
	Prometheus:         true,
	GovMaxSquareSize:   1024,
	MaxBlockBytes:      128 * toMiB,
	TestDuration:       10 * time.Minute,
	TxClients:          2,
	LocalTracingType:   "local",
	PushTrace:          true,
}

func TwoNodeSimple(_ *log.Logger) error {
	latestVersion, err := testnet.GetLatestVersion()
	testnet.NoError("failed to get latest version", err)

	log.Println("=== RUN TwoNodeSimple", "version:", latestVersion)

	manifest := Manifest{
		TestnetName:        "TwoNodeSimple",
		ChainID:            "two-node-simple",
		Validators:         2,
		ValidatorResource:  testnet.DefaultResources,
		TxClientsResource:  testnet.DefaultResources,
		SelfDelegation:     10000000,
		CelestiaAppVersion: latestVersion,
		TxClientVersion:    testnet.TxsimVersion,
		EnableLatency:      false,
		LatencyParams:      LatencyParams{100, 10}, // in  milliseconds
		BlobsPerSeq:        6,
		BlobSequences:      1,
		BlobSizes:          "200000",
		PerPeerBandwidth:   5 * 1024 * 1024,
		UpgradeHeight:      0,
		TimeoutCommit:      1 * time.Second,
		TimeoutPropose:     1 * time.Second,
		Mempool:            "v1",
		BroadcastTxs:       true,
		Prometheus:         true,
		GovMaxSquareSize:   appconsts.DefaultGovMaxSquareSize,
		MaxBlockBytes:      appconsts.DefaultMaxBytes,
		LocalTracingType:   "local",
		PushTrace:          false,
		DownloadTraces:     false,
		TestDuration:       2 * time.Minute,
		TxClients:          2,
	}

	benchTest, err := NewBenchmarkTest("TwoNodeSimple", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())

	testnet.NoError("failed to run the benchmark test", benchTest.Run())

	// post test data collection and validation

	// if local tracing is enabled,
	// pull round state traces to confirm tracing is working as expected.
	if benchTest.manifest.LocalTracingType == "local" {
		if _, err := benchTest.Node(0).PullRoundStateTraces("."); err != nil {
			return fmt.Errorf("failed to pull round state traces: %w", err)
		}
	}

	// download traces from S3, if enabled
	if benchTest.manifest.PushTrace && benchTest.manifest.DownloadTraces {
		// download traces from S3
		pushConfig, _ := trace.GetPushConfigFromEnv()
		err := trace.S3Download("./traces/", benchTest.manifest.ChainID,
			pushConfig)
		if err != nil {
			return fmt.Errorf("failed to download traces from S3: %w", err)
		}
	}

	log.Println("Reading blockchain")
	blockchain, err := testnode.ReadBlockchain(context.Background(),
		benchTest.Node(0).AddressRPC())
	testnet.NoError("failed to read blockchain", err)

	totalTxs := 0
	for _, block := range blockchain {
		if appconsts.LatestVersion != block.Version.App {
			return fmt.Errorf("expected app version %d, got %d", appconsts.LatestVersion, block.Version.App)
		}
		totalTxs += len(block.Data.Txs)
	}
	if totalTxs < 10 {
		return fmt.Errorf("expected at least 10 transactions, got %d", totalTxs)
	}

	return nil
}

func TwoNodeBigBlock_8MiB(logger *log.Logger) error {
	logger.Println("Running TwoNodeBigBlock_8MiB")
	manifest := bigBlockManifest
	manifest.TestnetName = "TwoNodeBigBlock_8MiB"
	manifest.ChainID = "two-node-big-block-8mib"
	manifest.MaxBlockBytes = 8 * toMiB

	benchTest, err := NewBenchmarkTest("TwoNodeBigBlock_8MiB", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: TwoNodeBigBlock_8MiB")
	return nil
}

func TwoNodeBigBlock_8MiB_Latency(logger *log.Logger) error {
	logger.Println("Running TwoNodeBigBlock_8MiB_Latency")
	manifest := bigBlockManifest
	manifest.TestnetName = "TwoNodeBigBlock_8MiB_Latency"
	manifest.ChainID = "two-node-big-block-8mib-latency"
	manifest.MaxBlockBytes = 8 * toMiB
	manifest.EnableLatency = true

	benchTest, err := NewBenchmarkTest("TwoNodeBigBlock_8MiB_Latency", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: TwoNodeBigBlock_8MiB_Latency")
	return nil
}

func TwoNodeBigBlock_8MiB_Bandwidth(logger *log.Logger) error {
	logger.Println("Running TwoNodeBigBlock_8MiB_Bandwidth")
	manifest := bigBlockManifest
	manifest.TestnetName = "TwoNodeBigBlock_8MiB_Bandwidth"
	manifest.ChainID = "two-node-big-block-8mib-bandwidth"
	manifest.MaxBlockBytes = 8 * toMiB
	manifest.BandwidthParams = 10000 // 10kbps

	benchTest, err := NewBenchmarkTest("TwoNodeBigBlock_8MiB_Bandwidth", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: TwoNodeBigBlock_8MiB_Latency")
	return nil
}

func TwoNodeBigBlock_32MiB(logger *log.Logger) error {
	logger.Println("Running TwoNodeBigBlock_32MiB")
	manifest := bigBlockManifest
	manifest.TestnetName = "TwoNodeBigBlock_32MiB"
	manifest.ChainID = "two-node-big-block-32mib"
	manifest.MaxBlockBytes = 32 * toMiB

	benchTest, err := NewBenchmarkTest("TwoNodeBigBlock_32MiB", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: TwoNodeBigBlock_32MiB")
	return nil
}

func TwoNodeBigBlock_64MiB(logger *log.Logger) error {
	logger.Println("Running TwoNodeBigBlock_64MiB")
	manifest := bigBlockManifest
	manifest.TestnetName = "TwoNodeBigBlock_64MiB"
	manifest.ChainID = "two-node-big-block-64mib"
	manifest.MaxBlockBytes = 64 * toMiB

	benchTest, err := NewBenchmarkTest("TwoNodeBigBlock_64MiB", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: TwoNodeBigBlock_64MiB")
	return nil
}

func LargeNetwork_BigBlock_8MiB(logger *log.Logger) error {

	latestVersion, err := testnet.GetLatestVersion()
	testnet.NoError("failed to get latest version", err)

	logger.Println("Running LargeNetwork_BigBlock_8MiB")
	manifest := bigBlockManifest
	manifest.CelestiaAppVersion = latestVersion
	manifest.TestnetName = "LargeNetwork_BigBlock_8MiB"
	manifest.ChainID = "large-network-big-block-8mib"
	manifest.MaxBlockBytes = 8 * toMiB
	manifest.Validators = 100
	manifest.TxClients = 100
	manifest.BlobSequences = 20
	manifest.TestDuration = 15 * time.Minute

	benchTest, err := NewBenchmarkTest("LargeNetwork_BigBlock_8MiB", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: LargeNetwork_BigBlock_8MiB")
	return nil
}

func LargeNetwork_BigBlock_8MiB_Latency(logger *log.Logger) error {
	logger.Println("Running LargeNetwork_BigBlock_8MiB_Latency")
	manifest := bigBlockManifest
	manifest.TestnetName = "LargeNetwork_BigBlock_8MiB_Latency"
	manifest.ChainID = "large-network-big-block-8mib-latency"
	manifest.MaxBlockBytes = 8 * toMiB
	manifest.Validators = 100
	manifest.TxClients = 100
	manifest.BlobSequences = 20
	manifest.TestDuration = 30 * time.Minute
	manifest.EnableLatency = true
	manifest.LatencyParams = LatencyParams{70, 0} // in  milliseconds

	benchTest, err := NewBenchmarkTest("LargeNetwork_BigBlock_8MiB_Latency", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: LargeNetwork_BigBlock_8MiB_Latency")
	return nil
}

func LargeNetwork_BigBlock_32MiB(logger *log.Logger) error {
	logger.Println("Running LargeNetwork_BigBlock_32MiB")
	manifest := bigBlockManifest
	manifest.TestnetName = "LargeNetwork_BigBlock_32MiB"
	manifest.ChainID = "large-network-big-block-32mib"
	manifest.MaxBlockBytes = 32 * toMiB
	manifest.Validators = 50
	manifest.TxClients = 50
	manifest.BlobSequences = 20

	benchTest, err := NewBenchmarkTest("LargeNetwork_BigBlock_32MiB", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: LargeNetwork_BigBlock_32MiB")
	return nil
}

func LargeNetwork_BigBlock_64MiB(logger *log.Logger) error {
	logger.Println("Running LargeNetwork_BigBlock_64MiB")
	manifest := bigBlockManifest
	manifest.TestnetName = "LargeNetwork_BigBlock_64MiB"
	manifest.ChainID = "large-network-big-block-64mib"
	manifest.MaxBlockBytes = 64 * toMiB
	manifest.Validators = 50
	manifest.TxClients = 50
	manifest.BlobSequences = 20

	benchTest, err := NewBenchmarkTest("LargeNetwork_BigBlock_64MiB", &manifest)
	testnet.NoError("failed to create benchmark test", err)

	defer func() {
		log.Print("Cleaning up testnet")
		benchTest.Cleanup()
	}()

	testnet.NoError("failed to setup nodes", benchTest.SetupNodes())
	testnet.NoError("failed to run the benchmark test", benchTest.Run())
	testnet.NoError("failed to check results", benchTest.CheckResults())

	log.Println("--- PASS ✅: LargeNetwork_BigBlock_64MiB")
	return nil

}
