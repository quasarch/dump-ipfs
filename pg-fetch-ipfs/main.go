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

}
