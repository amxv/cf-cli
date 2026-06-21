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
	"sort"
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
var recordValue string
var deleteAll bool
var priority uint16
var tokenOwner string
var tokenScope string
var tokenResourceID string
var tokenPreset string
var tokenStoreService string
var activateMintedToken bool
var tokenPermissions []string

type dnsRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
	Comment string `json:"comment"`
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
	Resources        map[string]any         `json:"resources"`
}

type tokenCreateResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Value  string `json:"value"`
}

type permissionGroup struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Scopes   []string `json:"scopes"`
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

type genericMintRequest struct {
	Owner     string
	AccountID string
	Payload   map[string]any
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
	updateCmd.Flags().Uint16Var(&priority, "priority", 0, "Priority for MX records")
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
	setCmd.Flags().Uint16Var(&priority, "priority", 0, "Priority for MX records")
	setCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")

	listCmd := &cobra.Command{
		Use:   "list [type] [key]",
		Short: "List DNS records for a domain or the current profile domain",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDomain, err := resolveDomain()
			if err != nil {
				return err
			}
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}

			recordType := ""
			key := ""
			if len(args) > 0 {
				recordType = args[0]
			}
			if len(args) > 1 {
				key = args[1]
			}

			records, err := listDNSRecords(resolvedToken, resolvedDomain, recordType, key)
			if err != nil {
				return err
			}
			printDNSRecords(records)
			return nil
		},
	}
	listCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	listCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")

	getCmd := &cobra.Command{
		Use:   "get [type] [key]",
		Short: "Get one or more exact-match DNS records for the current profile domain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDomain, err := resolveDomain()
			if err != nil {
				return err
			}
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}

			records, err := listDNSRecords(resolvedToken, resolvedDomain, args[0], args[1])
			if err != nil {
				return err
			}
			printDNSRecords(records)
			return nil
		},
	}
	getCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	getCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")

	deleteCmd := &cobra.Command{
		Use:   "delete [type] [key]",
		Short: "Delete DNS records from the current profile domain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDomain, err := resolveDomain()
			if err != nil {
				return err
			}
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}

			deleted, err := deleteDNSRecords(resolvedToken, resolvedDomain, args[0], args[1], recordValue, deleteAll)
			if err != nil {
				return err
			}
			fmt.Printf("✅ Deleted %d record(s)\n", deleted)
			return nil
		},
	}
	deleteCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	deleteCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")
	deleteCmd.Flags().StringVar(&recordValue, "value", "", "Only delete records matching this exact content value")
	deleteCmd.Flags().BoolVar(&deleteAll, "all", false, "Delete all matching records instead of only the first one")

	aCmd := makeRecordShortcutCommand("a", "A", "ipv4", "Shortcut for setting an A record on the default domain")
	aaaaCmd := makeRecordShortcutCommand("aaaa", "AAAA", "ipv6", "Shortcut for setting an AAAA record on the default domain")
	cnameCmd := makeRecordShortcutCommand("cname", "CNAME", "target", "Shortcut for setting a CNAME record on the default domain")
	txtCmd := makeRecordShortcutCommand("txt", "TXT", "text", "Shortcut for setting a TXT record on the default domain")
	mxCmd := &cobra.Command{
		Use:   "mx [key] [priority] [mail-server] [comment (optional)]",
		Short: "Shortcut for setting an MX record on the default domain",
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

			parsedPriority, err := parsePriority(args[1])
			if err != nil {
				return err
			}
			priority = parsedPriority

			comment := ""
			if len(args) > 3 {
				comment = args[3]
			}

			return updateDNSRecord(resolvedToken, resolvedDomain, "MX", args[0], args[2], comment)
		},
	}
	mxCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	mxCmd.Flags().BoolVar(&proxied, "proxied", true, "Whether to enable Cloudflare proxying")
	mxCmd.Flags().Int16Var(&ttl, "ttl", 3600, "Time to live for the DNS record in seconds")
	mxCmd.Flags().BoolVar(&upsert, "upsert", true, "Create the DNS record if it does not exist")
	mxCmd.Flags().StringVar(&domain, "domain", "", "Default domain override")

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

	mintGenericCmd := &cobra.Command{
		Use:   "mint:token [name]",
		Short: "Mint a generic Cloudflare token by permission name and scope",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tokenName := fmt.Sprintf("%s cloudflare token %s", profile, time.Now().UTC().Format("20060102-150405"))
			if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
				tokenName = strings.TrimSpace(args[0])
			}

			bootstrap, err := resolveBootstrapToken()
			if err != nil {
				return err
			}

			mintReq, err := buildMintRequest(tokenName)
			if err != nil {
				return err
			}

			token, err := mintGenericToken(bootstrap, mintReq)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Minted %s token %q (%s)\n", mintReq.Owner, token.Name, token.ID)

			service := tokenStoreService
			if service == "" {
				service = defaultNamedTokenServiceName(tokenName)
			}
			if storeMintedToken {
				if err := writeSecretToKeychain(service, token.Value); err != nil {
					return fmt.Errorf("minted token, but failed to store it in keychain: %w", err)
				}
				fmt.Printf("✅ Stored minted token in keychain service %q\n", service)
			}
			if activateMintedToken {
				if err := writeSecretToKeychain(defaultAPIServiceName(), token.Value); err != nil {
					return fmt.Errorf("minted token, but failed to activate it: %w", err)
				}
				fmt.Printf("✅ Activated minted token in keychain service %q\n", defaultAPIServiceName())
			}
			if !storeMintedToken && !activateMintedToken {
				fmt.Printf("Use it immediately with:\nCF_API_TOKEN=%s cf doctor\n", token.Value)
			}
			return nil
		},
	}
	mintGenericCmd.Flags().StringVar(&bootstrapToken, "bootstrap-token", "", "Cloudflare bootstrap token")
	mintGenericCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	mintGenericCmd.Flags().StringVar(&zoneID, "zone-id", "", "Cloudflare zone ID")
	mintGenericCmd.Flags().StringVar(&expiresOn, "expires-on", "", "Optional RFC3339 expiry time for the minted token")
	mintGenericCmd.Flags().StringVar(&tokenOwner, "owner", "user", "Token owner type: user or account")
	mintGenericCmd.Flags().StringVar(&tokenScope, "scope", "", "Resource scope: zone, account, all-zones-in-account, all-zones, or all-accounts")
	mintGenericCmd.Flags().StringVar(&tokenResourceID, "resource-id", "", "Explicit resource ID override for the chosen scope")
	mintGenericCmd.Flags().StringVar(&tokenPreset, "preset", "", "Preset: dns-edit, dns-read, zone-read, zone-write")
	mintGenericCmd.Flags().StringSliceVar(&tokenPermissions, "permission", nil, "Permission group names, repeatable")
	mintGenericCmd.Flags().StringVar(&tokenStoreService, "store-service", "", "Keychain service name for storing the minted token")
	mintGenericCmd.Flags().BoolVar(&storeMintedToken, "store", true, "Store the minted token in the macOS keychain")
	mintGenericCmd.Flags().BoolVar(&activateMintedToken, "activate", false, "Also store the minted token as the active API token for this profile")

	permsCmd := &cobra.Command{
		Use:   "permissions:list [filter]",
		Short: "List Cloudflare token permission groups available to the bootstrap token",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bootstrap, err := resolveBootstrapToken()
			if err != nil {
				return err
			}

			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}

			perms, err := listPermissionGroups(bootstrap)
			if err != nil {
				return err
			}
			printPermissionGroups(perms, filter)
			return nil
		},
	}
	permsCmd.Flags().StringVar(&bootstrapToken, "bootstrap-token", "", "Cloudflare bootstrap token")

	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(aCmd)
	rootCmd.AddCommand(aaaaCmd)
	rootCmd.AddCommand(cnameCmd)
	rootCmd.AddCommand(txtCmd)
	rootCmd.AddCommand(mxCmd)
	rootCmd.AddCommand(mintCmd)
	rootCmd.AddCommand(mintGenericCmd)
	rootCmd.AddCommand(permsCmd)
	rootCmd.AddCommand(doctorCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}
}

