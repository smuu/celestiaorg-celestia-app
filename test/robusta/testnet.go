package main

import (
	"fmt"
	"os"
	"time"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/knuu/pkg/knuu"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/rs/zerolog/log"
)

type Testnet struct {
	seed            int64
	nodes           []*Node
	genesisAccounts []*GenesisAccount
	keygen          *keyGenerator
	Txsim           *Txsim
}

func New(name string, seed int64) (*Testnet, error) {
	identifier := fmt.Sprintf("%s_%s", name, time.Now().Format("20060102_150405"))
	if err := knuu.InitializeWithIdentifier(identifier); err != nil {
		return nil, err
	}

	return &Testnet{
		seed:            seed,
		nodes:           make([]*Node, 0),
		genesisAccounts: make([]*GenesisAccount, 0),
		keygen:          newKeyGenerator(seed),
		Txsim:           nil,
	}, nil
}

func (t *Testnet) CreateGenesisNode(version string, selfDelegation int64) error {
	signerKey := t.keygen.Generate(ed25519Type)
	networkKey := t.keygen.Generate(ed25519Type)
	accountKey := t.keygen.Generate(secp256k1Type)
	node, err := NewNode(fmt.Sprintf("val%d", len(t.nodes)), version, 0, selfDelegation, nil, signerKey, networkKey, accountKey)
	if err != nil {
		return err
	}
	t.nodes = append(t.nodes, node)
	return nil
}

func (t *Testnet) CreateGenesisNodes(num int, version string, selfDelegation int64) error {
	for i := -0; i < num; i++ {
		if err := t.CreateGenesisNode(version, selfDelegation); err != nil {
			return err
		}
	}
	return nil
}

func (t *Testnet) CreateNode(version string, startHeight int64) error {
	signerKey := t.keygen.Generate(ed25519Type)
	networkKey := t.keygen.Generate(ed25519Type)
	accountKey := t.keygen.Generate(secp256k1Type)
	node, err := NewNode(fmt.Sprintf("node%d", len(t.nodes)), version, startHeight, 0, nil, signerKey, networkKey, accountKey)
	if err != nil {
		return err
	}
	t.nodes = append(t.nodes, node)
	return nil
}

func (t *Testnet) CreateNodes(num int, version string, startHeight int64) error {
	for i := 0; i < num; i++ {
		if err := t.CreateNode(version, startHeight); err != nil {
			return err
		}
	}
	return nil
}

func (t *Testnet) CreateTxSim(
	mnemomic, version string,
	pollTime time.Duration,
	blobSizes []int,
	blob, blobAmounts, seed, send int,
) error {
	signerKey := t.keygen.Generate(ed25519Type)
	networkKey := t.keygen.Generate(ed25519Type)
	accountKey := t.keygen.Generate(secp256k1Type)

	txSimNode, err := NewTxsimNode("txsim", version, signerKey, networkKey, accountKey, mnemomic, t.ExternalRPCEndpoints(), t.ExternalGRPCEndpoints(), pollTime, blobSizes, blob, blobAmounts, seed, send)
	if err != nil {
		return err
	}
	t.Txsim = txSimNode
	return nil
}

func (t *Testnet) CreateGenesisAccount(name string, tokens int64) (keyring.Keyring, string, error) {
	cdc := encoding.MakeConfig(app.ModuleEncodingRegisters...).Codec
	kr := keyring.NewInMemory(cdc)
	key, mnemomic, err := kr.NewMnemonic(name, keyring.English, "", "", hd.Secp256k1)
	if err != nil {
		return nil, "", err
	}
	pk, err := key.GetPubKey()
	if err != nil {
		return nil, "", err
	}
	t.genesisAccounts = append(t.genesisAccounts, &GenesisAccount{
		PubKey:        pk,
		InitialTokens: tokens,
	})
	return kr, mnemomic, nil
}

