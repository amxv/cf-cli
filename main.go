package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var profile string
var apiToken string
var bootstrapToken string
var zoneID string
var accountID string
var domain string
var apiTokenKeychainService string
var bootstrapTokenKeychainService string
var zoneIDKeychainService string
var accountIDKeychainService string
var domainKeychainService string
var storeMintedToken bool
var expiresOn string
var proxied bool
var ttl int16 = 3600
var upsert bool

type dnsRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type zone struct {
	ID string `json:"id"`
}

type tokenPermissionGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type tokenPolicy struct {
	Effect           string                 `json:"effect"`
	PermissionGroups []tokenPermissionGroup `json:"permission_groups"`
	Resources        map[string]string      `json:"resources"`
}

type tokenCreateResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Value  string `json:"value"`
}

type apiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiEnvelope[T any] struct {
	Success  bool         `json:"success"`
	Errors   []apiMessage `json:"errors"`
	Messages []apiMessage `json:"messages"`
	Result   T            `json:"result"`
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "cf",
		Short: "Cloudflare DNS helper for fast agent-driven DNS edits",
	}

	rootCmd.PersistentFlags().StringVar(&profile, "profile", defaultProfile(), "Cloudflare profile name used for keychain lookups")
	rootCmd.PersistentFlags().StringVar(&apiTokenKeychainService, "api-token-keychain-service", "", "Override the macOS keychain service name for the active DNS API token")
	rootCmd.PersistentFlags().StringVar(&bootstrapTokenKeychainService, "bootstrap-token-keychain-service", "", "Override the macOS keychain service name for the bootstrap token")
	rootCmd.PersistentFlags().StringVar(&zoneIDKeychainService, "zone-id-keychain-service", "", "Override the macOS keychain service name for the zone ID")
	rootCmd.PersistentFlags().StringVar(&accountIDKeychainService, "account-id-keychain-service", "", "Override the macOS keychain service name for the account ID")
	rootCmd.PersistentFlags().StringVar(&domainKeychainService, "domain-keychain-service", "", "Override the macOS keychain service name for the default domain")

	updateCmd := &cobra.Command{
		Use:   "update:dns [domain] [type] [key] [value] [comment (optional)]",
		Short: "Update or insert a DNS record for a domain",
		Args:  cobra.MinimumNArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			recordType := args[1]
			key := args[2]
			value := args[3]
			comment := ""

			if len(args) > 4 {
				comment = args[4]
			}
			if key == "" || value == "" {
				return fmt.Errorf("key and value must be provided")
			}

			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}

			return updateDNSRecord(resolvedToken, domain, recordType, key, value, comment)
		},
	}
	updateCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	updateCmd.Flags().BoolVar(&proxied, "proxied", true, "Whether to enable Cloudflare proxying")
	updateCmd.Flags().Int16Var(&ttl, "ttl", 3600, "Time to live for the DNS record in seconds")
	updateCmd.Flags().BoolVar(&upsert, "upsert", false, "Create the DNS record if it does not exist")
	updateCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")

	setCmd := &cobra.Command{
		Use:   "set [type] [key] [value] [comment (optional)]",
		Short: "Set a DNS record using the default domain for the current profile",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDomain, err := resolveDomain()
			if err != nil {
				return err
			}
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}

			comment := ""
			if len(args) > 3 {
				comment = args[3]
			}

			return updateDNSRecord(resolvedToken, resolvedDomain, args[0], args[1], args[2], comment)
		},
	}
	setCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	setCmd.Flags().BoolVar(&proxied, "proxied", true, "Whether to enable Cloudflare proxying")
	setCmd.Flags().Int16Var(&ttl, "ttl", 3600, "Time to live for the DNS record in seconds")
	setCmd.Flags().BoolVar(&upsert, "upsert", true, "Create the DNS record if it does not exist")
	setCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")

	aCmd := &cobra.Command{
		Use:   "a [key] [ipv4] [comment (optional)]",
		Short: "Shortcut for setting an A record on the default domain",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDomain, err := resolveDomain()
			if err != nil {
				return err
			}
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}

			comment := ""
			if len(args) > 2 {
				comment = args[2]
			}

			return updateDNSRecord(resolvedToken, resolvedDomain, "A", args[0], args[1], comment)
		},
	}
	aCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	aCmd.Flags().BoolVar(&proxied, "proxied", true, "Whether to enable Cloudflare proxying")
	aCmd.Flags().Int16Var(&ttl, "ttl", 3600, "Time to live for the DNS record in seconds")
	aCmd.Flags().BoolVar(&upsert, "upsert", true, "Create the DNS record if it does not exist")
	aCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Show which profile values resolve from environment or keychain",
		RunE: func(cmd *cobra.Command, args []string) error {
			reportResolved("profile", profile)
			reportSecretResolution("domain", resolveDomain)
			reportSecretResolution("api token", resolveAPIToken)
			reportSecretResolution("bootstrap token", resolveBootstrapToken)
			reportSecretResolution("zone id", resolveZoneID)
			reportSecretResolution("account id", resolveAccountID)
			return nil
		},
	}

	mintCmd := &cobra.Command{
		Use:   "mint:dns-token [name]",
		Short: "Mint a fresh zone-scoped DNS token via the Cloudflare API",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tokenName := fmt.Sprintf("%s cloudflare dns cli %s", profile, time.Now().UTC().Format("20060102-150405"))
			if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
				tokenName = strings.TrimSpace(args[0])
			}

			bootstrap, err := resolveBootstrapToken()
			if err != nil {
				return err
			}
			resolvedZoneID, err := resolveZoneID()
			if err != nil {
				return err
			}
			if _, err := resolveAccountID(); err != nil {
				return err
			}

			token, err := mintDNSToken(bootstrap, resolvedZoneID, tokenName, expiresOn)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Minted token %q (%s)\n", token.Name, token.ID)
			if storeMintedToken {
				if err := writeSecretToKeychain(defaultAPIServiceName(), token.Value); err != nil {
					return fmt.Errorf("minted token, but failed to store it in keychain: %w", err)
				}
				fmt.Printf("✅ Stored active API token in keychain service %q\n", defaultAPIServiceName())
				fmt.Printf("Use it with:\n./cf update:dns <domain> <type> <key> <value> [comment]\n")
				return nil
			}

			fmt.Printf("Use it immediately with:\nCF_API_TOKEN=%s ./cf update:dns <domain> <type> <key> <value> [comment]\n", token.Value)
			return nil
		},
	}
	mintCmd.Flags().StringVar(&bootstrapToken, "bootstrap-token", "", "Cloudflare bootstrap token with API Tokens Write")
	mintCmd.Flags().StringVar(&zoneID, "zone-id", "", "Cloudflare zone ID")
	mintCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	mintCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")
	mintCmd.Flags().StringVar(&expiresOn, "expires-on", "", "Optional RFC3339 expiry time for the minted token")
	mintCmd.Flags().BoolVar(&storeMintedToken, "store", true, "Store the minted API token in the macOS keychain")

	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(aCmd)
	rootCmd.AddCommand(mintCmd)
	rootCmd.AddCommand(doctorCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}
}

