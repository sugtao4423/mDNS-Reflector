package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	r "github.com/sugtao4423/mDNS-Reflector/reflector"
)

var Version = "dev"

func main() {
	var (
		interfaces  string
		debug       bool
		showIfaces  bool
		showVersion bool
	)

	flag.StringVar(&interfaces, "i", "", "Comma-separated list of interface names (e.g., eth0,eth1)")
	flag.BoolVar(&debug, "d", false, "Enable debug logging")
	flag.BoolVar(&showIfaces, "l", false, "List available network interfaces")
	flag.BoolVar(&showVersion, "v", false, "Show version information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mDNS Reflector %s - Reflect mDNS packets between network interfaces\n\n", Version)
		fmt.Fprintf(os.Stderr, "Usage: %s -i <interface1,interface2,...>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -i eth0,wlan0\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -i eth0,eth1,docker0 -d\n", os.Args[0])
	}

	flag.Parse()

	if showVersion {
		fmt.Printf("mDNS Reflector %s\n", Version)
		return
	}

	if showIfaces {
		listInterfaces()
		return
	}

	if interfaces == "" {
		fmt.Fprintf(os.Stderr, "Error: No interfaces specified\n\n")
		fmt.Fprintf(os.Stderr, "Use -i flag\n\n")
		flag.Usage()
		os.Exit(1)
	}

	ifaceNames := strings.Split(interfaces, ",")
	for i := range ifaceNames {
		ifaceNames[i] = strings.TrimSpace(ifaceNames[i])
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("Starting mDNS Reflector...")
	log.Printf("Interfaces: %v", ifaceNames)

	reflector, err := r.NewReflector(ifaceNames, debug)
	if err != nil {
		log.Fatalf("Failed to create reflector: %v", err)
	}

	if err := reflector.Start(); err != nil {
		log.Fatalf("Failed to start reflector: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal %v, shutting down...", sig)

	reflector.Stop()
	log.Printf("mDNS Reflector stopped")
}

func listInterfaces() {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatalf("Failed to list interfaces: %v", err)
	}

	fmt.Println("Available network interfaces:")
	fmt.Println("-----------------------------")
	for _, iface := range ifaces {
		flags := []string{}
		if iface.Flags&net.FlagUp != 0 {
			flags = append(flags, "UP")
		}
		if iface.Flags&net.FlagMulticast != 0 {
			flags = append(flags, "MULTICAST")
		}
		if iface.Flags&net.FlagLoopback != 0 {
			flags = append(flags, "LOOPBACK")
		}

		addrs, _ := iface.Addrs()
		addrStrs := []string{}
		for _, addr := range addrs {
			addrStrs = append(addrStrs, addr.String())
		}

		fmt.Printf("  %s: [%s]\n", iface.Name, strings.Join(flags, ", "))
		if len(addrStrs) > 0 {
			fmt.Printf("    Addresses: %s\n", strings.Join(addrStrs, ", "))
		}
	}
}
