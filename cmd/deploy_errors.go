package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DeployErrorClass represents a known failure class during deployment.
type DeployErrorClass string

const (
	ErrorClassDockerPortConflict DeployErrorClass = "docker_port_conflict"
	ErrorClassDockerBindFailed   DeployErrorClass = "docker_bind_failed"
	ErrorClassAzureAuth          DeployErrorClass = "azure_auth"
	ErrorClassAzureMySQLStopped  DeployErrorClass = "azure_mysql_stopped"
	ErrorClassAzureKeyVault      DeployErrorClass = "azure_keyvault_softdelete"
	ErrorClassUnknown            DeployErrorClass = "unknown"
)

// DeployError represents a classified deployment error with recovery context.
type DeployError struct {
	Class       DeployErrorClass
	OriginalErr error
	Port        string // For port conflict errors
	Container   string // For port conflict errors
	ComposeFile string // For port conflict errors
	Message     string // Human-readable classification
}

// classifyDockerComposeError inspects a docker compose error and returns
// a classified error with recovery context. This covers:
// - "port is already allocated"
// - "Bind for 0.0.0.0:PORT"
// - "ports are not available" / "Ports are not available"
// - "address already in use"
// - "failed programming external connectivity"
func classifyDockerComposeError(err error) *DeployError {
	if err == nil {
		return nil
	}

	errStr := err.Error()
	errStrLower := strings.ToLower(errStr)

	// Port conflict patterns (case-insensitive)
	portConflictPatterns := []string{
		"port is already allocated",
		"bind for",
		"ports are not available",
		"address already in use",
		"failed programming external connectivity",
	}

	isPortConflict := false
	for _, pattern := range portConflictPatterns {
		if strings.Contains(errStrLower, pattern) {
			isPortConflict = true
			break
		}
	}

	if !isPortConflict {
		return &DeployError{
			Class:       ErrorClassUnknown,
			OriginalErr: err,
			Message:     "Docker Compose failed",
		}
	}

	// Extract port number from various error formats
	port := extractPortFromError(errStr)

	result := &DeployError{
		Class:       ErrorClassDockerPortConflict,
		OriginalErr: err,
		Port:        port,
		Message:     "Docker port conflict detected",
	}

	// Try to identify owning container
	if port != "" {
		container, composeFile := findPortOwner(port)
		result.Container = container
		result.ComposeFile = composeFile
	}

	return result
}

// extractPortFromError extracts the port number from various Docker error formats:
// - "Bind for 0.0.0.0:8080: failed: port is already allocated"
// - "Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:8080"
// - "bind: address already in use (listening on [::]:8080)"
// - "failed programming external connectivity on endpoint devlake (8080/tcp)"
func extractPortFromError(errStr string) string {
	// Pattern 1: "Bind for 0.0.0.0:PORT" (case-insensitive using regexp)
	re := regexp.MustCompile(`(?i)bind for 0\.0\.0\.0:(\d+)`)
	if matches := re.FindStringSubmatch(errStr); len(matches) > 1 {
		port := matches[1]
		if isValidPort(port) {
			return port
		}
	}

	// Pattern 2: "exposing port TCP 0.0.0.0:PORT"
	if idx := strings.Index(errStr, "0.0.0.0:"); idx != -1 {
		rest := errStr[idx+len("0.0.0.0:"):]
		if end := strings.IndexAny(rest, " ->\n"); end > 0 {
			port := rest[:end]
			if isValidPort(port) {
				return port
			}
		}
	}

	// Pattern 3: "listening on [::]:PORT" or "[::]PORT"
	if idx := strings.Index(errStr, "[::]"); idx != -1 {
		rest := errStr[idx+len("[::]"):]
		// Skip potential colon separator
		if strings.HasPrefix(rest, ":") {
			rest = rest[1:]
		}
		if end := strings.IndexAny(rest, " )\n"); end > 0 {
			port := rest[:end]
			if isValidPort(port) {
				return port
			}
		}
		// If no delimiter found, but there are digits, use them
		if len(rest) > 0 {
			for i, ch := range rest {
				if ch < '0' || ch > '9' {
					if i > 0 {
						port := rest[:i]
						if isValidPort(port) {
							return port
						}
					}
					break
				}
			}
		}
	}

	// Pattern 4: "(PORT/tcp)" or "(PORT/udp)" in endpoint errors
	if idx := strings.Index(errStr, "("); idx != -1 {
		rest := errStr[idx+1:]
		if end := strings.Index(rest, "/tcp)"); end > 0 {
			port := strings.TrimSpace(rest[:end])
			if isValidPort(port) {
				return port
			}
		}
		if end := strings.Index(rest, "/udp)"); end > 0 {
			port := strings.TrimSpace(rest[:end])
			if isValidPort(port) {
				return port
			}
		}
	}

	// Pattern 5: Generic port number extraction (last resort)
	// Look for sequences like ":8080" or " 8080 " in the error
	for _, candidate := range strings.Fields(errStr) {
		// Try splitting by colons
		if strings.Contains(candidate, ":") {
			parts := strings.Split(candidate, ":")
			for _, part := range parts {
				part = strings.Trim(part, "(),[]")
				if isValidPort(part) {
					return part
				}
			}
		}
		// Try the field itself (for cases like "[::] 3002")
		cleaned := strings.Trim(candidate, "(),[]")
		if isValidPort(cleaned) {
			return cleaned
		}
	}

	return ""
}

