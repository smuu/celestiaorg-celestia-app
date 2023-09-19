package main

import "time"

const (
	latestVersion = "v1.0.0-rc12"
	txSimVersion  = "a9e2acd"
	seed          = 42 // the meaning of life
)

func main() {

	name := "test"

	testnet, err := New(name, seed)
	if err != nil {
		panic(err)
	}
	err = testnet.CreateGenesisNodes(2, latestVersion, 10000000)
	if err != nil {
		panic(err)
	}

	err = testnet.CreateNodes(2, latestVersion, 0)
	if err != nil {
		panic(err)
	}

	_, mnemomic, err := testnet.CreateGenesisAccount("txsim", 1e12)
	if err != nil {
		panic(err)
	}

	err = testnet.CreateTxSim(mnemomic, txSimVersion, 15*time.Second, []int{50000, 100000}, 100, 1, 50, 100)
	if err != nil {
		panic(err)
	}

	err = testnet.Setup()
	if err != nil {
		panic(err)
	}

	err = testnet.Start()
	if err != nil {
		panic(err)
	}
}