func updateDNSRecord(apiToken, domain, recordType, key, value, comment string) error {
	client := &http.Client{}

	zoneResp, err := getJSON[[]zone](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", url.QueryEscape(domain)), apiToken)
	if err != nil {
		return err
	}
	if len(zoneResp.Result) == 0 {
		return fmt.Errorf("zone not found for %s", domain)
	}

	recordName := normalizeRecordName(domain, key)
	recordDisplayName := key
	if recordDisplayName == "" {
		recordDisplayName = "@"
	}

	recordResp, err := getJSON[[]dnsRecord](client, fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=%s&name=%s",
		zoneResp.Result[0].ID,
		url.QueryEscape(strings.ToUpper(recordType)),
		url.QueryEscape(recordName),
	), apiToken)
	if err != nil {
		return err
	}

	if len(recordResp.Result) == 0 {
		if !upsert {
			return fmt.Errorf("the %s record was not found for %s", strings.ToUpper(recordType), recordName)
		}
		if err := insertRecord(apiToken, recordType, recordName, value, comment, zoneResp.Result[0].ID, client); err != nil {
			return err
		}
	} else {
		if err := updateRecord(apiToken, recordType, recordName, value, comment, zoneResp.Result[0].ID, recordResp.Result[0].ID, client); err != nil {
			return err
		}
	}

	fmt.Printf("✅ %s %s -> %s\n", strings.ToUpper(recordType), recordDisplayName, value)
	return nil
}

