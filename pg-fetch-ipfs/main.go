package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/web3-storage/go-w3s-client"
	"os"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	cidStr, err := reader.ReadString('\n')
	fmt.Printf("CID: %s\n", cidStr)
	if err != nil {
		panic(err)
	}
	cidValue, err := cid.Decode(cidStr)
	if err != nil {
		panic(err)
	}

	client, err := w3s.NewClient(w3s.WithToken(os.Getenv("API_KEY")))
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	fmt.Printf("Fetching %s\n", cidValue)
	res, err := client.Get(ctx, cidValue)
	if err != nil {
		panic(err)
	}

	f, _, err := res.Files()
	if err != nil {
		panic(err)
	}

	info, err := f.Stat()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s - File size: %d\n", cidValue, info.Size())

	status, err := client.Status(ctx, cidValue)
	if err != nil {
		panic(err)
	}
	fmt.Printf("DAG size: %d\n", status.DagSize)

	fmt.Println("IPFS pins:")
	for _, p := range status.Pins {
		fmt.Printf("IPFS peer ID: %s\n", p.PeerID)
		fmt.Printf("IPFS peer Name: %s\n", p.PeerName)
		fmt.Printf("Region: %s\n", p.Region)
		fmt.Printf("Pin status: %s\n", p.Status)
	}

	// expected to be empty since it takes around 48h
	fmt.Println("Filecoin deals:")
	for _, d := range status.Deals {
		fmt.Printf("Storage Provider: %s\n", d.StorageProvider)
		fmt.Printf("Deal ID: %d\n", d.DealID)
		fmt.Printf("Deal status: %s\n", d.Status)
		fmt.Printf("Data CID: %s\n", d.DataCid)
		fmt.Printf("Data Model: %s\n", d.DataModelSelector)
	}
}