func updateDNSRecord(apiToken, domain, recordType, key, value, comment string) error {
	client := &http.Client{}
	zoneID, err := fetchZoneID(client, apiToken, domain)
	if err != nil {
		return err
	}

	recordName := normalizeRecordName(domain, key)
	recordDisplayName := key
	if recordDisplayName == "" {
		recordDisplayName = "@"
	}

	recordResp, err := getJSON[[]dnsRecord](client, fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=%s&name=%s",
		zoneID,
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
		if err := insertRecord(apiToken, recordType, recordName, value, comment, zoneID, client); err != nil {
			return err
		}
	} else {
		if err := updateRecord(apiToken, recordType, recordName, value, comment, zoneID, recordResp.Result[0].ID, client); err != nil {
			return err
		}
	}

	fmt.Printf("✅ %s %s -> %s\n", strings.ToUpper(recordType), recordDisplayName, value)
	return nil
}

func listDNSRecords(apiToken, domain, recordType, key string) ([]dnsRecord, error) {
	client := &http.Client{}
	zoneID, err := fetchZoneID(client, apiToken, domain)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("per_page", "100")
	if recordType != "" {
		query.Set("type", strings.ToUpper(recordType))
	}

	resp, err := getJSON[[]dnsRecord](client, fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/zones/%s/dns_records?%s",
		zoneID,
		query.Encode(),
	), apiToken)
	if err != nil {
		return nil, err
	}

	if key == "" {
		return resp.Result, nil
	}

	expectedName := normalizeRecordName(domain, key)
	filtered := make([]dnsRecord, 0, len(resp.Result))
	for _, record := range resp.Result {
		if strings.EqualFold(record.Name, expectedName) {
			filtered = append(filtered, record)
		}
	}

	return filtered, nil
}

