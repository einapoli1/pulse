package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type HostStatus struct {
	Config    HostConfig
	Online    bool
	CPU       string // load average
	Memory    string // used/total
	Disk      string // used%
	Uptime    string
	LastCheck time.Time
	Error     string
}

func checkHost(hc HostConfig) HostStatus {
	status := HostStatus{
		Config:    hc,
		LastCheck: time.Now(),
	}

	client, err := sshConnect(hc)
	if err != nil {
		status.Online = false
		status.Error = err.Error()
		return status
	}
	defer client.Close()

	status.Online = true

	// Get load average
	if out, err := runCommand(client, "cat /proc/loadavg 2>/dev/null || sysctl -n vm.loadavg 2>/dev/null"); err == nil {
		parts := strings.Fields(strings.Trim(out, "{ }"))
		if len(parts) >= 3 {
			status.CPU = fmt.Sprintf("%s %s %s", parts[0], parts[1], parts[2])
		}
	}

	// Get memory (Linux: free, macOS: vm_stat + sysctl)
	if out, err := runCommand(client, `free -h 2>/dev/null | awk '/^Mem:/{print $3"/"$2}'`); err == nil && strings.TrimSpace(out) != "" {
		status.Memory = strings.TrimSpace(out)
	} else if out, err := runCommand(client, `echo $(( $(vm_stat 2>/dev/null | awk '/Pages active|Pages wired/{gsub(/\./,"",$NF);s+=$NF}END{print s}') * 4096 / 1048576 ))Mi/$(( $(sysctl -n hw.memsize 2>/dev/null) / 1048576 ))Mi`); err == nil && strings.TrimSpace(out) != "" && !strings.Contains(out, "error") {
		status.Memory = strings.TrimSpace(out)
	}

	// Get disk usage
	if out, err := runCommand(client, `df -h / 2>/dev/null | awk 'NR==2{print $5}'`); err == nil {
		status.Disk = strings.TrimSpace(out)
	}

	// Get uptime
	if out, err := runCommand(client, `uptime -p 2>/dev/null || uptime | sed 's/.*up //' | sed 's/,.*//'`); err == nil {
		u := strings.TrimSpace(out)
		// Clean up macOS uptime output
		if strings.Contains(u, "load") {
			parts := strings.SplitN(u, ",", 3)
			if len(parts) >= 1 {
				u = strings.TrimSpace(parts[0])
			}
		}
		status.Uptime = u
	}

	return status
}

func sshConnect(hc HostConfig) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	// Try SSH agent first (covers macOS Keychain keys)
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	// Try key file
	if hc.KeyFile != "" {
		key, err := os.ReadFile(expandHome(hc.KeyFile))
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	// Try default keys
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		key, err := os.ReadFile(fmt.Sprintf("%s/.ssh/%s", home, name))
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			continue
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Password auth
	if hc.Password != "" {
		authMethods = append(authMethods, ssh.Password(hc.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no auth methods available")
	}

	config := &ssh.ClientConfig{
		User:            hc.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", hc.Host, hc.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh: %w", err)
	}

	return ssh.NewClient(c, chans, reqs), nil
}

func runCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return home + path[1:]
	}
	return path
}
