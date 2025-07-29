package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/BPplays/dns-set-go/sddns"
)

func main() {
	var ctx context.Context
	var cancel context.CancelFunc

	configLocation := flag.String("config_dir", sddns.ConfigLocationDefault, "")
	timeout := flag.Int("timeout_sec", -1, "")

	flag.Parse()

	if *timeout >= 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(*timeout) * time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	log.Println("running sddns")
	sddns.Run(ctx, configLocation, log.Default())

}
