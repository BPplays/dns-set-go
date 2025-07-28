package sddns

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	dnsSet "github.com/BPplays/dns-set-go"

	"gopkg.in/yaml.v3"

	"github.com/vishvananda/netlink"
)

const (
	debug = false
)

var (
	BaseLocationDefault = filepath.Join(SystemConfigDir(), "sddns")
	ConfigLocationDefault = filepath.Join(BaseLocationDefault, "config.d")
)

type Service struct {
    Name           string   `yaml:"name"`
    Type           string   `yaml:"type"`
    Hostnames      []string `yaml:"hostnames"`
    IPv6Type       string   `yaml:"ipv6_type"`
    IPv6Interfaces []string `yaml:"ipv6_interfaces"`
    IPv4Type       string   `yaml:"ipv4_type"`
    IPv4Interfaces []string `yaml:"ipv4_interfaces"`
    TTL string `yaml:"ttl"`
    APIKey         string   `yaml:"api_key"`
    APISecretKey   string   `yaml:"api_secret_key"`
    Username       string   `yaml:"username"`
    Password       string   `yaml:"password"`
}

func (s *Service) setDefaults() {
    if s.IPv6Type == "" {
        s.IPv6Type = "disabled"
    }
    if s.IPv4Type == "" {
        s.IPv4Type = "disabled"
    }
}

// fileConfig is just a wrapper to match the top‑level "services:" key.
type fileConfig struct {
    Services []Service `yaml:"services"`
}


func SystemConfigDir() string {
    switch runtime.GOOS {
    case "freebsd":
        // FreeBSD installs into /usr/local/etc by default
        return "/usr/local/etc"
    case "darwin":
        // macOS usually uses /usr/local/etc for Homebrew‐style installs,
        // or /etc if you really want the root‐level one.
        return "/usr/local/etc"
    default:
        // most Linux/Unix distributions use /etc
        return "/etc"
    }
}

func getFileInterfaces(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ifaces []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			ifaces = append(ifaces, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ifaces, nil
}

func getIfAltnames(iface string) ([]string, error) {
    link, err := netlink.LinkByName(iface)
    if err != nil {
		return []string{}, fmt.Errorf("can't get iface: %w", err)
    }

	return link.Attrs().AltNames, nil
}

func isIPv6(ipNet *net.IPNet) (bool) {
	return !(ipNet.IP.To4() != nil || ipNet.IP.To16() == nil || ipNet.IP.IsLoopback())
}

func isIPv4(ipNet *net.IPNet) (bool) {
	return !(ipNet.IP.To4() == nil || ipNet.IP.IsLoopback())
}

func getIPaddresses(ifaceList []string,validateFunc func(*net.IPNet) bool) ([]string, error) {
	var ipAddresses []string
	log.Printf("mainifaces: %v", ifaceList)

	ifaceSet := make(map[string]bool)
	for _, name := range ifaceList {
		ifaceSet[name] = true
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		ifaceNames := []string{iface.Name}
		ifaceFound := false

		altNames, err := getIfAltnames(iface.Name)
		if err != nil {
			log.Printf("can't get altnames: %v", err)
		}

		ifaceNames = append(ifaceNames, altNames...)

		for _, ifaceName := range ifaceNames {
			if ifaceSet[ifaceName] {
				ifaceFound = true
				break
			}
		}
		if !ifaceFound {
			if debug {
				log.Printf("iface skipped: %v", iface.Name)
			}
			continue
		}


		addrs, err := iface.Addrs()
		if debug {
			log.Printf("iface new addr: %v", addrs)
		}
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || !validateFunc(ipNet) {
				continue
			}
			ipAddresses = append(ipAddresses, ipNet.IP.String())
		}
	}

	return ipAddresses, nil
}

func loadConfig(configLocation *string) []Service {
    var allServices []Service

    entries, err := os.ReadDir(*configLocation)
    if err != nil {
        fmt.Fprintf(os.Stderr, "read config dir: %v\n", err)
        os.Exit(1)
    }

    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := entry.Name()
        ext := filepath.Ext(name)
        if ext != ".yaml" && ext != ".yml" {
            continue
        }

        path := filepath.Join(*configLocation, name)
        data, err := os.ReadFile(path)
        if err != nil {
            fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
            continue
        }

        var fc fileConfig
        if err := yaml.Unmarshal(data, &fc); err != nil {
            fmt.Fprintf(os.Stderr, "unmarshal %s: %v\n", path, err)
            continue
        }

		for i := range fc.Services {
			fc.Services[i].setDefaults()
		}

        allServices = append(allServices, fc.Services...)
    }

	return allServices
}

func setRecords(ctx context.Context, configs []Service) {
	var err error

	for _, config := range configs {
		pv := dnsSet.Providers[config.Type]
		pv.SetAuth(dnsSet.Auth{
			ApiKey: config.APIKey,
			ApiSecretKey: config.APISecretKey,
			Username: config.Username,
			Password: config.Password,
		})
		var ips6, ips4 []string

		if config.IPv6Type == "interfaces" {
			ips6, err = getIPaddresses(config.IPv6Interfaces, isIPv6)
			if err != nil {
				log.Printf("error %v", err)
			}

		}

		if config.IPv4Type == "interfaces" {
			ips4, err = getIPaddresses(config.IPv4Interfaces, isIPv4)
			if err != nil {
				log.Printf("error %v", err)
			}
		}


		var records []dnsSet.Record
		for _, host := range config.Hostnames {
			for _, ip := range ips6 {
				record := dnsSet.Record{
					Domain: host,
					Content: ip,
					Type: "AAAA",
					TTL: config.TTL,
				}
				record.SetDefaults()
				records = append(records, record)
			}

			for _, ip := range ips4 {
				record := dnsSet.Record{
					Domain: host,
					Content: ip,
					Type: "A",
					TTL: config.TTL,
				}
				record.SetDefaults()
				records = append(records, record)
			}
		}

		pv.SetDns(ctx, records)
	}

}

func Run(ctx context.Context, configLocation *string, logger *log.Logger) {
	select {
	case <-ctx.Done():
		fmt.Println("Task cancelled")
		return
	default:
		for {
			configs := loadConfig(configLocation)
			setRecords(ctx, configs)

			time.Sleep(20 * time.Second)
		}
	}
}

