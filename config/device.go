package config

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type DeviceInfo struct {
	MemoryMB int    `json:"memory_mb"`
	CPUCores int    `json:"cpu_cores"`
	Tier     string `json:"tier"`
}

func devicePath() string {
	return filepath.Join(Dir(), "device.json")
}

func DetectAndSaveDevice() error {
	info := detectDevice()
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(devicePath(), data, 0644)
}

func LoadDevice() DeviceInfo {
	data, err := os.ReadFile(devicePath())
	if err != nil {
		return DeviceInfo{Tier: "unknown"}
	}
	var info DeviceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return DeviceInfo{Tier: "unknown"}
	}
	return info
}

func detectDevice() DeviceInfo {
	mem := readMemoryMB()
	cores := runtime.NumCPU()
	return DeviceInfo{
		MemoryMB: mem,
		CPUCores: cores,
		Tier:     deviceTier(mem),
	}
}

func readMemoryMB() int {
	// Linux
	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, _ := strconv.Atoi(fields[1])
					return kb / 1024
				}
			}
		}
	}
	// macOS
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		b, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		return int(b / (1024 * 1024))
	}
	return 0
}

func deviceTier(memMB int) string {
	switch {
	case memMB >= 16*1024:
		return "high"
	case memMB >= 8*1024:
		return "mid"
	case memMB > 0:
		return "low"
	default:
		return "unknown"
	}
}
