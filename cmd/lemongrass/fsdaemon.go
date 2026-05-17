package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/faizalv/lemongrass/config"
)

func cmdFsDaemon() {
	sockPath := filepath.Join(config.Dir(), "fs.sock")
	pidPath := filepath.Join(config.Dir(), "fs-daemon.pid")

	os.Remove(sockPath)

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fs-daemon: %v\n", err)
		os.Exit(1)
	}

	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		l.Close()
	}()

	defer os.Remove(sockPath)
	defer os.Remove(pidPath)

	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go serveConn(conn)
	}
}

func serveConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	switch scanner.Text() {
	case "BROWSE":
		w := bufio.NewWriter(conn)
		walkFS(w)
		w.Flush()

	case "REMOUNT":
		var paths []string
		for scanner.Scan() {
			if p := scanner.Text(); p != "" {
				paths = append(paths, p)
			}
		}
		if len(paths) > 0 {
			go exec.Command("lemongrass", append([]string{"remount"}, paths...)...).Start()
		}
	}
}
