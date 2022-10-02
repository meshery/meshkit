package main

import (
	"log"
	"os"

	"github.com/layer5io/meshkit/cmd/syncmodutil/internal/modsync"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("modsync <src> <dest>")
	}
	src := os.Args[1]
	dest := os.Args[2]
	f, err := os.Open(src)
	if err != nil {
		log.Fatal(err)
	}
	g, err := modsync.New(f)
	if err != nil {
		log.Fatal(err)
	}
	f2, err := os.Open(dest)
	if err != nil {
		log.Fatal(err)
	}
	newgomod, err := g.SyncRequire(f2)
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(dest, []byte(newgomod), 0777)
	if err != nil {
		log.Fatal(err)
	}
	g.PrintReplacedVersions()
}
