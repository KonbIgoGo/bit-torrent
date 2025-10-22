package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/KonbIgoGo/Bit-torrent/client"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	in := "debian-13.0.0-amd64-netinst.iso.torrent"
	file, err := os.Open(in)
	defer file.Close()
	if err != nil {
		log.Fatal(err)
	}

	client, err := client.New(file)
	if err != nil {
		log.Fatal(err)
	}
	err = client.StartDownloadingTo(ctx, "")
	if err != nil {
		log.Fatal(err)
	}
}