// isValidPort checks if a string looks like a valid port number (all digits, 1-65535).
func isValidPort(s string) bool {
	if len(s) < 1 || len(s) > 5 {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	// Parse to int and validate range 1-65535
	port := 0
	for _, ch := range s {
		port = port*10 + int(ch-'0')
	}
	return port >= 1 && port <= 65535
}

// findPortOwner queries Docker to find which container is using the specified port.
// Returns (containerName, composeFilePath).
func findPortOwner(port string) (string, string) {
	out, err := exec.Command(
		"docker",
		"ps",
		"--filter", "publish="+port,
		"--format", "{{.Names}}\t{{.Label \"com.docker.compose.project.config_files\"}}\t{{.Label \"com.docker.compose.project.working_dir\"}}",
	).Output()

	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return "", ""
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	parts := strings.SplitN(lines[0], "\t", 3)

	containerName := parts[0]
	configFiles := ""
	workDir := ""

	if len(parts) >= 2 {
		configFiles = strings.TrimSpace(parts[1])
	}
	if len(parts) == 3 {
		workDir = strings.TrimSpace(parts[2])
	}

	// Prefer the exact compose file path Docker recorded
	if configFiles != "" {
		configFile := strings.Split(configFiles, ";")[0]
		configFile = strings.TrimSpace(configFile)
		if configFile != "" {
			if _, statErr := os.Stat(configFile); statErr == nil {
				return containerName, configFile
			}
		}
	}

	// Fallback: assume docker-compose.yml under working_dir
	if workDir != "" {
		composePath := filepath.Join(workDir, "docker-compose.yml")
		if _, statErr := os.Stat(composePath); statErr == nil {
			return containerName, composePath
		}
	}

	return containerName, ""
}

// printDockerPortConflictError prints a user-friendly error message for port conflicts
// with actionable remediation steps.
// If customHeader is provided, it replaces the default "Port conflict detected" header.
// If nextSteps is provided, it replaces the default "Then re-run: gh devlake deploy local" text.
func printDockerPortConflictError(de *DeployError, customHeader string, nextSteps string) {
	// Print header
	if customHeader != "" {
		fmt.Println(customHeader)
	} else {
		if de.Port != "" {
			fmt.Printf("\n❌ Port conflict detected: port %s is already in use.\n", de.Port)
		} else {
			fmt.Println("\n❌ Port conflict detected: a required port is already in use.")
		}
	}

	// Print container info and stop commands
	if de.Container != "" {
		fmt.Printf("   Container holding the port: %s\n", de.Container)

		if de.ComposeFile != "" {
			fmt.Println("   Stop it with:")
			fmt.Printf("   docker compose -f \"%s\" down\n", de.ComposeFile)
		} else {
			fmt.Println("   Stop it with:")
			fmt.Printf("   docker stop %s\n", de.Container)
		}
	} else if de.Port != "" {
		fmt.Println("   Find what's using it:")
		fmt.Println("   docker ps --format \"table {{.Names}}\\t{{.Ports}}\"")
	}

	// Print next steps
	if nextSteps != "" {
		fmt.Println(nextSteps)
	} else {
		fmt.Println("   Then re-run:")
		fmt.Println("   gh devlake deploy local")
	}

	fmt.Println("\n💡 To clean up partial artifacts:")
	fmt.Println("   gh devlake cleanup --local --force")
}