func deleteDNSRecords(apiToken, domain, recordType, key, content string, all bool) (int, error) {
	client := &http.Client{}
	zoneID, err := fetchZoneID(client, apiToken, domain)
	if err != nil {
		return 0, err
	}

	records, err := listDNSRecords(apiToken, domain, recordType, key)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, fmt.Errorf("no %s record found for %s", strings.ToUpper(recordType), normalizeRecordName(domain, key))
	}

	filtered := make([]dnsRecord, 0, len(records))
	for _, record := range records {
		if content == "" || record.Content == content {
			filtered = append(filtered, record)
		}
	}
	if len(filtered) == 0 {
		return 0, fmt.Errorf("no matching records found for %s with value %q", normalizeRecordName(domain, key), content)
	}
	if !all {
		filtered = filtered[:1]
	}

	for _, record := range filtered {
		req, err := newJSONRequest("DELETE", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, record.ID), apiToken, nil)
		if err != nil {
			return 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		if err := checkAPIResponse(resp); err != nil {
			resp.Body.Close()
			return 0, err
		}
		resp.Body.Close()
	}

	return len(filtered), nil
}

func mintDNSToken(bootstrap, resolvedZoneID, tokenName, tokenExpiry string) (*tokenCreateResult, error) {
	mintReq := genericMintRequest{
		Owner: "user",
		Payload: map[string]any{
			"name": tokenName,
			"policies": []tokenPolicy{
				{
					Effect: "allow",
					PermissionGroups: []tokenPermissionGroup{
						{ID: "c8fed203ed3043cba015a93ad1616f1f", Name: "Zone Read"},
						{ID: "4755a26eedb94da69e1066d98aa820be", Name: "DNS Write"},
					},
					Resources: map[string]any{
						fmt.Sprintf("com.cloudflare.api.account.zone.%s", resolvedZoneID): "*",
					},
				},
			},
		},
	}
	if tokenExpiry != "" {
		mintReq.Payload["expires_on"] = tokenExpiry
	}

	return mintGenericToken(bootstrap, mintReq)
}

