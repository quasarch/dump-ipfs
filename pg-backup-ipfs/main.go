package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/spf13/afero"
	w3s "github.com/web3-storage/go-w3s-client"
	"io/fs"
	"log"
	"os"
	"time"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		log.Println("Data on stdin...")
		stdin := scanner.Text()

		// Store in Filecoin with a timestamp.
		client, err := w3s.NewClient(w3s.WithToken(os.Getenv("API_KEY")))
		if err != nil {
			panic(err)
		}

		file, err := writeFileInMemory([]byte(stdin))
		if err != nil {
			panic(err)
		}

		ctx := context.Background()
		cid, err := client.Put(ctx, file)
		if err != nil {
			panic(err)
		}

		fmt.Printf("https://ipfs.io/ipfs/%s\n", cid)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalln(err)
	}
}

func writeFileInMemory(data []byte) (fs.File, error) {
	// Create a file with a timestamp in memory only.
	t := time.Now()
	filename := fmt.Sprintf("dump-%s.sql", t.Format(`20060102-150405`))
	mem_fs := afero.NewMemMapFs()
	file, err := mem_fs.Create(filename)
	if err != nil {
		return nil, err
	}

	if _, err := file.Write(data); err != nil {
		return nil, err
	}

	// reset the read pointer to the start of the file
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}

	return file, err
}