func mintDNSToken(bootstrap, resolvedZoneID, tokenName, tokenExpiry string) (*tokenCreateResult, error) {
	client := &http.Client{}

	payload := map[string]any{
		"name": tokenName,
		"policies": []tokenPolicy{
			{
				Effect: "allow",
				PermissionGroups: []tokenPermissionGroup{
					{ID: "c8fed203ed3043cba015a93ad1616f1f", Name: "Zone Read"},
					{ID: "4755a26eedb94da69e1066d98aa820be", Name: "DNS Write"},
				},
				Resources: map[string]string{
					fmt.Sprintf("com.cloudflare.api.account.zone.%s", resolvedZoneID): "*",
				},
			},
		},
	}
	if tokenExpiry != "" {
		payload["expires_on"] = tokenExpiry
	}

	req, err := newJSONRequest("POST", "https://api.cloudflare.com/client/v4/user/tokens", bootstrap, payload)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope apiEnvelope[tokenCreateResult]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Success {
		return nil, apiErrors(envelope.Errors)
	}
	if envelope.Result.Value == "" {
		return nil, errors.New("cloudflare did not return a token value")
	}

	return &envelope.Result, nil
}

func insertRecord(apiToken, recordType, recordName, value, comment, zoneID string, client *http.Client) error {
	payload := map[string]any{
		"type":    strings.ToUpper(recordType),
		"name":    recordName,
		"content": value,
		"ttl":     ttl,
		"proxied": proxied,
		"comment": comment,
	}

	req, err := newJSONRequest("POST", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID), apiToken, payload)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkAPIResponse(resp)
}

func updateRecord(apiToken, recordType, recordName, value, comment, zoneID, recordID string, client *http.Client) error {
	payload := map[string]any{
		"type":    strings.ToUpper(recordType),
		"name":    recordName,
		"content": value,
		"ttl":     ttl,
		"proxied": proxied,
		"comment": comment,
	}

	req, err := newJSONRequest("PUT", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID), apiToken, payload)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkAPIResponse(resp)
}

func newJSONRequest(method, requestURL, token string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(payloadBytes)
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	return req, nil
}

func getJSON[T any](client *http.Client, requestURL, apiToken string) (*apiEnvelope[T], error) {
	req, err := newJSONRequest("GET", requestURL, apiToken, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope apiEnvelope[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Success {
		return nil, apiErrors(envelope.Errors)
	}

	return &envelope, nil
}

func checkAPIResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var envelope apiEnvelope[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return fmt.Errorf("cloudflare API request failed with status %s", resp.Status)
		}
		return nil
	}
	if !envelope.Success {
		return apiErrors(envelope.Errors)
	}

	return nil
}

func apiErrors(errs []apiMessage) error {
	if len(errs) == 0 {
		return errors.New("cloudflare API request failed")
	}

	parts := make([]string, 0, len(errs))
	for _, apiErr := range errs {
		parts = append(parts, fmt.Sprintf("%d: %s", apiErr.Code, apiErr.Message))
	}

	return errors.New(strings.Join(parts, "; "))
}

func resolveAPIToken() (string, error) {
	if apiToken != "" {
		return apiToken, nil
	}
	if envToken := os.Getenv("CF_API_TOKEN"); envToken != "" {
		return envToken, nil
	}
	if envToken := os.Getenv("CLOUDFLARE_API_TOKEN"); envToken != "" {
		return envToken, nil
	}

	service := chooseService(apiTokenKeychainService, defaultAPIServiceName())
	token, err := readSecretFromKeychain(service)
	if err == nil && token != "" {
		return token, nil
	}

	return "", fmt.Errorf("cloudflare API token not provided; set CF_API_TOKEN/CLOUDFLARE_API_TOKEN or store it in keychain service %q", service)
}

func resolveBootstrapToken() (string, error) {
	if bootstrapToken != "" {
		return bootstrapToken, nil
	}
	if envToken := os.Getenv("CF_BOOTSTRAP_TOKEN"); envToken != "" {
		return envToken, nil
	}
	if envToken := os.Getenv("CLOUDFLARE_BOOTSTRAP_TOKEN"); envToken != "" {
		return envToken, nil
	}

	service := chooseService(bootstrapTokenKeychainService, defaultBootstrapServiceName())
	token, err := readSecretFromKeychain(service)
	if err == nil && token != "" {
		return token, nil
	}

	return "", fmt.Errorf("cloudflare bootstrap token not provided; set CF_BOOTSTRAP_TOKEN/CLOUDFLARE_BOOTSTRAP_TOKEN or store it in keychain service %q", service)
}

