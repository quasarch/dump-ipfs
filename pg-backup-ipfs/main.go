package main

import (
	"io"
	"os"
)

func main() {
	stdin, err := io.ReadAll(os.Stdin)

	if err != nil {
		panic(err)
	}

	// Store in Filecoin with a timestamp.

}
