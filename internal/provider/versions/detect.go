package versions

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Source records how the API version used by the provider was chosen.
type Source int

const (
	// SourceLatest means neither a pin nor detection applied, so the latest
	// compiled-in version is used.
	SourceLatest Source = iota
	// SourceDetected means the version was detected from the server given by
	// BLASTSHIELD_HOST and BLASTSHIELD_TOKEN.
	SourceDetected
	// SourcePinned means the version was pinned with BLASTSHIELD_API_VERSION.
	SourcePinned
)

// Selection is the API version the provider serves. It is chosen at startup,
// because Terraform requests the resource schemas before it sends the provider
// configuration.
type Selection struct {
	Provider VersionedProvider
	// Version is the compiled-in API version in use.
	Version string
	Source  Source
	// Host is the server the version was detected from (SourceDetected only).
	Host string
	// Err is set when BLASTSHIELD_API_VERSION is not a supported version. The
	// latest version is used, and the error is reported when the provider is
	// configured.
	Err error
}

// Detect selects the API version from the environment: BLASTSHIELD_API_VERSION
// pins it, otherwise it is detected from the server given by BLASTSHIELD_HOST and
// BLASTSHIELD_TOKEN, otherwise the latest compiled-in version is used.
func Detect() Selection {
	if pin := os.Getenv("BLASTSHIELD_API_VERSION"); pin != "" {
		vp, ver, err := SelectVersion(pin)
		if err == nil {
			log.Printf("[INFO] BLASTSHIELD_API_VERSION=%s, using provider version: %s", pin, ver)
			return Selection{Provider: vp, Version: ver, Source: SourcePinned}
		}
		sel := latest()
		sel.Err = fmt.Errorf("BLASTSHIELD_API_VERSION=%s: %w", pin, err)
		log.Printf("[ERROR] %v, using latest API version: %s", sel.Err, sel.Version)
		return sel
	}

	host := os.Getenv("BLASTSHIELD_HOST")
	token := os.Getenv("BLASTSHIELD_TOKEN")

	if host != "" && token != "" {
		serverVersion, err := FetchAPIVersion(host, token)
		if err != nil {
			sel := latest()
			log.Printf("[WARN] Could not fetch API version from %s: %v, using latest API version: %s", host, err, sel.Version)
			return sel
		}

		vp, ver, err := SelectVersion(serverVersion)
		if err != nil {
			log.Fatalf("[ERROR] API version %s not supported: %v", serverVersion, err)
		}
		log.Printf("[INFO] Server API version: %s, using provider version: %s", serverVersion, ver)
		return Selection{Provider: vp, Version: ver, Source: SourceDetected, Host: host}
	}

	// No credentials available (e.g., terraform validate) — use latest version
	sel := latest()
	log.Printf("[INFO] No BLASTSHIELD_HOST/TOKEN set, using latest API version: %s", sel.Version)
	return sel
}

func latest() Selection {
	vp, ver := LatestVersion()
	return Selection{Provider: vp, Version: ver, Source: SourceLatest}
}

// FetchAPIVersion returns the API version from the server's OpenAPI spec.
func FetchAPIVersion(host, token string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", host+"/openapi.json", nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching openapi.json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openapi.json returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var spec struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		return "", fmt.Errorf("parsing openapi.json: %w", err)
	}

	if spec.Info.Version == "" {
		return "", fmt.Errorf("openapi.json missing info.version")
	}

	return spec.Info.Version, nil
}