func resolveZoneID() (string, error) {
	if zoneID != "" {
		return zoneID, nil
	}
	if env := os.Getenv("CF_ZONE_ID"); env != "" {
		return env, nil
	}
	if env := os.Getenv("CLOUDFLARE_ZONE_ID"); env != "" {
		return env, nil
	}

	service := chooseService(zoneIDKeychainService, defaultZoneIDServiceName())
	value, err := readSecretFromKeychain(service)
	if err == nil && value != "" {
		return value, nil
	}

	return "", fmt.Errorf("cloudflare zone ID not provided; set CF_ZONE_ID/CLOUDFLARE_ZONE_ID or store it in keychain service %q", service)
}

func resolveAccountID() (string, error) {
	if accountID != "" {
		return accountID, nil
	}
	if env := os.Getenv("CF_ACCOUNT_ID"); env != "" {
		return env, nil
	}
	if env := os.Getenv("CLOUDFLARE_ACCOUNT_ID"); env != "" {
		return env, nil
	}

	service := chooseService(accountIDKeychainService, defaultAccountIDServiceName())
	value, err := readSecretFromKeychain(service)
	if err == nil && value != "" {
		return value, nil
	}

	return "", fmt.Errorf("cloudflare account ID not provided; set CF_ACCOUNT_ID/CLOUDFLARE_ACCOUNT_ID or store it in keychain service %q", service)
}

func resolveDomain() (string, error) {
	if domain != "" {
		return domain, nil
	}
	if env := os.Getenv("CF_DOMAIN"); env != "" {
		return env, nil
	}
	if env := os.Getenv("CLOUDFLARE_DOMAIN"); env != "" {
		return env, nil
	}

	service := chooseService(domainKeychainService, defaultDomainServiceName())
	value, err := readSecretFromKeychain(service)
	if err == nil && value != "" {
		return value, nil
	}

	return "", fmt.Errorf("default domain not provided; set CF_DOMAIN/CLOUDFLARE_DOMAIN or store it in keychain service %q", service)
}

func normalizeRecordName(domain, key string) string {
	if key == "" || key == "@" {
		return domain
	}
	if keyHasDomainSuffix(key, domain) {
		return key
	}
	return key + "." + domain
}

func keyHasDomainSuffix(key, domain string) bool {
	if len(key) < len(domain) {
		return false
	}
	return strings.EqualFold(key[len(key)-len(domain):], domain)
}

func defaultProfile() string {
	if value := os.Getenv("CF_PROFILE"); value != "" {
		return value
	}
	return "ama"
}

func defaultAPIServiceName() string {
	return fmt.Sprintf("%s cloudflare api token", profile)
}

func defaultBootstrapServiceName() string {
	return fmt.Sprintf("%s cloudflare bootstrap token", profile)
}

func defaultZoneIDServiceName() string {
	return fmt.Sprintf("%s cloudflare zone id", profile)
}

func defaultAccountIDServiceName() string {
	return fmt.Sprintf("%s cloudflare account id", profile)
}

func defaultDomainServiceName() string {
	return fmt.Sprintf("%s cloudflare domain", profile)
}

func chooseService(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	return fallback
}

func readSecretFromKeychain(service string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("keychain lookup is only supported on macOS")
	}

	cmd := exec.Command("security", "find-generic-password", "-s", service, "-w", os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db"))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func writeSecretToKeychain(service, value string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("keychain storage is only supported on macOS")
	}

	cmd := exec.Command(
		"security", "add-generic-password",
		"-U",
		"-a", "cloudflare-dns-cli",
		"-s", service,
		"-w", value,
		os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

func reportResolved(label, value string) {
	fmt.Printf("✅ %s: %s\n", label, value)
}

func reportSecretResolution(label string, resolver func() (string, error)) {
	value, err := resolver()
	if err != nil {
		fmt.Printf("❌ %s: %s\n", label, err)
		return
	}

	redacted := value
	if strings.Contains(label, "token") && len(value) > 10 {
		redacted = value[:8] + "..." + value[len(value)-4:]
	}
	fmt.Printf("✅ %s: %s\n", label, redacted)
}
