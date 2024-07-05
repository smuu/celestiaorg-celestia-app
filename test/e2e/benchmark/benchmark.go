//nolint:staticcheck
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

type BenchmarkTest struct {
	*testnet.Testnet
	manifest *Manifest
}

func NewBenchmarkTest(name string, manifest *Manifest) (*BenchmarkTest, error) {
	// create a new testnet
	testNet, err := testnet.New(name, seed,
		testnet.GetGrafanaInfoFromEnvVar(), manifest.ChainID,
		manifest.GetGenesisModifiers()...)
	if err != nil {
		return nil, err
	}

	testNet.SetConsensusParams(manifest.GetConsensusParams())
	return &BenchmarkTest{Testnet: testNet, manifest: manifest}, nil
}

// SetupNodes creates genesis nodes and tx clients based on the manifest.
// There will be manifest.Validators validators and manifest.TxClients tx clients.
// Each tx client connects to one validator. If TxClients are fewer than Validators, some validators will not have a tx client.
func (b *BenchmarkTest) SetupNodes() error {
	if b.manifest.Validators == 1 {
		err := b.CreateGenesisNodes(1, b.manifest.CelestiaAppVersion, b.manifest.SelfDelegation, b.manifest.UpgradeHeight, b.manifest.ValidatorResource, true)
		if err != nil {
			return fmt.Errorf("failed to create genesis node: %v", err)
		}
	} else {
		err := b.CreateGenesisNodes(1, b.manifest.CelestiaAppVersion, b.manifest.SelfDelegation, b.manifest.UpgradeHeight, b.manifest.ValidatorResource, true)
		if err != nil {
			return fmt.Errorf("failed to create genesis nodes with tsharkToS3 enabled: %v", err)
		}
		err = b.CreateGenesisNodes(1, b.manifest.CelestiaAppVersion, b.manifest.SelfDelegation, b.manifest.UpgradeHeight, b.manifest.ValidatorResource, true)
		if err != nil {
			return fmt.Errorf("failed to create genesis nodes with tsharkToS3 enabled: %v", err)
		}
		if b.manifest.Validators > 2 {
			err = b.CreateGenesisNodes(b.manifest.Validators-2, b.manifest.CelestiaAppVersion, b.manifest.SelfDelegation, b.manifest.UpgradeHeight, b.manifest.ValidatorResource, false)
			if err != nil {
				return fmt.Errorf("failed to create remaining genesis nodes: %v", err)
			}
		}
	}
	// enable latency if specified in the manifest
	if b.manifest.EnableLatency || b.manifest.BandwidthParams > 0 {
		for _, node := range b.Nodes() {
			if err := node.Instance.EnableBitTwister(); err != nil {
				return fmt.Errorf("failed to enable bit twister: %v", err)
			}
		}
	}

	// obtain the GRPC endpoints of the validators
	gRPCEndpoints, err := b.RemoteGRPCEndpoints()
	if err != nil {
		return fmt.Errorf("failed to get validators GRPC endpoints: %v", err)

	}
	log.Println("validators GRPC endpoints", gRPCEndpoints)
	// create tx clients and point them to the validators
	log.Println("Creating tx clients")

	err = b.CreateTxClients(b.manifest.TxClientVersion,
		b.manifest.BlobSequences,
		b.manifest.BlobSizes,
		b.manifest.BlobsPerSeq,
		b.manifest.TxClientsResource, gRPCEndpoints)
	if err != nil {
		return fmt.Errorf("failed to create tx clients: %v", err)

	}

	log.Println("Setting up testnet")
	err = b.Setup(
		testnet.WithPerPeerBandwidth(b.manifest.PerPeerBandwidth),
		testnet.WithTimeoutPropose(b.manifest.TimeoutPropose),
		testnet.WithTimeoutCommit(b.manifest.TimeoutCommit),
		testnet.WithPrometheus(b.manifest.Prometheus),
		testnet.WithLocalTracing(b.manifest.LocalTracingType),
	)
	if err != nil {
		return fmt.Errorf("failed to setup testnet: %v", err)
	}

	if b.manifest.PushTrace {
		log.Println("reading trace push config")
		if pushConfig, err := trace.GetPushConfigFromEnv(); err == nil {
			log.Print("Setting up trace push config")
			for _, node := range b.Nodes() {
				if err = node.Instance.SetEnvironmentVariable(trace.PushBucketName, pushConfig.BucketName); err != nil {
					return fmt.Errorf("failed to set TRACE_PUSH_BUCKET_NAME: %v", err)
				}
				if err = node.Instance.SetEnvironmentVariable(trace.PushRegion, pushConfig.Region); err != nil {
					return fmt.Errorf("failed to set TRACE_PUSH_REGION: %v", err)
				}
				if err = node.Instance.SetEnvironmentVariable(trace.PushAccessKey, pushConfig.AccessKey); err != nil {
					return fmt.Errorf("failed to set TRACE_PUSH_ACCESS_KEY: %v", err)
				}
				if err = node.Instance.SetEnvironmentVariable(trace.PushKey, pushConfig.SecretKey); err != nil {
					return fmt.Errorf("failed to set TRACE_PUSH_SECRET_KEY: %v", err)
				}
				if err = node.Instance.SetEnvironmentVariable(trace.PushDelay, fmt.Sprintf("%d", pushConfig.PushDelay)); err != nil {
					return fmt.Errorf("failed to set TRACE_PUSH_DELAY: %v", err)
				}
			}
		}
	}
	return nil
}

