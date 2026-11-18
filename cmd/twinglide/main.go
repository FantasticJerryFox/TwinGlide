package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"twinglide/internal/link"
)

func main() {
	root, _ := os.UserConfigDir()
	if runtime.GOOS == "windows" {
		root = os.Getenv("LOCALAPPDATA")
	}
	data := flag.String("data-dir", filepath.Join(root, "TwinGlide"), "private application data directory")
	peerPort := flag.Int("peer-port", 59121, "TLS control port")
	discoveryPort := flag.Int("discovery-port", 59120, "LAN discovery port")
	uiPort := flag.Int("ui-port", 0, "loopback settings port (0 chooses a free port)")
	noInput := flag.Bool("no-input", false, "exercise discovery/pairing without running an input engine")
	audioTest := flag.Bool("audio-test", false, "synthetic audio transport only; disables all physical input/capture/playback")
	noDiscovery := flag.Bool("no-discovery", false, "disable LAN discovery (tests and restricted networks)")
	engine := flag.String("engine", "", "bundled Deskflow engine executable")
	parent := flag.Bool("parent-stdin", false, "exit when the desktop launcher's input pipe closes")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *parent {
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	}
	a, err := link.New(link.Options{DataDir: *data, PeerPort: *peerPort, DiscoveryPort: *discoveryPort, UIPort: *uiPort, NoInput: *noInput || *audioTest, AudioTest: *audioTest, NoDiscovery: *noDiscovery, EnginePath: *engine})
	if err == nil {
		err = a.Run(ctx)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
