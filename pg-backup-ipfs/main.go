package main

import (
	"context"
	"fmt"
	"github.com/spf13/afero"
	"github.com/web3-storage/go-w3s-client"
	"io"
	"io/fs"
	"os"
	"time"
)

func main() {
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	// Store in Filecoin with a timestamp.
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweGJGN2RCRTU5RkUyNzEyMzMyQTIzREU2MTE3NTUzNzY5MzE2ZjFENzUiLCJpc3MiOiJ3ZWIzLXN0b3JhZ2UiLCJpYXQiOjE2OTAwMjEwNzE4NTcsIm5hbWUiOiJoYWNrYXRob24ifQ.NOvJIxbLrxAQvrG0Cp7F9ltmyZjT0EI_u8e28ruYFf4"
	client, err := w3s.NewClient(w3s.WithToken(token))
	if err != nil {
		panic(err)
	}

	file, err := writeFileInMemory(stdin)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	cid, err := client.Put(ctx, file)
	if err != nil {
		panic(err)
	}

	fmt.Printf("https://ipfs.io/ipfs/%s\n", cid)

	res, err := client.Get(ctx, cid)
	if err != nil {
		panic(err)
	}

	res.Files()
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
