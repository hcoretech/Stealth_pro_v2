package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	torControl = "127.0.0.1:9051"
	torSocks   = "127.0.0.1:9050"
	adapter    = "Wi-Fi" // Change to 'Ethernet' if needed
)

var countryList = []string{"{us}", "{ca}", "{de}", "{ch}", "{gb}", "{fr}", "{jp}", "{nl}", "{se}", "{no}"}

func main() {
	fmt.Println("[*] INITIALIZING TOTAL STEALTH PROTOCOL...")

	// Handle cleanup (restore real IP/Proxy) on exit (Ctrl+C)
	setupExitHandler()

	// 1. Initial Setup: Kill switches and System Proxy
	applySystemKillswitches()
	setSystemProxy(true)
	spoofMAC(adapter)
	exec.Command("ipconfig", "/flushdns").Run()

	rand.Seed(time.Now().UnixNano())

	for {
		selectedCountry := countryList[rand.Intn(len(countryList))]
		fmt.Printf("\n[*] SHIFTING IDENTITY TO: %s\n", strings.ToUpper(selectedCountry))

		setTorCountry(selectedCountry)
		clearLocationCache()
		printCurrentIP()

		fmt.Println("[+] Stealth stable. iplocation.net will now show this location.")
		fmt.Println("[*] Next jump in 3 minutes...")
		time.Sleep(3 * time.Minute)
	}
}

func setupExitHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n[!] SHUTDOWN DETECTED: Restoring real location and cleaning up...")
		setSystemProxy(false)
		os.Exit(0)
	}()
}

func applySystemKillswitches() {
	fmt.Println("[*] Hard-disabling Windows Geolocation Services...")
	commands := []string{
		`Set-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location' -Name 'Value' -Value 'Deny'`,
		`if (!(Test-Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\LocationAndSensors')) { New-Item -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\LocationAndSensors' -Force }`,
		`Set-ItemProperty -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\LocationAndSensors' -Name 'DisableLocation' -Value 1 -Type DWord`,
		`Stop-Service -Name 'lfsvc' -Force -ErrorAction SilentlyContinue`,
		`Set-Service -Name 'lfsvc' -StartupType Disabled`,
	}
	for _, cmd := range commands {
		exec.Command("powershell", "-Command", cmd).Run()
	}
}

func setSystemProxy(enabled bool) {
	proxyServer := "socks=127.0.0.1:9050"
	var state int
	if enabled {
		state = 1
		fmt.Println("[*] ENABLING SYSTEM-WIDE TOR TUNNEL...")
	} else {
		state = 0
		fmt.Println("[!] DISABLING PROXY. REAL LOCATION EXPOSED.")
	}

	commands := []string{
		fmt.Sprintf(`Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name 'ProxyEnable' -Value %d`, state),
		fmt.Sprintf(`Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name 'ProxyServer' -Value '%s'`, proxyServer),
		`Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name 'ProxyOverride' -Value '<local>'`,
	}
	for _, cmd := range commands {
		exec.Command("powershell", "-Command", cmd).Run()
	}
}

func clearLocationCache() {
	exec.Command("powershell", "-Command", `Remove-Item -Path 'C:\ProgramData\Microsoft\Windows\Geolocation\*' -Recurse -Force -ErrorAction SilentlyContinue`).Run()
}

func setTorCountry(countryCode string) {
	conn, err := net.Dial("tcp", torControl)
	if err != nil {
		log.Printf("[-] Tor ControlPort error: %v", err)
		return
	}
	defer conn.Close()
	fmt.Fprintf(conn, "AUTHENTICATE \"\"\r\n")
	fmt.Fprintf(conn, "SETCONF ExitNodes=%s StrictNodes=1\r\n", countryCode)
	fmt.Fprintf(conn, "SIGNAL NEWNYM\r\n")
	time.Sleep(5 * time.Second)
}

func spoofMAC(adapterName string) {
	newMAC := fmt.Sprintf("02%02X%02X%02X%02X%02X", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
	fmt.Printf("[*] Randomizing Hardware MAC: %s\n", newMAC)
	psCmd := fmt.Sprintf("Set-NetAdapterAdvancedProperty -Name '%s' -RegistryKeyword 'NetworkAddress' -RegistryValue '%s'", adapterName, newMAC)
	exec.Command("powershell", "-Command", psCmd).Run()
	exec.Command("powershell", "-Command", fmt.Sprintf("Restart-NetAdapter -Name '%s'", adapterName)).Run()
}

func printCurrentIP() {
	proxyURL, _ := url.Parse("socks5://" + torSocks)
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   15 * time.Second,
	}
	resp, err := client.Get("http://checkip.amazonaws.com")
	if err != nil {
		log.Printf("[-] IP Verification Failed: %v", err)
		return
	}
	defer resp.Body.Close()
	ip, _ := io.ReadAll(resp.Body)
	fmt.Printf("[+] Verified Public IP: %s", string(ip))
}