func (t *Testnet) Setup() error {
	genesisNodes := make([]*Node, 0)
	for _, node := range t.nodes {
		if node.StartHeight == 0 && node.SelfDelegation > 0 {
			genesisNodes = append(genesisNodes, node)
		}
	}
	genesis, err := MakeGenesis(genesisNodes, t.genesisAccounts)
	if err != nil {
		return err
	}
	for _, node := range t.nodes {
		// nodes are initialized with the addresses of all other
		// nodes in their addressbook
		peers := make([]string, 0, len(t.nodes)-1)
		for _, peer := range t.nodes {
			if peer.Name != node.Name {
				peers = append(peers, peer.AddressP2P(true))
			}
		}

		log.Info().Str("name", node.Name).Msg("Initializing node")

		err = node.Init(genesis, peers)
		if err != nil {
			return err
		}
	}

	if t.Txsim != nil {
		err := t.Txsim.Init()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Testnet) RPCEndpoints() []string {
	rpcEndpoints := make([]string, len(t.nodes))
	for idx, node := range t.nodes {
		rpcEndpoints[idx] = node.AddressRPC()
	}
	return rpcEndpoints
}

func (t *Testnet) ExternalRPCEndpoints() []string {
	rpcEndpoints := make([]string, len(t.nodes))
	for idx, node := range t.nodes {
		rpcEndpoints[idx] = node.ExternalAddressRPC()
	}
	return rpcEndpoints
}

func (t *Testnet) GRPCEndpoints() []string {
	grpcEndpoints := make([]string, len(t.nodes))
	for idx, node := range t.nodes {
		grpcEndpoints[idx] = node.AddressGRPC()
	}
	return grpcEndpoints
}

func (t *Testnet) ExternalGRPCEndpoints() []string {
	grpcEndpoints := make([]string, len(t.nodes))
	for idx, node := range t.nodes {
		grpcEndpoints[idx] = node.ExternalAddressGRPC()
	}
	return grpcEndpoints
}

func (t *Testnet) Start() error {
	genesisNodes := make([]*Node, 0)
	for _, node := range t.nodes {
		if node.StartHeight == 0 {
			genesisNodes = append(genesisNodes, node)
		}
	}
	for _, node := range genesisNodes {
		err := node.Start()
		log.Info().Str("name", node.Name).Msg("Starting node")
		if err != nil {
			return fmt.Errorf("node %s failed to start: %w", node.Name, err)
		}
	}
	for _, node := range t.nodes {
		if node.StartHeight > 0 {
			err := node.Start()

			log.Info().Str("name", node.Name).Msg("Starting node")
			if err != nil {
				return fmt.Errorf("node %s failed to start: %w", node.Name, err)
			}
		}
	}
	//for _, node := range genesisNodes {
	//	client, err := node.Client()
	//	if err != nil {
	//		return fmt.Errorf("failed to initialized node %s: %w", node.Name, err)
	//	}
	//	for i := 0; i < 10; i++ {
	//		resp, err := client.Status(context.Background())
	//		if err != nil {
	//			return fmt.Errorf("node %s status response: %w", node.Name, err)
	//		}
	//		if resp.SyncInfo.LatestBlockHeight > 0 {
	//			break
	//		}
	//		if i == 9 {
	//			return fmt.Errorf("failed to start node %s", node.Name)
	//		}
	//		time.Sleep(time.Second)
	//	}
	//}
	if t.Txsim != nil {
		err := t.Txsim.Start()
		if err != nil {
			return fmt.Errorf("failed to start txsim: %w", err)
		}
	}
	return nil
}

func (t *Testnet) Cleanup() {
	if os.Getenv("KNUU_SKIP_CLEANUP") == "true" {
		log.Info().Msg("skipping cleanup")
		return
	}
	for _, node := range t.nodes {
		err := node.Instance.Destroy()
		if err != nil {
			log.Err(err).Msg(fmt.Sprintf("node %s failed to cleanup", node.Name))
		}
	}
}

func (t *Testnet) Node(i int) *Node {
	return t.nodes[i]
}
