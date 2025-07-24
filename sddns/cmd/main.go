package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/BPplays/dns-set-go/sddns"
)

func main() {
	configLocation := flag.String("config_location", sddns.ConfigLocationDefault, "")
	timeout := flag.Int("timeout_sec", 25, "")

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout) * time.Second)
		defer cancel()

		sddns.Run(ctx, configLocation, log.Default())

}
