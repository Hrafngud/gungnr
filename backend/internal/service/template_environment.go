package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"

	"go-notes/internal/infra/contract"
	"go-notes/internal/jobs"
	"go-notes/internal/validate"
)

// Serialize selection and writing so simultaneous clones reserve distinct
// subnets even before either Compose stack creates its Docker network.
var templateEnvironmentMu sync.Mutex

type templateEnvironmentClient interface {
	DockerNetworkSubnets(context.Context, string) (contract.Result, error)
	ProjectFileWriteAtomic(context.Context, string, contract.ProjectFileWriteAtomicPayload) (contract.Result, error)
}

func prepareTemplateEnvironment(ctx context.Context, bridge infraBridgeClient, logger jobs.Logger, owner, repo, projectDir, name string, proxyPort, dbPort int) error {
	if !strings.EqualFold(owner, "Hrafngud") || !strings.EqualFold(repo, "ent-go-vue") {
		return nil
	}
	client, ok := bridge.(templateEnvironmentClient)
	if !ok {
		return fmt.Errorf("template environment bridge client unavailable")
	}
	if err := PrepareEntGoVueEnvironment(ctx, client, projectDir, name, proxyPort, dbPort); err != nil {
		return fmt.Errorf("prepare ent-go-vue .env: %w", err)
	}
	logger.Log("ent-go-vue environment ready: project database credentials, JWT secret, HTTP port, and isolated network configured")
	return nil
}

// PrepareEntGoVueEnvironment materializes a fresh clone's example through the
// host file bridge. Existing operator-managed environments are never replaced.
func PrepareEntGoVueEnvironment(ctx context.Context, client templateEnvironmentClient, projectDir, name string, proxyPort, dbPort int) error {
	templateEnvironmentMu.Lock()
	defer templateEnvironmentMu.Unlock()
	if err := validate.ProjectName(name); err != nil {
		return err
	}
	if err := validate.Port(proxyPort); err != nil {
		return err
	}
	if err := validate.Port(dbPort); err != nil {
		return err
	}
	envPath := filepath.Join(projectDir, ".env")
	if info, err := os.Lstat(envPath); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf(".env must be a regular file")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect .env: %w", err)
	}
	examplePath := filepath.Join(projectDir, ".env.example")
	info, err := os.Lstat(examplePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > defaultProjectEnvMaxBytes {
		return fmt.Errorf("template requires a regular .env.example of at most 1 MiB")
	}
	values, err := godotenv.Read(examplePath)
	if err != nil {
		return fmt.Errorf("read .env.example: %w", err)
	}
	for _, key := range []string{"POSTGRES_DB", "POSTGRES_USER", "POSTGRES_PASSWORD", "DATABASE_URL", "JWT_SECRET", "APP_SUBNET", "APP_IP_RANGE", "NGINX_INTERNAL_IP"} {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("template .env.example is missing %s", key)
		}
	}
	password, err := randomTemplateSecret()
	if err != nil {
		return err
	}
	jwt, err := randomTemplateSecret()
	if err != nil {
		return err
	}
	dbName := strings.ReplaceAll(name, "-", "_")
	if len(dbName) > 63 {
		dbName = dbName[:63]
	}
	values["POSTGRES_DB"] = dbName
	values["POSTGRES_USER"] = dbName
	values["POSTGRES_PASSWORD"] = password
	values["DATABASE_URL"] = (&url.URL{Scheme: "postgres", User: url.UserPassword(dbName, password), Host: "postgres:5432", Path: "/" + dbName, RawQuery: "sslmode=disable"}).String()
	values["JWT_SECRET"] = jwt
	values["ROOT_EMAIL"], values["ROOT_PASSWORD"] = "", ""
	values["HTTP_PORT"] = strconv.Itoa(proxyPort)
	values["POSTGRES_PORT"] = strconv.Itoa(dbPort)
	if client == nil {
		return fmt.Errorf("template environment bridge client unavailable")
	}
	result, err := client.DockerNetworkSubnets(ctx, "")
	if err != nil {
		return fmt.Errorf("read Docker network subnets: %w", err)
	}
	lines, err := decodeBridgeLinesPayload(result)
	if err != nil {
		return err
	}
	// Include not-yet-started sibling clones whose .env already reserves a subnet.
	entries, err := os.ReadDir(filepath.Dir(projectDir))
	if err != nil {
		return fmt.Errorf("read template subnet reservations: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		siblingEnv := filepath.Join(filepath.Dir(projectDir), entry.Name(), ".env")
		if info, err := os.Lstat(siblingEnv); err == nil && info.Mode().IsRegular() && info.Size() <= defaultProjectEnvMaxBytes {
			other, err := godotenv.Read(siblingEnv)
			if err != nil {
				return fmt.Errorf("read subnet reservation for %s", entry.Name())
			}
			if subnet := other["APP_SUBNET"]; subnet != "" {
				lines = append(lines, subnet)
			}
		}
	}
	subnet, err := selectTemplateSubnet(lines)
	if err != nil {
		return err
	}
	values["APP_SUBNET"] = subnet.String()
	// Automatic addresses occupy the upper /25; Nginx stays at .2 below it.
	addr := subnet.Addr().As4()
	addr[3] = 128
	values["APP_IP_RANGE"] = netip.PrefixFrom(netip.AddrFrom4(addr), 25).String()
	values["NGINX_INTERNAL_IP"] = subnet.Addr().Next().Next().String()
	content, err := godotenv.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode template .env: %w", err)
	}
	_, err = client.ProjectFileWriteAtomic(ctx, "", contract.ProjectFileWriteAtomicPayload{
		BasePath: projectDir, Path: envPath, Content: content + "\n", Mode: 0o600, CreateParents: true,
	})
	if err != nil {
		return fmt.Errorf("write template .env through host bridge: %w", err)
	}
	return nil
}

func randomTemplateSecret() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate template secret: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func selectTemplateSubnet(occupied []string) (netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(occupied))
	for _, raw := range occupied {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("invalid Docker/project subnet reservation")
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	for octet := 90; octet <= 255; octet++ {
		candidate := netip.PrefixFrom(netip.AddrFrom4([4]byte{172, 30, byte(octet), 0}), 24)
		available := true
		for _, prefix := range prefixes {
			if prefix.Overlaps(candidate) {
				available = false
				break
			}
		}
		if available {
			return candidate, nil
		}
	}
	return netip.Prefix{}, fmt.Errorf("no free ent-go-vue subnet in 172.30.90.0–172.30.255.0")
}