// Run runs the benchmark test for the specified duration in the manifest.
func (b *BenchmarkTest) Run() error {
	log.Println("Starting testnet")
	ctx := context.Background()

	err := b.StartWithFunc(
		func(node *testnet.Node) error {
			if !b.manifest.EnableLatency && b.manifest.BandwidthParams <= 0 {
				return nil
			}

			if err := node.Instance.BitTwister.WaitForStart(ctx); err != nil {
				return fmt.Errorf("failed to wait for bit twister to start: %v", err)
			}

			if b.manifest.EnableLatency {
				err := node.Instance.SetLatencyAndJitter(
					b.manifest.LatencyParams.Latency,
					b.manifest.LatencyParams.Jitter,
				)
				if err != nil {
					return fmt.Errorf("failed to set latency and jitter: %v", err)
				}
			}

			if b.manifest.BandwidthParams > 0 {
				err := node.Instance.SetBandwidthLimit(b.manifest.BandwidthParams)
				if err != nil {
					return fmt.Errorf("failed to set bandwidth: %v", err)
				}
			}
			return nil
		})
	if err != nil {
		return fmt.Errorf("failed to start testnet: %v", err)
	}

	// add latency if specified in the manifest

	// wait some time for the tx clients to submit transactions
	log.Println("Waiting for", b.manifest.TestDuration, "for the tx clients to submit transactions")
	time.Sleep(b.manifest.TestDuration)
	log.Println("Tx clients have submitted transactions")

	return nil
}

func (b *BenchmarkTest) CheckResults() error {
	log.Println("Checking results")

	// If pulling traced data is enabled, pull the data and return an error if it fails
	if true {
		if _, err := b.Node(0).PullRoundStateTraces("."); err != nil {
			return fmt.Errorf("failed to pull round state traces: %w", err)
		}
		if _, err := b.Node(0).PullReceivedBytes(); err != nil {
			return fmt.Errorf("failed to pull received bytes traces: %w", err)
		}
	}

	// check if any tx has been submitted
	log.Println("Reading blockchain, this may take a while...")
	blockchain, err := testnode.ReadBlockchain(context.Background(), b.Node(0).AddressRPC())
	if err != nil {
		return fmt.Errorf("failed to read blockchain: %w", err)

	}

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

	// save the blockchain headers to a CSV file
	err = SaveToCSV(extractHeaders(blockchain),
		fmt.Sprintf("./blockchain_%s.csv", b.manifest.TestnetName))
	if err != nil {
		log.Println("failed to save blockchain headers to a CSV file", err)
	}

	return nil
}
