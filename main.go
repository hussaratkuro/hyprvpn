package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"gioui.org/app"

	"github.com/hussaratkuro/hyprvpn/internal/openvpn3"
	"github.com/hussaratkuro/hyprvpn/internal/ui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()
	if *showVersion {
		fmt.Printf("hyprvpn %s\n", version)
		return
	}
	if os.Getenv("HYPRVPN_DEMO") != "" {
		go func() {
			if err := ui.Run(ui.NewDemoBackend(), version+" demo"); err != nil {
				log.Fatal(err)
			}
			os.Exit(0)
		}()
		app.Main()
		return
	}
	client, err := openvpn3.New()
	if err != nil {
		message := err.Error()
		if errors.Is(err, openvpn3.ErrSystemBus) {
			message += "\n\nThe OpenVPN 3 Linux system bus is unavailable. Check that the openvpn3 package is installed and its D-Bus services are running."
		}
		fmt.Fprintln(os.Stderr, "hyprvpn:", message)
		os.Exit(1)
	}
	go func() {
		err := ui.Run(client, version)
		client.Close()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
