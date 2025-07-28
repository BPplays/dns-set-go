package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/BPplays/dns-set-go/sddns"
)

func main() {
	configLocation := flag.String("config_dir", sddns.ConfigLocationDefault, "")
	timeout := flag.Int("timeout_sec", 25, "")

	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout) * time.Second)
	defer cancel()

	log.Println("running sddns")
	sddns.Run(ctx, configLocation, log.Default())

}
