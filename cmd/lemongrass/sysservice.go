package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/faizalv/lemongrass/config"
)

const (
	systemdServiceName = "lemongrass-daemon"
	launchdLabel       = "dev.lemongrass.daemon"
)

func installAndStartDaemon(binPath string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = installSystemdUnit(binPath)
	case "darwin":
		err = installLaunchAgent(binPath)
	default:
		startDaemonProcess(binPath)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not install daemon service (%v) -- falling back to process\n", err)
		startDaemonProcess(binPath)
		return
	}
	waitForDaemonSocket()
}

func uninstallDaemon() {
	switch runtime.GOOS {
	case "linux":
		uninstallSystemdUnit()
	case "darwin":
		uninstallLaunchAgent()
	default:
		killDaemonByPid()
	}
	os.Remove(filepath.Join(config.Dir(), "lemongrass.sock"))
	os.Remove(filepath.Join(config.Dir(), "lg-daemon.pid"))
}

func installSystemdUnit(binPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return err
	}
	unit := "[Unit]\n" +
		"Description=Lemongrass Daemon\n" +
		"After=default.target\n\n" +
		"[Service]\n" +
		"ExecStart=" + binPath + " start-daemon\n" +
		"Restart=on-failure\n" +
		"RestartSec=3\n\n" +
		"[Install]\n" +
		"WantedBy=default.target\n"
	unitPath := filepath.Join(unitDir, systemdServiceName+".service")
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return err
	}
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	return exec.Command("systemctl", "--user", "enable", "--now", systemdServiceName).Run()
}

func installLaunchAgent(binPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	agentDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return err
	}
	logPath := filepath.Join(config.Dir(), "logs", "lg-daemon.log")
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchdLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + binPath + `</string>
		<string>start-daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardErrorPath</key>
	<string>` + logPath + `</string>
</dict>
</plist>
`
	plistPath := filepath.Join(agentDir, launchdLabel+".plist")
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return err
	}
	exec.Command("launchctl", "unload", plistPath).Run()
	return exec.Command("launchctl", "load", plistPath).Run()
}

func uninstallSystemdUnit() {
	exec.Command("systemctl", "--user", "stop", systemdServiceName).Run()
	exec.Command("systemctl", "--user", "disable", systemdServiceName).Run()
	home, err := os.UserHomeDir()
	if err == nil {
		os.Remove(filepath.Join(home, ".config", "systemd", "user", systemdServiceName+".service"))
	}
	exec.Command("systemctl", "--user", "daemon-reload").Run()
}

func uninstallLaunchAgent() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
	exec.Command("launchctl", "unload", plistPath).Run()
	os.Remove(plistPath)
}

func startDaemonProcess(binPath string) {
	cmd := exec.Command(binPath, "start-daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start lemongrass-daemon: %v\n", err)
		return
	}
	waitForDaemonSocket()
}

func waitForDaemonSocket() {
	sockPath := filepath.Join(config.Dir(), "lemongrass.sock")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "warning: lemongrass-daemon did not start in time")
}

func killDaemonByPid() {
	pidPath := filepath.Join(config.Dir(), "lg-daemon.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	proc.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(config.Dir(), "lemongrass.sock")); os.IsNotExist(err) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
