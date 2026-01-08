package main

import (
	"fmt"
	"log"
	"os"

	"blogcms/internal/app"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <password>", os.Args[0])
	}
	hash, err := app.HashPassword(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(hash)
}