func insertRecord(apiToken, recordType, recordName, value, comment, zoneID string, client *http.Client) error {
	payload, err := makeRecordPayload(recordType, recordName, value, comment)
	if err != nil {
		return err
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
	payload, err := makeRecordPayload(recordType, recordName, value, comment)
	if err != nil {
		return err
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

func makeRecordPayload(recordType, recordName, value, comment string) (map[string]any, error) {
	upperType := strings.ToUpper(recordType)
	payload := map[string]any{
		"type":    upperType,
		"name":    recordName,
		"content": value,
		"ttl":     ttl,
		"proxied": proxied,
		"comment": comment,
	}
	if upperType == "MX" {
		if priority == 0 {
			return nil, errors.New("MX records require --priority or the mx shortcut syntax: cf mx <key> <priority> <mail-server>")
		}
		payload["priority"] = priority
	}

	return payload, nil
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

func fetchZoneID(client *http.Client, apiToken, domain string) (string, error) {
	zoneResp, err := getJSON[[]zone](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", url.QueryEscape(domain)), apiToken)
	if err != nil {
		return "", err
	}
	if len(zoneResp.Result) == 0 {
		return "", fmt.Errorf("zone not found for %s", domain)
	}
	return zoneResp.Result[0].ID, nil
}

func mintGenericToken(bootstrap string, req genericMintRequest) (*tokenCreateResult, error) {
	client := &http.Client{}

	endpoint := "https://api.cloudflare.com/client/v4/user/tokens"
	if req.Owner == "account" {
		if req.AccountID == "" {
			return nil, errors.New("account-owned tokens require an account ID")
		}
		endpoint = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/tokens", req.AccountID)
	}

	httpReq, err := newJSONRequest("POST", endpoint, bootstrap, req.Payload)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
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

func buildMintRequest(tokenName string) (genericMintRequest, error) {
	owner := strings.ToLower(strings.TrimSpace(tokenOwner))
	if owner == "" {
		owner = "user"
	}
	if owner != "user" && owner != "account" {
		return genericMintRequest{}, fmt.Errorf("unsupported owner %q; use user or account", tokenOwner)
	}

	presetPermissions, presetScope, err := permissionsFromPreset(tokenPreset)
	if err != nil {
		return genericMintRequest{}, err
	}

	scope := strings.TrimSpace(tokenScope)
	if scope == "" {
		scope = presetScope
	}
	if scope == "" {
		return genericMintRequest{}, errors.New("token scope is required; use --scope or a --preset")
	}

	permissionNames := append([]string{}, tokenPermissions...)
	if len(permissionNames) == 0 {
		permissionNames = presetPermissions
	}
	if len(permissionNames) == 0 {
		return genericMintRequest{}, errors.New("at least one --permission or a --preset is required")
	}

	bootstrap, err := resolveBootstrapToken()
	if err != nil {
		return genericMintRequest{}, err
	}
	groups, err := resolvePermissionGroupsByName(bootstrap, permissionNames)
	if err != nil {
		return genericMintRequest{}, err
	}

	resolvedAccountID := ""
	if owner == "account" || scope == "account" || scope == "all-zones-in-account" {
		resolvedAccountID, err = resolveAccountID()
		if err != nil {
			return genericMintRequest{}, err
		}
	}

	resourceID := strings.TrimSpace(tokenResourceID)
	if resourceID == "" && scope == "zone" {
		resourceID, err = resolveZoneID()
		if err != nil {
			return genericMintRequest{}, err
		}
	}
	if resourceID == "" && scope == "account" {
		resourceID = resolvedAccountID
	}

	resources, err := buildResources(scope, resourceID, resolvedAccountID)
	if err != nil {
		return genericMintRequest{}, err
	}

	payload := map[string]any{
		"name": tokenName,
		"policies": []tokenPolicy{
			{
				Effect:           "allow",
				PermissionGroups: groups,
				Resources:        resources,
			},
		},
	}
	if expiresOn != "" {
		payload["expires_on"] = expiresOn
	}

	return genericMintRequest{
		Owner:     owner,
		AccountID: resolvedAccountID,
		Payload:   payload,
	}, nil
}

func permissionsFromPreset(preset string) ([]string, string, error) {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "":
		return nil, "", nil
	case "dns-edit":
		return []string{"Zone Read", "DNS Write"}, "zone", nil
	case "dns-read":
		return []string{"Zone Read", "DNS Read"}, "zone", nil
	case "zone-read":
		return []string{"Zone Read"}, "zone", nil
	case "zone-write":
		return []string{"Zone Write"}, "zone", nil
	default:
		return nil, "", fmt.Errorf("unknown preset %q", preset)
	}
}

func listPermissionGroups(bootstrap string) ([]permissionGroup, error) {
	client := &http.Client{}
	resp, err := getJSON[[]permissionGroup](client, "https://api.cloudflare.com/client/v4/user/tokens/permission_groups", bootstrap)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func resolvePermissionGroupsByName(bootstrap string, names []string) ([]tokenPermissionGroup, error) {
	available, err := listPermissionGroups(bootstrap)
	if err != nil {
		return nil, err
	}

	byName := make(map[string]permissionGroup, len(available))
	for _, group := range available {
		byName[strings.ToLower(group.Name)] = group
	}

	resolved := make([]tokenPermissionGroup, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}

		if group, ok := byName[strings.ToLower(trimmed)]; ok {
			resolved = append(resolved, tokenPermissionGroup{ID: group.ID, Name: group.Name})
			continue
		}
		if len(trimmed) == 32 {
			resolved = append(resolved, tokenPermissionGroup{ID: trimmed, Name: trimmed})
			continue
		}
		return nil, fmt.Errorf("permission group %q not found; use `cf permissions:list %s` to discover valid names", trimmed, shellSafeFilter(trimmed))
	}

	if len(resolved) == 0 {
		return nil, errors.New("no valid permission groups resolved")
	}
	return resolved, nil
}

func buildResources(scope, resourceID, accountID string) (map[string]any, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "zone":
		if resourceID == "" {
			return nil, errors.New("zone scope requires a zone ID")
		}
		return map[string]any{
			fmt.Sprintf("com.cloudflare.api.account.zone.%s", resourceID): "*",
		}, nil
	case "account":
		if resourceID == "" {
			return nil, errors.New("account scope requires an account ID")
		}
		return map[string]any{
			fmt.Sprintf("com.cloudflare.api.account.%s", resourceID): "*",
		}, nil
	case "all-zones-in-account":
		if accountID == "" {
			return nil, errors.New("all-zones-in-account scope requires an account ID")
		}
		return map[string]any{
			fmt.Sprintf("com.cloudflare.api.account.%s", accountID): map[string]any{
				"com.cloudflare.api.account.zone.*": "*",
			},
		}, nil
	case "all-zones":
		return map[string]any{
			"com.cloudflare.api.account.zone.*": "*",
		}, nil
	case "all-accounts":
		return map[string]any{
			"com.cloudflare.api.account.*": "*",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported scope %q; use zone, account, all-zones-in-account, all-zones, or all-accounts", scope)
	}
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

func defaultNamedTokenServiceName(tokenName string) string {
	return fmt.Sprintf("%s cloudflare token %s", profile, slugify(tokenName))
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

func printPermissionGroups(groups []permissionGroup, filter string) {
	filter = strings.ToLower(strings.TrimSpace(filter))
	filtered := make([]permissionGroup, 0, len(groups))
	for _, group := range groups {
		if filter == "" ||
			strings.Contains(strings.ToLower(group.Name), filter) ||
			strings.Contains(strings.ToLower(group.Category), filter) ||
			strings.Contains(strings.ToLower(strings.Join(group.Scopes, " ")), filter) {
			filtered = append(filtered, group)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	if len(filtered) == 0 {
		fmt.Println("No permission groups found.")
		return
	}

	for _, group := range filtered {
		fmt.Printf("%s\t%s\tscopes=%s\tid=%s\n", group.Name, group.Category, strings.Join(group.Scopes, ","), group.ID)
	}
}

func printDNSRecords(records []dnsRecord) {
	if len(records) == 0 {
		fmt.Println("No records found.")
		return
	}

	for _, record := range records {
		comment := record.Comment
		if comment == "" {
			comment = "-"
		}
		fmt.Printf("%s\t%s\t%s\tttl=%d\tproxied=%t\tcomment=%s\n",
			record.Type,
			record.Name,
			record.Content,
			record.TTL,
			record.Proxied,
			comment,
		)
	}
}

func makeRecordShortcutCommand(name, recordType, valueLabel, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [key] [%s] [comment (optional)]", name, valueLabel),
		Short: short,
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

			return updateDNSRecord(resolvedToken, resolvedDomain, recordType, args[0], args[1], comment)
		},
	}

	cmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	cmd.Flags().BoolVar(&proxied, "proxied", true, "Whether to enable Cloudflare proxying")
	cmd.Flags().Int16Var(&ttl, "ttl", 3600, "Time to live for the DNS record in seconds")
	cmd.Flags().BoolVar(&upsert, "upsert", true, "Create the DNS record if it does not exist")
	cmd.Flags().Uint16Var(&priority, "priority", 0, "Priority for MX records")
	cmd.Flags().StringVar(&domain, "domain", "", "Default domain override")
	return cmd
}

func parsePriority(value string) (uint16, error) {
	var parsed uint16
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return 0, fmt.Errorf("invalid MX priority %q", value)
	}
	return parsed, nil
}

func slugify(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return "token"
	}

	var b strings.Builder
	lastDash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "token"
	}
	return result
}

func shellSafeFilter(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "\"\""
	}
	return fmt.Sprintf("%q", value)
}
