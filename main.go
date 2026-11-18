package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: subsoil serve --config <configuration.json> [--listen <address>]")
		return nil
	}
	if args[0] != "serve" {
		return fmt.Errorf("unknown command %q", args[0])
	}
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := flags.String("config", "", "Path to the service configuration")
	listen := flags.String("listen", "127.0.0.1:5000", "HTTP listen address")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *configPath == "" || flags.NArg() != 0 {
		return errors.New("use subsoil serve --config <configuration.json> [--listen <address>]")
	}
	file, err := os.Open(*configPath)
	if err != nil {
		return err
	}
	var config configuration
	err = decodeJSON(file, &config)
	file.Close()
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	svc, err := newService(config)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           svc.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result := make(chan error, 1)
	go func() {
		log.Printf("Serving Subsoil on %s", server.Addr)
		result <- server.ListenAndServe()
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		deadline, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return server.Shutdown(deadline)
	}
}
