package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
var workersSince string
var workersLimit int
var workersView string
var workersSearch string
var workerLogsPersist bool
var workerLogsInvocation bool
var workerLogsSampleRate float64
var workerLogsEnableLogpush bool
var workerR2Bucket string
var workerR2Path string
var workerR2AccessKeyID string
var workerR2SecretAccessKey string
var workerLogpushName string
var workerLogpushSampleRate float64
var workerLogpushMaxUploadInterval int
var workerLogpushMaxUploadRecords int
var workerLogpushMaxUploadBytes int
var r2Jurisdiction string
var r2LocationHint string
var r2StorageClass string
var activateR2ControlToken bool
var r2StoreLogpushSecrets bool
var wranglerCmd string
var wranglerAccountLabel string

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

type workerRoute struct {
	Pattern string `json:"pattern"`
}

type workerObservabilityLogs struct {
	Enabled        bool    `json:"enabled"`
	InvocationLogs bool    `json:"invocation_logs"`
	Persist        bool    `json:"persist"`
	HeadSampling   float64 `json:"head_sampling_rate"`
}

type workerObservability struct {
	Enabled bool                    `json:"enabled"`
	Logs    workerObservabilityLogs `json:"logs"`
}

type workerScript struct {
	ID            string              `json:"id"`
	ModifiedOn    string              `json:"modified_on"`
	Logpush       bool                `json:"logpush"`
	Routes        []workerRoute       `json:"routes"`
	Observability workerObservability `json:"observability"`
}

type workerScriptSettings struct {
	Logpush       bool                `json:"logpush"`
	Observability workerObservability `json:"observability"`
}

type telemetryEvent struct {
	Dataset   string         `json:"dataset"`
	Source    any            `json:"source"`
	Timestamp int64          `json:"timestamp"`
	Metadata  map[string]any `json:"$metadata"`
	Workers   map[string]any `json:"$workers"`
}

type telemetryEventsResult struct {
	Count  int              `json:"count"`
	Events []telemetryEvent `json:"events"`
}

type telemetryQueryResult struct {
	Events      telemetryEventsResult       `json:"events"`
	Invocations map[string][]telemetryEvent `json:"invocations"`
}

type logpushOutputOptions struct {
	FieldNames      []string `json:"field_names,omitempty"`
	OutputType      string   `json:"output_type,omitempty"`
	TimestampFormat string   `json:"timestamp_format,omitempty"`
	SampleRate      float64  `json:"sample_rate,omitempty"`
}

type logpushJob struct {
	ID                       int                  `json:"id"`
	Name                     string               `json:"name"`
	Dataset                  string               `json:"dataset"`
	DestinationConf          string               `json:"destination_conf"`
	Enabled                  bool                 `json:"enabled"`
	ErrorMessage             string               `json:"error_message"`
	LastComplete             string               `json:"last_complete"`
	LastError                string               `json:"last_error"`
	MaxUploadBytes           int                  `json:"max_upload_bytes"`
	MaxUploadIntervalSeconds int                  `json:"max_upload_interval_seconds"`
	MaxUploadRecords         int                  `json:"max_upload_records"`
	OutputOptions            logpushOutputOptions `json:"output_options"`
}

type r2Bucket struct {
	Name         string `json:"name"`
	Jurisdiction string `json:"jurisdiction"`
	Location     string `json:"location"`
	StorageClass string `json:"storage_class"`
	CreationDate string `json:"creation_date"`
}

type profileRegistry struct {
	Profiles []string `json:"profiles"`
}

type wranglerWhoamiInfo struct {
	Email       string
	AccountID   string
	AccountName string
}

type wranglerAccount struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	AddedAt    time.Time `json:"added_at"`
	ConfigHash string    `json:"config_hash,omitempty"`
}

type wranglerAccountsDB struct {
	Accounts    []wranglerAccount `json:"accounts"`
	Current     string            `json:"current"`
	WranglerCmd string            `json:"wrangler_cmd,omitempty"`
}

type apiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareAPIError struct {
	Messages []apiMessage
}

func (e cloudflareAPIError) Error() string {
	if len(e.Messages) == 0 {
		return "cloudflare API request failed"
	}

	parts := make([]string, 0, len(e.Messages))
	for _, apiErr := range e.Messages {
		parts = append(parts, fmt.Sprintf("%d: %s", apiErr.Code, apiErr.Message))
	}
	return strings.Join(parts, "; ")
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
		Short: "Cloudflare operations CLI for DNS, tokens, Workers, R2, profiles, and Wrangler auth",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if !commandNeedsExplicitProfile(cmd) {
				return nil
			}
			if strings.TrimSpace(profile) == "" {
				return errors.New("cloudflare profile is required; pass --profile <name> or set CF_PROFILE")
			}
			if err := registerProfile(profile); err != nil {
				return err
			}
			fmt.Printf("Active profile: %s\n", profile)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&profile, "profile", defaultProfile(), "Cloudflare profile name used for keychain lookups")
	rootCmd.PersistentFlags().StringVar(&apiTokenKeychainService, "api-token-keychain-service", "", "Override the macOS keychain service name for the active DNS API token")
	rootCmd.PersistentFlags().StringVar(&bootstrapTokenKeychainService, "bootstrap-token-keychain-service", "", "Override the macOS keychain service name for the bootstrap token")
	rootCmd.PersistentFlags().StringVar(&zoneIDKeychainService, "zone-id-keychain-service", "", "Override the macOS keychain service name for the zone ID")
	rootCmd.PersistentFlags().StringVar(&accountIDKeychainService, "account-id-keychain-service", "", "Override the macOS keychain service name for the account ID")
	rootCmd.PersistentFlags().StringVar(&domainKeychainService, "domain-keychain-service", "", "Override the macOS keychain service name for the default domain")

	dnsCmd := &cobra.Command{
		Use:   "dns",
		Short: "Manage Cloudflare DNS records for the active profile",
	}

	updateCmd := &cobra.Command{
		Use:   "update [domain] [type] [key] [value] [comment (optional)]",
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
			reportSecretResolution("workers log R2 bucket", resolveWorkerR2Bucket)
			reportSecretResolution("workers log R2 access key id", resolveWorkerR2AccessKeyID)
			reportSecretResolution("workers log R2 secret access key", resolveWorkerR2SecretAccessKey)
			reportSecretResolution("workers log R2 path", func() (string, error) {
				return resolveWorkerR2Path("default")
			})
			return nil
		},
	}

	skillCmd := &cobra.Command{
		Use:   "skill",
		Short: "Print the built-in agent guide for this CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(`Cloudflare CLI Agent Guide

Purpose
- Use this CLI for Cloudflare DNS, token minting, Workers logs, R2 helpers, profile discovery, and Wrangler auth switching.

Core rule
- For Cloudflare API operations, always pass --profile <name> or set CF_PROFILE.

Top-level command areas
- cf dns ...
- cf tokens ...
- cf workers ...
- cf r2 ...
- cf profiles ...
- cf wrangler ...
- cf doctor

Fastest DNS flow
- All DNS operations use the cf dns <cmd> shape.
- cf --profile <name> dns get A @
- cf --profile <name> dns a @ 1.2.3.4
- cf --profile <name> dns txt verify abc123
- cf --profile <name> dns delete TXT verify --value abc123

Fastest token flow
- cf --profile <name> tokens dns
- cf --profile <name> tokens mint "token name" --preset dns-edit
- cf --profile <name> tokens permissions list DNS

Workers flow
- cf --profile <name> workers list
- cf --profile <name> workers logs <worker> --since 10m --limit 50
- cf --profile <name> workers logs enable <worker>
- cf --profile <name> workers logs sink setup-r2 <worker>

R2 flow
- cf --profile <name> r2 bucket create <name>
- cf --profile <name> r2 creds mint <bucket>
- cf --profile <name> r2 logpush bootstrap <worker> [bucket]

Profile discovery
- cf profiles list
- cf profiles add <name>

Wrangler auth switching
- Separate from Cloudflare API profiles.
- cf wrangler add --wrangler-cmd "npx wrangler" --label <label>
- cf wrangler list
- cf wrangler current
- cf wrangler switch <name-or-id>
- cf wrangler login

Notes
- cf doctor shows what resolves from env/keychain for the active API profile.
- cf skill prints this guide, so no external skill file is required.`)
			return nil
		},
	}

	profilesCmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage known Cloudflare profiles",
	}

	profilesListCmd := &cobra.Command{
		Use:   "list",
		Short: "List locally known Cloudflare profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := readProfileRegistry()
			if err != nil {
				return err
			}
			if len(registry.Profiles) == 0 {
				fmt.Println("No known profiles.")
				fmt.Println("Add one with `cf profiles add <name>` or run any command with `--profile <name>`.")
				return nil
			}
			for _, p := range registry.Profiles {
				fmt.Println(p)
			}
			return nil
		},
	}

	profilesAddCmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Register a profile name locally without touching the keychain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return errors.New("profile name is required")
			}
			if err := registerProfile(name); err != nil {
				return err
			}
			fmt.Printf("✅ Registered profile: %s\n", name)
			return nil
		},
	}

	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesAddCmd)

	tokensCmd := &cobra.Command{
		Use:   "tokens",
		Short: "Mint and inspect Cloudflare API tokens",
	}

	wranglerRootCmd := &cobra.Command{
		Use:   "wrangler",
		Short: "Switch and manage local Wrangler authentication snapshots",
	}

	wranglerListCmd := &cobra.Command{
		Use:   "list",
		Short: "List saved Wrangler auth snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := loadWranglerAccountsDB()
			if err != nil {
				return err
			}
			if len(db.Accounts) == 0 {
				fmt.Println("No saved Wrangler accounts.")
				fmt.Println("Use `cf wrangler add` after logging in with Wrangler, or `cf wrangler login` to log in and save one.")
				return nil
			}
			for _, account := range db.Accounts {
				currentMarker := ""
				if account.ID == db.Current {
					currentMarker = " [current]"
				}
				fmt.Printf("%s\t%s\t%s%s\n", account.Name, account.Email, account.ID, currentMarker)
			}
			return nil
		},
	}

	wranglerCurrentCmd := &cobra.Command{
		Use:   "current",
		Short: "Show the current saved Wrangler account",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := loadWranglerAccountsDB()
			if err != nil {
				return err
			}
			if db.Current == "" {
				return errors.New("no current Wrangler account saved; use `cf wrangler add` or `cf wrangler login` first")
			}
			account := db.getAccount(db.Current)
			if account == nil {
				return fmt.Errorf("current Wrangler account %q is missing from the local database", db.Current)
			}
			fmt.Printf("%s\t%s\t%s\n", account.Name, account.Email, account.ID)
			return nil
		},
	}

	wranglerAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Save the current Wrangler authentication as a switchable snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := loadWranglerAccountsDB()
			if err != nil {
				return err
			}
			resolvedWranglerCmd, err := ensureWranglerCommand(db)
			if err != nil {
				return err
			}
			fmt.Printf("Checking current Wrangler auth with %q...\n", resolvedWranglerCmd)
			info, err := wranglerWhoami(resolvedWranglerCmd)
			if err != nil {
				return err
			}
			accountName := strings.TrimSpace(wranglerAccountLabel)
			if accountName == "" {
				accountName = info.AccountName
			}
			fmt.Printf("Saving Wrangler auth snapshot for %s...\n", accountName)
			configHash, err := saveWranglerAccountConfig(info.AccountID)
			if err != nil {
				return err
			}
			db.addAccount(wranglerAccount{
				ID:         info.AccountID,
				Name:       accountName,
				Email:      info.Email,
				AddedAt:    time.Now(),
				ConfigHash: configHash,
			})
			db.Current = info.AccountID
			if err := saveWranglerAccountsDB(db); err != nil {
				return err
			}
			fmt.Printf("✅ Saved Wrangler account: %s (%s)\n", accountName, info.Email)
			fmt.Printf("account_id=%s\n", info.AccountID)
			return nil
		},
	}
	wranglerAddCmd.Flags().StringVar(&wranglerCmd, "wrangler-cmd", "", "Override the Wrangler command, for example 'wrangler' or 'npx wrangler'")
	wranglerAddCmd.Flags().StringVar(&wranglerAccountLabel, "label", "", "Optional local label override for this Wrangler account")

	wranglerSwitchCmd := &cobra.Command{
		Use:   "switch [account-name-or-id]",
		Short: "Switch the local Wrangler auth snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := loadWranglerAccountsDB()
			if err != nil {
				return err
			}
			if len(db.Accounts) == 0 {
				return errors.New("no saved Wrangler accounts; use `cf wrangler add` or `cf wrangler login` first")
			}

			target, err := db.findAccount(strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}

			if db.Current != "" && db.Current != target.ID {
				current := db.getAccount(db.Current)
				if current != nil {
					changed, newHash, err := saveWranglerAccountConfigIfChanged(current.ID, current.ConfigHash)
					if err == nil && changed {
						current.ConfigHash = newHash
						db.addAccount(*current)
					}
				}
			}

			if err := restoreWranglerAccountConfig(target.ID); err != nil {
				return err
			}
			db.Current = target.ID
			if err := saveWranglerAccountsDB(db); err != nil {
				return err
			}
			fmt.Printf("✅ Switched Wrangler auth to: %s (%s)\n", target.Name, target.Email)
			fmt.Printf("account_id=%s\n", target.ID)
			return nil
		},
	}

	wranglerLoginCmd := &cobra.Command{
		Use:   "login",
		Short: "Run Wrangler login, then save the new auth snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := loadWranglerAccountsDB()
			if err != nil {
				return err
			}
			resolvedWranglerCmd, err := ensureWranglerCommand(db)
			if err != nil {
				return err
			}
			if db.Current != "" {
				current := db.getAccount(db.Current)
				if current != nil {
					changed, newHash, err := saveWranglerAccountConfigIfChanged(current.ID, current.ConfigHash)
					if err == nil && changed {
						current.ConfigHash = newHash
						db.addAccount(*current)
						_ = saveWranglerAccountsDB(db)
					}
				}
			}
			fmt.Println("Running Wrangler login...")
			if err := runWranglerLogin(resolvedWranglerCmd); err != nil {
				return err
			}
			info, err := wranglerWhoami(resolvedWranglerCmd)
			if err != nil {
				return err
			}
			accountName := strings.TrimSpace(wranglerAccountLabel)
			if accountName == "" {
				accountName = info.AccountName
			}
			configHash, err := saveWranglerAccountConfig(info.AccountID)
			if err != nil {
				return err
			}
			db.addAccount(wranglerAccount{
				ID:         info.AccountID,
				Name:       accountName,
				Email:      info.Email,
				AddedAt:    time.Now(),
				ConfigHash: configHash,
			})
			db.Current = info.AccountID
			if err := saveWranglerAccountsDB(db); err != nil {
				return err
			}
			fmt.Printf("✅ Logged in and saved Wrangler account: %s (%s)\n", accountName, info.Email)
			fmt.Printf("account_id=%s\n", info.AccountID)
			return nil
		},
	}
	wranglerLoginCmd.Flags().StringVar(&wranglerCmd, "wrangler-cmd", "", "Override the Wrangler command, for example 'wrangler' or 'npx wrangler'")
	wranglerLoginCmd.Flags().StringVar(&wranglerAccountLabel, "label", "", "Optional local label override for this Wrangler account")

	wranglerRootCmd.AddCommand(wranglerListCmd)
	wranglerRootCmd.AddCommand(wranglerCurrentCmd)
	wranglerRootCmd.AddCommand(wranglerAddCmd)
	wranglerRootCmd.AddCommand(wranglerSwitchCmd)
	wranglerRootCmd.AddCommand(wranglerLoginCmd)

	mintCmd := &cobra.Command{
		Use:   "dns [name]",
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
				fmt.Printf("Use it with:\ncf --profile %s dns update <domain> <type> <key> <value> [comment]\n", profile)
				return nil
			}

			fmt.Printf("Use it immediately with:\nCF_API_TOKEN=%s cf --profile %s dns update <domain> <type> <key> <value> [comment]\n", token.Value, profile)
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
		Use:   "mint [name]",
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
	mintGenericCmd.Flags().StringVar(&tokenPreset, "preset", "", "Preset: dns-edit, dns-read, zone-read, zone-write, workers-logs-read, workers-logs-admin")
	mintGenericCmd.Flags().StringSliceVar(&tokenPermissions, "permission", nil, "Permission group names, repeatable")
	mintGenericCmd.Flags().StringVar(&tokenStoreService, "store-service", "", "Keychain service name for storing the minted token")
	mintGenericCmd.Flags().BoolVar(&storeMintedToken, "store", true, "Store the minted token in the macOS keychain")
	mintGenericCmd.Flags().BoolVar(&activateMintedToken, "activate", false, "Also store the minted token as the active API token for this profile")

	tokensPermissionsCmd := &cobra.Command{
		Use:   "permissions",
		Short: "Inspect permission groups available for token minting",
	}

	permsCmd := &cobra.Command{
		Use:   "list [filter]",
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
	tokensPermissionsCmd.AddCommand(permsCmd)

	r2Cmd := &cobra.Command{
		Use:   "r2",
		Short: "Cloudflare R2 utilities",
	}

	r2BucketCmd := &cobra.Command{
		Use:   "bucket",
		Short: "Manage R2 buckets",
	}

	r2BucketCreateCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create an R2 bucket for the current account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}
			resolvedAccountID, err := resolveAccountID()
			if err != nil {
				return err
			}

			bucket, created, err := ensureR2Bucket(resolvedToken, resolvedAccountID, args[0], r2Jurisdiction, r2LocationHint, r2StorageClass)
			if err != nil {
				return err
			}
			if created {
				fmt.Printf("✅ Created R2 bucket %s\n", bucket.Name)
			} else {
				fmt.Printf("✅ Reusing existing R2 bucket %s\n", bucket.Name)
			}
			fmt.Printf("jurisdiction=%s location=%s storage_class=%s\n", emptyDash(bucket.Jurisdiction), emptyDash(bucket.Location), emptyDash(bucket.StorageClass))
			return nil
		},
	}
	r2BucketCreateCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	r2BucketCreateCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	r2BucketCreateCmd.Flags().StringVar(&r2Jurisdiction, "jurisdiction", "default", "R2 jurisdiction: default, eu, or fedramp")
	r2BucketCreateCmd.Flags().StringVar(&r2LocationHint, "location", "", "Optional bucket location hint: apac, eeur, enam, weur, wnam, or oc")
	r2BucketCreateCmd.Flags().StringVar(&r2StorageClass, "storage-class", "Standard", "Default storage class: Standard or InfrequentAccess")

	r2CredsCmd := &cobra.Command{
		Use:   "creds",
		Short: "Mint R2 S3-compatible credentials",
	}

	r2CredsMintCmd := &cobra.Command{
		Use:   "mint [bucket]",
		Short: "Mint bucket-scoped R2 S3 credentials via the Cloudflare token API",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bucketName := strings.TrimSpace(args[0])
			if bucketName == "" {
				return errors.New("bucket name is required")
			}
			bootstrap, err := resolveBootstrapToken()
			if err != nil {
				return err
			}
			resolvedAccountID, err := resolveAccountID()
			if err != nil {
				return err
			}

			token, accessKeyID, secretAccessKey, endpoint, err := mintR2BucketCredentials(
				bootstrap,
				resolvedAccountID,
				bucketName,
				r2Jurisdiction,
				fmt.Sprintf("%s r2 %s %s", profile, bucketName, time.Now().UTC().Format("20060102-150405")),
				expiresOn,
			)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Minted R2 bucket token %q (%s)\n", token.Name, token.ID)
			fmt.Printf("bucket=%s jurisdiction=%s endpoint=%s\n", bucketName, normalizeR2Jurisdiction(r2Jurisdiction), endpoint)
			fmt.Printf("access_key_id=%s\n", accessKeyID)
			fmt.Printf("secret_access_key=%s\n", secretAccessKey)

			if r2StoreLogpushSecrets {
				if err := storeR2LogpushSecrets(bucketName, accessKeyID, secretAccessKey, "workers-trace-events"); err != nil {
					return err
				}
				fmt.Printf("✅ Stored logpush R2 credentials in keychain for profile %q\n", profile)
			}
			return nil
		},
	}
	r2CredsMintCmd.Flags().StringVar(&bootstrapToken, "bootstrap-token", "", "Cloudflare bootstrap token")
	r2CredsMintCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	r2CredsMintCmd.Flags().StringVar(&r2Jurisdiction, "jurisdiction", "default", "R2 jurisdiction: default, eu, or fedramp")
	r2CredsMintCmd.Flags().StringVar(&expiresOn, "expires-on", "", "Optional RFC3339 expiry time for the minted token")
	r2CredsMintCmd.Flags().BoolVar(&r2StoreLogpushSecrets, "store-logpush", true, "Store the resulting bucket, access key, secret key, and default path in the macOS keychain for Worker Logpush")

	r2LogpushCmd := &cobra.Command{
		Use:   "logpush",
		Short: "Bootstrap R2 for Worker Logpush",
	}

	r2LogpushBootstrapCmd := &cobra.Command{
		Use:   "bootstrap [worker] [bucket(optional)]",
		Short: "Create an R2 bucket, mint R2 credentials, store them, and configure Worker Logpush",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerName := strings.TrimSpace(args[0])
			bucketName := fmt.Sprintf("%s-workers-trace-events", profile)
			if len(args) == 2 && strings.TrimSpace(args[1]) != "" {
				bucketName = strings.TrimSpace(args[1])
			}

			bootstrap, err := resolveBootstrapToken()
			if err != nil {
				return err
			}
			resolvedAccountID, err := resolveAccountID()
			if err != nil {
				return err
			}

			controlToken, err := mintPresetToken(
				bootstrap,
				fmt.Sprintf("%s logpush control %s", profile, time.Now().UTC().Format("20060102-150405")),
				"workers-r2-logpush-admin",
				resolvedAccountID,
				"",
				expiresOn,
			)
			if err != nil {
				return err
			}

			if _, _, err := ensureR2Bucket(controlToken.Value, resolvedAccountID, bucketName, r2Jurisdiction, r2LocationHint, r2StorageClass); err != nil {
				return err
			}
			_, accessKeyID, secretAccessKey, _, err := mintR2BucketCredentials(
				bootstrap,
				resolvedAccountID,
				bucketName,
				r2Jurisdiction,
				fmt.Sprintf("%s r2 logpush %s %s", profile, bucketName, time.Now().UTC().Format("20060102-150405")),
				expiresOn,
			)
			if err != nil {
				return err
			}
			if err := storeR2LogpushSecrets(bucketName, accessKeyID, secretAccessKey, "workers-trace-events"); err != nil {
				return err
			}

			job, err := ensureWorkerLogpushR2Job(controlToken.Value, resolvedAccountID, workerName)
			if err != nil {
				return err
			}
			if _, err := enableWorkerLogs(controlToken.Value, resolvedAccountID, workerName, true, true, 1, true); err != nil {
				return err
			}
			if activateR2ControlToken {
				if err := writeSecretToKeychain(defaultAPIServiceName(), controlToken.Value); err != nil {
					return fmt.Errorf("bootstrapped R2 logpush, but failed to activate the control token: %w", err)
				}
				fmt.Printf("✅ Activated logpush control token in keychain service %q\n", defaultAPIServiceName())
			}

			fmt.Printf("✅ Bootstrapped R2 Logpush for %s using bucket %s\n", workerName, bucketName)
			fmt.Printf("job_id=%d destination=%s\n", job.ID, redactDestinationConf(job.DestinationConf))
			return nil
		},
	}
	r2LogpushBootstrapCmd.Flags().StringVar(&bootstrapToken, "bootstrap-token", "", "Cloudflare bootstrap token")
	r2LogpushBootstrapCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	r2LogpushBootstrapCmd.Flags().StringVar(&r2Jurisdiction, "jurisdiction", "default", "R2 jurisdiction: default, eu, or fedramp")
	r2LogpushBootstrapCmd.Flags().StringVar(&r2LocationHint, "location", "", "Optional bucket location hint: apac, eeur, enam, weur, wnam, or oc")
	r2LogpushBootstrapCmd.Flags().StringVar(&r2StorageClass, "storage-class", "Standard", "Default storage class: Standard or InfrequentAccess")
	r2LogpushBootstrapCmd.Flags().StringVar(&expiresOn, "expires-on", "", "Optional RFC3339 expiry time for minted helper tokens")
	r2LogpushBootstrapCmd.Flags().BoolVar(&activateR2ControlToken, "activate-control-token", true, "Activate the minted control-plane token as the profile API token")

	workerLogsRun := func(args []string) error {
		resolvedToken, err := resolveAPIToken()
		if err != nil {
			return err
		}
		resolvedAccountID, err := resolveAccountID()
		if err != nil {
			return err
		}

		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		return runWorkerLogsRecent(resolvedToken, resolvedAccountID, name)
	}

	workersCmd := &cobra.Command{
		Use:   "workers",
		Short: "Manage deployed Workers and their observability",
	}

	workersListCmd := &cobra.Command{
		Use:   "list [filter]",
		Short: "List deployed Workers for the current account",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}
			resolvedAccountID, err := resolveAccountID()
			if err != nil {
				return err
			}

			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}

			workers, err := listWorkers(resolvedToken, resolvedAccountID)
			if err != nil {
				return err
			}
			printWorkers(workers, filter)
			return nil
		},
	}
	workersListCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	workersListCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")

	workerLogsCmd := &cobra.Command{
		Use:    "worker:logs [worker]",
		Short:  "Show recent persisted logs for a deployed Worker",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return workerLogsRun(args)
		},
	}
	workerLogsCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	workerLogsCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	workerLogsCmd.Flags().StringVar(&workersSince, "since", "1h", "Look back window, for example 30m, 2h, or 24h")
	workerLogsCmd.Flags().IntVar(&workersLimit, "limit", 20, "Maximum number of recent log entries or invocation groups to print")
	workerLogsCmd.Flags().StringVar(&workersView, "view", "events", "Observability view to query: events or invocations")
	workerLogsCmd.Flags().StringVar(&workersSearch, "search", "", "Optional substring filter against log messages")

	workerLogsNestedCmd := &cobra.Command{
		Use:              "logs [worker]",
		Short:            "Show recent persisted logs for a deployed Worker",
		TraverseChildren: true,
		Args:             cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return workerLogsRun(args)
		},
	}
	workerLogsNestedCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	workerLogsNestedCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	workerLogsNestedCmd.Flags().StringVar(&workersSince, "since", "10m", "Look back window, for example 5m, 10m, 30m, 2h, or 24h")
	workerLogsNestedCmd.Flags().IntVar(&workersLimit, "limit", 50, "Maximum number of recent log entries or invocation groups to print")
	workerLogsNestedCmd.Flags().StringVar(&workersView, "view", "events", "Observability view to query: events or invocations")
	workerLogsNestedCmd.Flags().StringVar(&workersSearch, "search", "", "Optional substring filter against log messages")

	workerLogsRecentCmd := &cobra.Command{
		Use:   "recent [worker]",
		Short: "Show recent persisted logs for a deployed Worker",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return workerLogsRun(args)
		},
	}
	workerLogsRecentCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	workerLogsRecentCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	workerLogsRecentCmd.Flags().StringVar(&workersSince, "since", "10m", "Look back window, for example 5m, 10m, 30m, 2h, or 24h")
	workerLogsRecentCmd.Flags().IntVar(&workersLimit, "limit", 50, "Maximum number of recent log entries or invocation groups to print")
	workerLogsRecentCmd.Flags().StringVar(&workersView, "view", "events", "Observability view to query: events or invocations")
	workerLogsRecentCmd.Flags().StringVar(&workersSearch, "search", "", "Optional substring filter against log messages")

	workerLogsEnableCmd := &cobra.Command{
		Use:   "enable [worker]",
		Short: "Enable persisted Worker logs and invocation logs for a Worker",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}
			resolvedAccountID, err := resolveAccountID()
			if err != nil {
				return err
			}

			name := ""
			if len(args) == 1 {
				name = args[0]
			}

			runEnable := func(token string) (*workerScript, *workerScriptSettings, error) {
				workers, err := listWorkers(token, resolvedAccountID)
				if err != nil {
					return nil, nil, err
				}
				worker, err := selectWorker(workers, name)
				if err != nil {
					return nil, nil, err
				}
				updated, err := enableWorkerLogs(
					token,
					resolvedAccountID,
					worker.ID,
					workerLogsPersist,
					workerLogsInvocation,
					workerLogsSampleRate,
					workerLogsEnableLogpush,
				)
				if err != nil {
					return nil, nil, err
				}
				return worker, updated, nil
			}

			worker, updated, err := runEnable(resolvedToken)
			if err != nil {
				fallbackToken, mintErr := mintTemporaryPresetToken("workers-logs-admin", resolvedAccountID)
				if mintErr == nil {
					worker, updated, err = runEnable(fallbackToken)
				}
				if err != nil {
					return err
				}
			}

			fmt.Printf("✅ Enabled logs for %s\n", worker.ID)
			fmt.Printf("observability=%t persist=%t invocation_logs=%t sample=%.2f logpush=%t\n",
				updated.Observability.Enabled,
				updated.Observability.Logs.Persist,
				updated.Observability.Logs.InvocationLogs,
				updated.Observability.Logs.HeadSampling,
				updated.Logpush,
			)
			return nil
		},
	}
	workerLogsEnableCmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	workerLogsEnableCmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	workerLogsEnableCmd.Flags().BoolVar(&workerLogsPersist, "persist", true, "Enable persisted Workers Logs")
	workerLogsEnableCmd.Flags().BoolVar(&workerLogsInvocation, "invocations", true, "Enable invocation logs")
	workerLogsEnableCmd.Flags().Float64Var(&workerLogsSampleRate, "sample", 1, "Head sampling rate from 0 to 1")
	workerLogsEnableCmd.Flags().BoolVar(&workerLogsEnableLogpush, "logpush", false, "Also enable the Worker-level logpush flag so account-level Workers Logpush jobs can export this Worker")

	workerLogSinkCmd := &cobra.Command{
		Use:   "sink",
		Short: "Manage Workers log export sinks",
	}

	workerLogSinkSetupR2Cmd := &cobra.Command{
		Use:   "setup-r2 [worker]",
		Short: "Create or verify an R2 Logpush job for Workers trace events and enable logpush for the target Worker",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedToken, err := resolveAPIToken()
			if err != nil {
				return err
			}
			resolvedAccountID, err := resolveAccountID()
			if err != nil {
				return err
			}

			name := ""
			if len(args) == 1 {
				name = args[0]
			}

			runSetup := func(token string) (*workerScript, *logpushJob, error) {
				workers, err := listWorkers(token, resolvedAccountID)
				if err != nil {
					return nil, nil, err
				}
				worker, err := selectWorker(workers, name)
				if err != nil {
					return nil, nil, err
				}
				job, err := ensureWorkerLogpushR2Job(token, resolvedAccountID, worker.ID)
				if err != nil {
					return nil, nil, err
				}
				if _, err := enableWorkerLogs(token, resolvedAccountID, worker.ID, true, true, 1, true); err != nil {
					return nil, nil, err
				}
				return worker, job, nil
			}

			worker, job, err := runSetup(resolvedToken)
			if err != nil {
				fallbackToken, mintErr := mintTemporaryPresetToken("workers-r2-logpush-admin", resolvedAccountID)
				if mintErr == nil {
					worker, job, err = runSetup(fallbackToken)
				}
				if err != nil {
					return err
				}
			}

			fmt.Printf("✅ Workers Logpush R2 sink ready for %s\n", worker.ID)
			fmt.Printf("job_id=%d enabled=%t interval=%ds records=%d bytes=%d\n",
				job.ID,
				job.Enabled,
				job.MaxUploadIntervalSeconds,
				job.MaxUploadRecords,
				job.MaxUploadBytes,
			)
			fmt.Printf("destination=%s\n", redactDestinationConf(job.DestinationConf))
			return nil
		},
	}
	workerLogSinkSetupR2Cmd.Flags().StringVar(&apiToken, "api-token", "", "Cloudflare API token")
	workerLogSinkSetupR2Cmd.Flags().StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	workerLogSinkSetupR2Cmd.Flags().StringVar(&workerR2Bucket, "bucket", "", "R2 bucket for Workers trace event logs")
	workerLogSinkSetupR2Cmd.Flags().StringVar(&workerR2Path, "path", "", "R2 path prefix, for example workers-trace-events")
	workerLogSinkSetupR2Cmd.Flags().StringVar(&workerR2AccessKeyID, "r2-access-key-id", "", "R2 access key ID")
	workerLogSinkSetupR2Cmd.Flags().StringVar(&workerR2SecretAccessKey, "r2-secret-access-key", "", "R2 secret access key")
	workerLogSinkSetupR2Cmd.Flags().StringVar(&workerLogpushName, "job-name", "", "Optional Logpush job name")
	workerLogSinkSetupR2Cmd.Flags().Float64Var(&workerLogpushSampleRate, "sample", 1, "Logpush sampling rate from 0 to 1")
	workerLogSinkSetupR2Cmd.Flags().IntVar(&workerLogpushMaxUploadInterval, "max-upload-interval", 30, "Max upload interval in seconds for Workers Logpush batches")
	workerLogSinkSetupR2Cmd.Flags().IntVar(&workerLogpushMaxUploadRecords, "max-upload-records", 1000, "Max records per Workers Logpush batch")
	workerLogSinkSetupR2Cmd.Flags().IntVar(&workerLogpushMaxUploadBytes, "max-upload-bytes", 5000000, "Max uncompressed bytes per Workers Logpush batch")

	workerLogsNestedCmd.AddCommand(workerLogsRecentCmd)
	workerLogsNestedCmd.AddCommand(workerLogsEnableCmd)
	workerLogSinkCmd.AddCommand(workerLogSinkSetupR2Cmd)
	workerLogsNestedCmd.AddCommand(workerLogSinkCmd)
	workersCmd.AddCommand(workersListCmd)
	workersCmd.AddCommand(workerLogsNestedCmd)
	r2BucketCmd.AddCommand(r2BucketCreateCmd)
	r2CredsCmd.AddCommand(r2CredsMintCmd)
	r2LogpushCmd.AddCommand(r2LogpushBootstrapCmd)
	r2Cmd.AddCommand(r2BucketCmd)
	r2Cmd.AddCommand(r2CredsCmd)
	r2Cmd.AddCommand(r2LogpushCmd)
	dnsCmd.AddCommand(updateCmd)
	dnsCmd.AddCommand(setCmd)
	dnsCmd.AddCommand(listCmd)
	dnsCmd.AddCommand(getCmd)
	dnsCmd.AddCommand(deleteCmd)
	dnsCmd.AddCommand(aCmd)
	dnsCmd.AddCommand(aaaaCmd)
	dnsCmd.AddCommand(cnameCmd)
	dnsCmd.AddCommand(txtCmd)
	dnsCmd.AddCommand(mxCmd)
	tokensCmd.AddCommand(mintCmd)
	tokensCmd.AddCommand(mintGenericCmd)
	tokensCmd.AddCommand(tokensPermissionsCmd)

	rootCmd.AddCommand(dnsCmd)
	rootCmd.AddCommand(tokensCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(skillCmd)
	rootCmd.AddCommand(profilesCmd)
	rootCmd.AddCommand(wranglerRootCmd)
	rootCmd.AddCommand(workersCmd)
	rootCmd.AddCommand(workerLogsCmd)
	rootCmd.AddCommand(r2Cmd)

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
			return nil, errors.New("MX records require --priority or the DNS command syntax: cf dns mx <key> <priority> <mail-server>")
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
	case "workers-logs-read":
		return []string{
			"Workers Scripts Read",
			"Workers Observability Read",
			"Workers Observability Telemetry Write",
		}, "account", nil
	case "workers-logs-admin":
		return []string{
			"Workers Scripts Read",
			"Workers Scripts Write",
			"Workers Observability Read",
			"Workers Observability Write",
			"Workers Observability Telemetry Write",
			"Logs Write",
		}, "account", nil
	case "r2-admin":
		return []string{
			"Workers R2 Storage Read",
			"Workers R2 Storage Write",
		}, "account", nil
	case "workers-r2-logpush-admin":
		return []string{
			"Workers Scripts Read",
			"Workers Scripts Write",
			"Workers Observability Read",
			"Workers Observability Write",
			"Workers Observability Telemetry Write",
			"Logs Write",
			"Workers R2 Storage Read",
			"Workers R2 Storage Write",
		}, "account", nil
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

func listWorkers(apiToken, accountID string) ([]workerScript, error) {
	client := &http.Client{}
	resp, err := getJSON[[]workerScript](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts", accountID), apiToken)
	if err != nil {
		return nil, err
	}

	sort.Slice(resp.Result, func(i, j int) bool {
		return resp.Result[i].ID < resp.Result[j].ID
	})
	return resp.Result, nil
}

func selectWorker(workers []workerScript, requested string) (*workerScript, error) {
	if len(workers) == 0 {
		return nil, errors.New("no deployed Workers found for this account")
	}
	if requested == "" {
		if len(workers) == 1 {
			return &workers[0], nil
		}
		names := make([]string, 0, len(workers))
		for _, worker := range workers {
			names = append(names, worker.ID)
		}
		return nil, fmt.Errorf("multiple Workers found; specify one explicitly. Available: %s", strings.Join(names, ", "))
	}

	for i := range workers {
		if workers[i].ID == requested {
			return &workers[i], nil
		}
	}

	lowerRequested := strings.ToLower(requested)
	var matched []workerScript
	for _, worker := range workers {
		if strings.EqualFold(worker.ID, requested) || strings.Contains(strings.ToLower(worker.ID), lowerRequested) {
			matched = append(matched, worker)
		}
	}
	if len(matched) == 1 {
		return &matched[0], nil
	}
	if len(matched) > 1 {
		names := make([]string, 0, len(matched))
		for _, worker := range matched {
			names = append(names, worker.ID)
		}
		return nil, fmt.Errorf("worker name %q is ambiguous; matches: %s", requested, strings.Join(names, ", "))
	}

	return nil, fmt.Errorf("worker %q not found; use `cf workers:list` to discover deployed Workers", requested)
}

func runWorkerLogsRecent(apiToken, accountID, name string) error {
	since, err := time.ParseDuration(strings.TrimSpace(workersSince))
	if err != nil || since <= 0 {
		return fmt.Errorf("invalid --since value %q; use a Go duration like 5m, 10m, 30m, 2h, or 24h", workersSince)
	}
	if workersLimit <= 0 {
		return errors.New("--limit must be greater than 0")
	}

	view := strings.ToLower(strings.TrimSpace(workersView))
	if view != "events" && view != "invocations" {
		return fmt.Errorf("unsupported --view %q; use events or invocations", workersView)
	}

	workers, err := listWorkers(apiToken, accountID)
	if err != nil {
		return err
	}

	worker, err := selectWorker(workers, name)
	if err != nil {
		return err
	}

	result, err := queryWorkerLogs(apiToken, accountID, worker.ID, view, since, workersLimit, workersSearch)
	if err != nil {
		return err
	}

	fmt.Printf("Worker: %s\n", worker.ID)
	fmt.Printf("Window: last %s\n", since)
	fmt.Printf("View: %s\n", view)
	if workersSearch != "" {
		fmt.Printf("Search: %s\n", workersSearch)
	}
	if !worker.Observability.Enabled || !worker.Observability.Logs.Persist {
		fmt.Println("Warning: this Worker does not report persisted logs as fully enabled in script metadata, so results may be empty. Run `cf worker logs enable <worker>` first.")
	}

	if view == "invocations" {
		printWorkerInvocations(result.Invocations)
		return nil
	}

	printWorkerEvents(result.Events.Events)
	return nil
}

func queryWorkerLogs(apiToken, accountID, workerName, view string, since time.Duration, limit int, search string) (*telemetryQueryResult, error) {
	now := time.Now().UTC()
	filters := []map[string]any{
		{
			"key":       "$workers.scriptName",
			"operation": "eq",
			"type":      "string",
			"value":     workerName,
		},
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		filters = append(filters, map[string]any{
			"key":       "$metadata.message",
			"operation": "contains",
			"type":      "string",
			"value":     trimmed,
		})
	}

	payload := map[string]any{
		"queryId": "adhoc",
		"timeframe": map[string]int64{
			"from": now.Add(-since).UnixMilli(),
			"to":   now.UnixMilli(),
		},
		"view":  view,
		"limit": limit,
		"parameters": map[string]any{
			"datasets":          []string{"cloudflare-workers"},
			"filterCombination": "and",
			"filters":           filters,
		},
	}

	client := &http.Client{}
	resp, err := postJSON[telemetryQueryResult](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/observability/telemetry/query", accountID), apiToken, payload)
	if err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

func getWorkerSettings(apiToken, accountID, workerName string) (*workerScriptSettings, error) {
	client := &http.Client{}
	resp, err := getJSON[workerScriptSettings](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/settings", accountID, url.PathEscape(workerName)), apiToken)
	if err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

func enableWorkerLogs(apiToken, accountID, workerName string, persist, invocationLogs bool, sampleRate float64, enableLogpush bool) (*workerScriptSettings, error) {
	if sampleRate < 0 || sampleRate > 1 {
		return nil, fmt.Errorf("invalid sample rate %.4f; expected a value between 0 and 1", sampleRate)
	}

	settings := map[string]any{
		"observability": map[string]any{
			"enabled":            true,
			"head_sampling_rate": sampleRate,
			"logs": map[string]any{
				"enabled":            true,
				"persist":            persist,
				"invocation_logs":    invocationLogs,
				"head_sampling_rate": sampleRate,
			},
		},
	}
	if enableLogpush {
		settings["logpush"] = true
	}

	client := &http.Client{}
	resp, err := patchMultipartJSON[workerScriptSettings](
		client,
		fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/settings", accountID, url.PathEscape(workerName)),
		apiToken,
		"settings",
		settings,
	)
	if err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

func listLogpushJobs(apiToken, accountID string) ([]logpushJob, error) {
	client := &http.Client{}
	resp, err := getJSON[[]logpushJob](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/logpush/jobs", accountID), apiToken)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func createLogpushJob(apiToken, accountID string, payload map[string]any) (*logpushJob, error) {
	client := &http.Client{}
	resp, err := postJSON[logpushJob](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/logpush/jobs", accountID), apiToken, payload)
	if err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

func listR2Buckets(apiToken, accountID string) ([]r2Bucket, error) {
	client := &http.Client{}
	resp, err := getJSON[[]r2Bucket](client, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets", accountID), apiToken)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func createR2Bucket(apiToken, accountID, name, jurisdiction, locationHint, storageClass string) (*r2Bucket, error) {
	payload := map[string]any{
		"name": name,
	}
	if trimmed := strings.TrimSpace(locationHint); trimmed != "" {
		payload["locationHint"] = trimmed
	}
	if trimmed := strings.TrimSpace(storageClass); trimmed != "" {
		payload["storageClass"] = trimmed
	}

	req, err := newJSONRequest("POST", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets", accountID), apiToken, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("cf-r2-jurisdiction", normalizeR2Jurisdiction(jurisdiction))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope apiEnvelope[r2Bucket]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Success {
		apiErr := apiErrors(envelope.Errors)
		if hasAPIErrorCode(apiErr, 10004) {
			if buckets, err := listR2Buckets(apiToken, accountID); err == nil {
				for _, bucket := range buckets {
					if bucket.Name == name && bucket.Jurisdiction == normalizeR2Jurisdiction(jurisdiction) {
						return &bucket, nil
					}
				}
			}

			return &r2Bucket{
				Name:         name,
				Jurisdiction: normalizeR2Jurisdiction(jurisdiction),
			}, nil
		}
		return nil, apiErr
	}
	return &envelope.Result, nil
}

func ensureR2Bucket(apiToken, accountID, name, jurisdiction, locationHint, storageClass string) (*r2Bucket, bool, error) {
	buckets, err := listR2Buckets(apiToken, accountID)
	if err == nil {
		for _, bucket := range buckets {
			if bucket.Name == name && bucket.Jurisdiction == normalizeR2Jurisdiction(jurisdiction) {
				return &bucket, false, nil
			}
		}
	}
	created, createErr := createR2Bucket(apiToken, accountID, name, jurisdiction, locationHint, storageClass)
	if createErr != nil {
		return nil, false, createErr
	}
	return created, true, nil
}

func mintPresetToken(bootstrap, tokenName, preset, accountID, zoneID, tokenExpiry string) (*tokenCreateResult, error) {
	permissionNames, presetScope, err := permissionsFromPreset(preset)
	if err != nil {
		return nil, err
	}
	groups, err := resolvePermissionGroupsByName(bootstrap, permissionNames)
	if err != nil {
		return nil, err
	}

	resourceID := ""
	if presetScope == "account" {
		resourceID = accountID
	}
	if presetScope == "zone" {
		resourceID = zoneID
	}
	resources, err := buildResources(presetScope, resourceID, accountID)
	if err != nil {
		return nil, err
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
	if tokenExpiry != "" {
		payload["expires_on"] = tokenExpiry
	}

	return mintGenericToken(bootstrap, genericMintRequest{
		Owner:     "user",
		AccountID: accountID,
		Payload:   payload,
	})
}

func mintR2BucketCredentials(bootstrap, accountID, bucketName, jurisdiction, tokenName, tokenExpiry string) (*tokenCreateResult, string, string, string, error) {
	groups, err := resolvePermissionGroupsByName(bootstrap, []string{
		"Workers R2 Storage Bucket Item Read",
		"Workers R2 Storage Bucket Item Write",
	})
	if err != nil {
		return nil, "", "", "", err
	}

	normalizedJurisdiction := normalizeR2Jurisdiction(jurisdiction)
	resourceKey := fmt.Sprintf("com.cloudflare.edge.r2.bucket.%s_%s_%s", accountID, normalizedJurisdiction, bucketName)
	payload := map[string]any{
		"name": tokenName,
		"policies": []tokenPolicy{
			{
				Effect:           "allow",
				PermissionGroups: groups,
				Resources: map[string]any{
					resourceKey: "*",
				},
			},
		},
	}
	if tokenExpiry != "" {
		payload["expires_on"] = tokenExpiry
	}

	token, err := mintGenericToken(bootstrap, genericMintRequest{
		Owner:   "user",
		Payload: payload,
	})
	if err != nil {
		return nil, "", "", "", err
	}

	secretHash := sha256.Sum256([]byte(token.Value))
	secretAccessKey := fmt.Sprintf("%x", secretHash[:])
	return token, token.ID, secretAccessKey, r2EndpointForAccount(accountID, normalizedJurisdiction), nil
}

func storeR2LogpushSecrets(bucketName, accessKeyID, secretAccessKey, path string) error {
	if err := writeSecretToKeychain(defaultWorkerR2BucketServiceName(), bucketName); err != nil {
		return err
	}
	if err := writeSecretToKeychain(defaultWorkerR2AccessKeyIDServiceName(), accessKeyID); err != nil {
		return err
	}
	if err := writeSecretToKeychain(defaultWorkerR2SecretAccessKeyServiceName(), secretAccessKey); err != nil {
		return err
	}
	if path != "" {
		if err := writeSecretToKeychain(defaultWorkerR2PathServiceName(), path); err != nil {
			return err
		}
	}
	return nil
}

func mintTemporaryPresetToken(preset, accountID string) (string, error) {
	bootstrap, err := resolveBootstrapToken()
	if err != nil {
		return "", err
	}

	token, err := mintPresetToken(
		bootstrap,
		fmt.Sprintf("%s %s %s", profile, preset, time.Now().UTC().Format("20060102-150405")),
		preset,
		accountID,
		"",
		expiresOn,
	)
	if err != nil {
		return "", err
	}
	return token.Value, nil
}

func ensureWorkerLogpushR2Job(apiToken, accountID, workerName string) (*logpushJob, error) {
	bucket, err := resolveWorkerR2Bucket()
	if err != nil {
		return nil, err
	}
	accessKeyID, err := resolveWorkerR2AccessKeyID()
	if err != nil {
		return nil, err
	}
	secretAccessKey, err := resolveWorkerR2SecretAccessKey()
	if err != nil {
		return nil, err
	}
	pathPrefix, err := resolveWorkerR2Path(workerName)
	if err != nil {
		return nil, err
	}
	if workerLogpushSampleRate < 0 || workerLogpushSampleRate > 1 {
		return nil, fmt.Errorf("invalid logpush sample rate %.4f; expected a value between 0 and 1", workerLogpushSampleRate)
	}
	if workerLogpushMaxUploadInterval != 0 && (workerLogpushMaxUploadInterval < 30 || workerLogpushMaxUploadInterval > 300) {
		return nil, fmt.Errorf("--max-upload-interval must be 0 or between 30 and 300 seconds")
	}
	if workerLogpushMaxUploadRecords != 0 && (workerLogpushMaxUploadRecords < 1000 || workerLogpushMaxUploadRecords > 1000000) {
		return nil, fmt.Errorf("--max-upload-records must be 0 or between 1000 and 1000000")
	}
	if workerLogpushMaxUploadBytes != 0 && (workerLogpushMaxUploadBytes < 5000000 || workerLogpushMaxUploadBytes > 1000000000) {
		return nil, fmt.Errorf("--max-upload-bytes must be 0 or between 5000000 and 1000000000")
	}

	destination := fmt.Sprintf(
		"r2://%s/%s/{DATE}?account-id=%s&access-key-id=%s&secret-access-key=%s",
		strings.TrimSpace(bucket),
		strings.Trim(pathPrefix, "/"),
		accountID,
		url.QueryEscape(strings.TrimSpace(accessKeyID)),
		url.QueryEscape(strings.TrimSpace(secretAccessKey)),
	)

	jobs, err := listLogpushJobs(apiToken, accountID)
	if err != nil {
		return nil, explainLogpushAccessError(err)
	}
	for _, job := range jobs {
		if job.Dataset != "workers_trace_events" {
			continue
		}
		if strings.HasPrefix(job.DestinationConf, fmt.Sprintf("r2://%s/%s/", strings.TrimSpace(bucket), strings.Trim(pathPrefix, "/"))) ||
			strings.HasPrefix(job.DestinationConf, fmt.Sprintf("r2://%s/%s?", strings.TrimSpace(bucket), strings.Trim(pathPrefix, "/"))) {
			return &job, nil
		}
	}

	jobName := strings.TrimSpace(workerLogpushName)
	if jobName == "" {
		jobName = fmt.Sprintf("%s workers trace events", profile)
	}

	payload := map[string]any{
		"name":             jobName,
		"destination_conf": destination,
		"dataset":          "workers_trace_events",
		"enabled":          true,
		"output_options": map[string]any{
			"field_names": []string{
				"Event",
				"EventTimestampMs",
				"Outcome",
				"Exceptions",
				"Logs",
				"ScriptName",
				"ScriptVersion",
				"CPUTimeMs",
				"WallTimeMs",
			},
			"output_type":      "ndjson",
			"timestamp_format": "rfc3339",
			"sample_rate":      workerLogpushSampleRate,
		},
		"max_upload_interval_seconds": workerLogpushMaxUploadInterval,
		"max_upload_records":          workerLogpushMaxUploadRecords,
		"max_upload_bytes":            workerLogpushMaxUploadBytes,
	}

	job, err := createLogpushJob(apiToken, accountID, payload)
	if err != nil {
		return nil, explainLogpushAccessError(err)
	}
	return job, nil
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

func postJSON[T any](client *http.Client, requestURL, apiToken string, payload any) (*apiEnvelope[T], error) {
	req, err := newJSONRequest("POST", requestURL, apiToken, payload)
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

func patchMultipartJSON[T any](client *http.Client, requestURL, apiToken, fieldName string, payload any) (*apiEnvelope[T], error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fieldName+".json")
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(part).Encode(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+apiToken)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope apiEnvelope[T]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
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

	return cloudflareAPIError{Messages: errs}
}

func hasAPIErrorCode(err error, code int) bool {
	var apiErr cloudflareAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, msg := range apiErr.Messages {
		if msg.Code == code {
			return true
		}
	}
	return false
}

func explainLogpushAccessError(err error) error {
	if !hasAPIErrorCode(err, 10000) {
		return err
	}
	return fmt.Errorf(
		"%w. Cloudflare accepted the token format but rejected account-level Logpush access. This account likely needs Logpush configuration access in the Cloudflare dashboard (Administrator, Super Administrator, or Log Share edit role) in addition to an account-scoped token with Logs Write",
		err,
	)
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
	return ""
}

func commandNeedsExplicitProfile(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if strings.HasPrefix(cmd.CommandPath(), "cf profiles") || strings.HasPrefix(cmd.CommandPath(), "cf wrangler") || cmd.CommandPath() == "cf skill" {
		return false
	}
	if cmd.Name() == "help" || cmd.Name() == "completion" {
		return false
	}
	if cmd.Parent() == nil && !cmd.Flags().Changed("profile") && len(os.Args) <= 1 {
		return false
	}
	helpRequested, err := cmd.Flags().GetBool("help")
	if err == nil && helpRequested {
		return false
	}
	return true
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

func defaultWorkerR2BucketServiceName() string {
	return fmt.Sprintf("%s cloudflare r2 log bucket", profile)
}

func defaultWorkerR2AccessKeyIDServiceName() string {
	return fmt.Sprintf("%s cloudflare r2 access key id", profile)
}

func defaultWorkerR2SecretAccessKeyServiceName() string {
	return fmt.Sprintf("%s cloudflare r2 secret access key", profile)
}

func defaultWorkerR2PathServiceName() string {
	return fmt.Sprintf("%s cloudflare workers logpush path", profile)
}

func normalizeR2Jurisdiction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default":
		return "default"
	case "eu":
		return "eu"
	case "fedramp":
		return "fedramp"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func r2EndpointForAccount(accountID, jurisdiction string) string {
	switch normalizeR2Jurisdiction(jurisdiction) {
	case "eu":
		return fmt.Sprintf("https://%s.eu.r2.cloudflarestorage.com", accountID)
	case "fedramp":
		return fmt.Sprintf("https://%s.fedramp.r2.cloudflarestorage.com", accountID)
	default:
		return fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	}
}

func defaultNamedTokenServiceName(tokenName string) string {
	return fmt.Sprintf("%s cloudflare token %s", profile, slugify(tokenName))
}

func cfCLIConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".cf-cli"), nil
}

func legacyCodexConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".gg", "codex"), nil
}

func wranglerAuthDir() (string, error) {
	baseDir, err := cfCLIConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "wrangler-auth"), nil
}

func legacyWranglerAuthDir() (string, error) {
	baseDir, err := legacyCodexConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "wrangler-auth"), nil
}

func wranglerAccountsDir() (string, error) {
	baseDir, err := wranglerAuthDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "accounts"), nil
}

func wranglerAccountsDBPath() (string, error) {
	baseDir, err := wranglerAuthDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "accounts.json"), nil
}

func wranglerConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "Library", "Preferences", ".wrangler", "config", "default.toml"), nil
}

func ensureWranglerAuthDirs() error {
	if err := migrateWranglerAuthState(); err != nil {
		return err
	}
	baseDir, err := wranglerAuthDir()
	if err != nil {
		return err
	}
	accountsDir, err := wranglerAccountsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(accountsDir, 0o700); err != nil {
		return err
	}
	return chmodRecursive(baseDir, 0o700, 0o600)
}

func loadWranglerAccountsDB() (*wranglerAccountsDB, error) {
	if err := migrateWranglerAuthState(); err != nil {
		return nil, err
	}
	path, err := wranglerAccountsDBPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &wranglerAccountsDB{Accounts: []wranglerAccount{}}, nil
		}
		return nil, err
	}
	var db wranglerAccountsDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, err
	}
	if db.Accounts == nil {
		db.Accounts = []wranglerAccount{}
	}
	return &db, nil
}

func saveWranglerAccountsDB(db *wranglerAccountsDB) error {
	if err := ensureWranglerAuthDirs(); err != nil {
		return err
	}
	path, err := wranglerAccountsDBPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func (db *wranglerAccountsDB) addAccount(account wranglerAccount) {
	for i, existing := range db.Accounts {
		if existing.ID == account.ID {
			db.Accounts[i] = account
			return
		}
	}
	db.Accounts = append(db.Accounts, account)
	sort.Slice(db.Accounts, func(i, j int) bool {
		return strings.ToLower(db.Accounts[i].Name) < strings.ToLower(db.Accounts[j].Name)
	})
}

func (db *wranglerAccountsDB) getAccount(id string) *wranglerAccount {
	for i := range db.Accounts {
		if db.Accounts[i].ID == id {
			return &db.Accounts[i]
		}
	}
	return nil
}

func (db *wranglerAccountsDB) findAccount(query string) (*wranglerAccount, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, errors.New("wrangler account query is required")
	}
	for i := range db.Accounts {
		if db.Accounts[i].ID == trimmed || strings.EqualFold(db.Accounts[i].Name, trimmed) || strings.EqualFold(db.Accounts[i].Email, trimmed) {
			return &db.Accounts[i], nil
		}
	}
	lowerQuery := strings.ToLower(trimmed)
	var matches []*wranglerAccount
	for i := range db.Accounts {
		account := &db.Accounts[i]
		if strings.Contains(strings.ToLower(account.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(account.Email), lowerQuery) ||
			strings.Contains(strings.ToLower(account.ID), lowerQuery) {
			matches = append(matches, account)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, fmt.Sprintf("%s (%s)", match.Name, match.ID))
		}
		return nil, fmt.Errorf("wrangler account query %q is ambiguous: %s", trimmed, strings.Join(names, ", "))
	}
	return nil, fmt.Errorf("no saved Wrangler account matches %q", trimmed)
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:]), nil
}

func currentWranglerConfigHash() (string, error) {
	path, err := wranglerConfigPath()
	if err != nil {
		return "", err
	}
	return hashFile(path)
}

func wranglerAccountConfigSnapshotPath(accountID string) (string, error) {
	accountsDir, err := wranglerAccountsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(accountsDir, accountID+".toml"), nil
}

func saveWranglerAccountConfig(accountID string) (string, error) {
	if err := ensureWranglerAuthDirs(); err != nil {
		return "", err
	}
	srcPath, err := wranglerConfigPath()
	if err != nil {
		return "", err
	}
	dstPath, err := wranglerAccountConfigSnapshotPath(accountID)
	if err != nil {
		return "", err
	}
	if err := copyFileWithMode(srcPath, dstPath, 0o600); err != nil {
		return "", err
	}
	return hashFile(srcPath)
}

func saveWranglerAccountConfigIfChanged(accountID, currentHash string) (bool, string, error) {
	newHash, err := currentWranglerConfigHash()
	if err != nil {
		return false, "", err
	}
	if newHash == currentHash {
		return false, currentHash, nil
	}
	_, err = saveWranglerAccountConfig(accountID)
	if err != nil {
		return false, "", err
	}
	return true, newHash, nil
}

func restoreWranglerAccountConfig(accountID string) error {
	srcPath, err := wranglerAccountConfigSnapshotPath(accountID)
	if err != nil {
		return err
	}
	dstPath, err := wranglerConfigPath()
	if err != nil {
		return err
	}
	return copyFileWithMode(srcPath, dstPath, 0o600)
}

func resolveWranglerCommand() string {
	if strings.TrimSpace(wranglerCmd) != "" {
		return normalizeWranglerCommand(strings.TrimSpace(wranglerCmd))
	}
	if env := os.Getenv("CF_WRANGLER_CMD"); env != "" {
		return normalizeWranglerCommand(strings.TrimSpace(env))
	}
	if env := os.Getenv("CLOUDFLARE_WRANGLER_CMD"); env != "" {
		return normalizeWranglerCommand(strings.TrimSpace(env))
	}
	return ""
}

func normalizeWranglerCommand(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "npx wrangler" {
		return "npx --yes wrangler"
	}
	return trimmed
}

func tryWranglerCommand(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	parts := strings.Fields(command)
	args := append(parts[1:], "--version")
	cmd := exec.Command(parts[0], args...)
	return cmd.Run() == nil
}

func detectWranglerCommand() string {
	candidates := []string{"wrangler", "npx --yes wrangler"}
	for _, candidate := range candidates {
		if tryWranglerCommand(candidate) {
			return candidate
		}
	}
	return ""
}

func ensureWranglerCommand(db *wranglerAccountsDB) (string, error) {
	if resolved := resolveWranglerCommand(); resolved != "" {
		if !tryWranglerCommand(resolved) {
			return "", fmt.Errorf("configured Wrangler command %q is not runnable", resolved)
		}
		db.WranglerCmd = resolved
		_ = saveWranglerAccountsDB(db)
		return resolved, nil
	}
	if strings.TrimSpace(db.WranglerCmd) != "" && tryWranglerCommand(db.WranglerCmd) {
		normalized := normalizeWranglerCommand(db.WranglerCmd)
		if normalized != db.WranglerCmd {
			db.WranglerCmd = normalized
			_ = saveWranglerAccountsDB(db)
		}
		return normalized, nil
	}
	detected := detectWranglerCommand()
	if detected == "" {
		return "", errors.New("wrangler command not found; install Wrangler or set CF_WRANGLER_CMD to something like 'wrangler' or 'npx wrangler'")
	}
	db.WranglerCmd = detected
	_ = saveWranglerAccountsDB(db)
	return detected, nil
}

func runWranglerCommand(command string, args ...string) ([]byte, error) {
	return runWranglerCommandWithTimeout(0, command, args...)
}

func runWranglerCommandWithTimeout(timeout time.Duration, command string, args ...string) ([]byte, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, errors.New("wrangler command is empty")
	}
	var cmd *exec.Cmd
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd = exec.CommandContext(ctx, parts[0], append(parts[1:], args...)...)
	} else {
		cmd = exec.Command(parts[0], append(parts[1:], args...)...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
			return output, fmt.Errorf("%s %s timed out after %s; run `%s whoami` manually to confirm Wrangler is responsive, or set CF_WRANGLER_CMD to a working command", command, strings.Join(args, " "), timeout, command)
		}
		return output, fmt.Errorf("failed to run %s %s: %w\nOutput: %s", command, strings.Join(args, " "), err, string(output))
	}
	return output, nil
}

func wranglerWhoami(command string) (*wranglerWhoamiInfo, error) {
	output, err := runWranglerCommandWithTimeout(20*time.Second, command, "whoami")
	if err != nil {
		return nil, err
	}
	return parseWranglerWhoamiOutput(string(output))
}

func parseWranglerWhoamiOutput(output string) (*wranglerWhoamiInfo, error) {
	info := &wranglerWhoamiInfo{}
	emailRegex := regexp.MustCompile(`associated with the email (\S+)`)
	if match := emailRegex.FindStringSubmatch(output); len(match) > 1 {
		info.Email = strings.TrimSuffix(match[1], ".")
	}
	rowRegex := regexp.MustCompile(`│\s*([^│]+?)\s*│\s*([^│]+?)\s*│`)
	matches := rowRegex.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		name := strings.TrimSpace(match[1])
		id := strings.TrimSpace(match[2])
		if name == "Account Name" && id == "Account ID" {
			continue
		}
		if name != "" && id != "" {
			info.AccountName = name
			info.AccountID = id
			break
		}
	}
	if info.AccountID == "" {
		return nil, errors.New("could not parse account ID from `wrangler whoami` output")
	}
	return info, nil
}

func runWranglerLogin(command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return errors.New("wrangler command is empty")
	}
	cmd := exec.Command(parts[0], append(parts[1:], "login")...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func profileRegistryPath() (string, error) {
	baseDir, err := cfCLIConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "cloudflare-profiles.json"), nil
}

func legacyProfileRegistryPath() (string, error) {
	baseDir, err := legacyCodexConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "cloudflare-profiles.json"), nil
}

func readProfileRegistry() (*profileRegistry, error) {
	if err := migrateProfileRegistry(); err != nil {
		return nil, err
	}
	path, err := profileRegistryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &profileRegistry{}, nil
		}
		return nil, err
	}

	var registry profileRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}
	return &registry, nil
}

func writeProfileRegistry(registry *profileRegistry) error {
	if err := migrateProfileRegistry(); err != nil {
		return err
	}
	path, err := profileRegistryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func registerProfile(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	registry, err := readProfileRegistry()
	if err != nil {
		return err
	}
	for _, existing := range registry.Profiles {
		if existing == trimmed {
			return nil
		}
	}
	registry.Profiles = append(registry.Profiles, trimmed)
	sort.Strings(registry.Profiles)
	return writeProfileRegistry(registry)
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

func resolveWorkerR2Bucket() (string, error) {
	if strings.TrimSpace(workerR2Bucket) != "" {
		return strings.TrimSpace(workerR2Bucket), nil
	}
	if env := os.Getenv("CF_R2_LOG_BUCKET"); env != "" {
		return strings.TrimSpace(env), nil
	}
	if env := os.Getenv("CLOUDFLARE_R2_LOG_BUCKET"); env != "" {
		return strings.TrimSpace(env), nil
	}

	service := defaultWorkerR2BucketServiceName()
	value, err := readSecretFromKeychain(service)
	if err == nil && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), nil
	}

	return "", fmt.Errorf("workers log R2 bucket not provided; set CF_R2_LOG_BUCKET/CLOUDFLARE_R2_LOG_BUCKET or store it in keychain service %q", service)
}

func resolveWorkerR2AccessKeyID() (string, error) {
	if strings.TrimSpace(workerR2AccessKeyID) != "" {
		return strings.TrimSpace(workerR2AccessKeyID), nil
	}
	if env := os.Getenv("CF_R2_ACCESS_KEY_ID"); env != "" {
		return strings.TrimSpace(env), nil
	}
	if env := os.Getenv("AWS_ACCESS_KEY_ID"); env != "" {
		return strings.TrimSpace(env), nil
	}

	service := defaultWorkerR2AccessKeyIDServiceName()
	value, err := readSecretFromKeychain(service)
	if err == nil && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), nil
	}

	return "", fmt.Errorf("workers log R2 access key ID not provided; set CF_R2_ACCESS_KEY_ID/AWS_ACCESS_KEY_ID or store it in keychain service %q", service)
}

func resolveWorkerR2SecretAccessKey() (string, error) {
	if strings.TrimSpace(workerR2SecretAccessKey) != "" {
		return strings.TrimSpace(workerR2SecretAccessKey), nil
	}
	if env := os.Getenv("CF_R2_SECRET_ACCESS_KEY"); env != "" {
		return strings.TrimSpace(env), nil
	}
	if env := os.Getenv("AWS_SECRET_ACCESS_KEY"); env != "" {
		return strings.TrimSpace(env), nil
	}

	service := defaultWorkerR2SecretAccessKeyServiceName()
	value, err := readSecretFromKeychain(service)
	if err == nil && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), nil
	}

	return "", fmt.Errorf("workers log R2 secret access key not provided; set CF_R2_SECRET_ACCESS_KEY/AWS_SECRET_ACCESS_KEY or store it in keychain service %q", service)
}

func resolveWorkerR2Path(workerName string) (string, error) {
	if strings.TrimSpace(workerR2Path) != "" {
		return strings.Trim(strings.TrimSpace(workerR2Path), "/"), nil
	}
	if env := os.Getenv("CF_WORKERS_LOGPUSH_PATH"); env != "" {
		return strings.Trim(strings.TrimSpace(env), "/"), nil
	}
	if env := os.Getenv("CLOUDFLARE_WORKERS_LOGPUSH_PATH"); env != "" {
		return strings.Trim(strings.TrimSpace(env), "/"), nil
	}

	service := defaultWorkerR2PathServiceName()
	value, err := readSecretFromKeychain(service)
	if err == nil && strings.TrimSpace(value) != "" {
		return strings.Trim(strings.TrimSpace(value), "/"), nil
	}

	return fmt.Sprintf("workers-trace-events/%s", slugify(workerName)), nil
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

func printWorkers(workers []workerScript, filter string) {
	filter = strings.ToLower(strings.TrimSpace(filter))
	filtered := make([]workerScript, 0, len(workers))
	for _, worker := range workers {
		if filter == "" {
			filtered = append(filtered, worker)
			continue
		}

		routeText := make([]string, 0, len(worker.Routes))
		for _, route := range worker.Routes {
			routeText = append(routeText, route.Pattern)
		}
		searchable := strings.ToLower(worker.ID + " " + strings.Join(routeText, " "))
		if strings.Contains(searchable, filter) {
			filtered = append(filtered, worker)
		}
	}

	if len(filtered) == 0 {
		fmt.Println("No Workers found.")
		return
	}

	for _, worker := range filtered {
		routes := make([]string, 0, len(worker.Routes))
		for _, route := range worker.Routes {
			routes = append(routes, route.Pattern)
		}
		routeDisplay := "-"
		if len(routes) > 0 {
			routeDisplay = strings.Join(routes, ",")
		}

		logsState := "disabled"
		if worker.Observability.Enabled {
			logsState = "enabled"
		}
		if worker.Observability.Logs.Persist {
			logsState += "/persisted"
		}
		if worker.Observability.Logs.InvocationLogs {
			logsState += "/invocations"
		}

		fmt.Printf("%s\tmodified=%s\tlogs=%s\tlogpush=%t\troutes=%s\n",
			worker.ID,
			worker.ModifiedOn,
			logsState,
			worker.Logpush,
			routeDisplay,
		)
	}
}

func printWorkerEvents(events []telemetryEvent) {
	if len(events) == 0 {
		fmt.Println("No recent persisted logs found.")
		return
	}

	for _, event := range events {
		when := formatEventTime(event)
		level := firstNonEmpty(
			asString(event.Metadata["level"]),
			asString(event.Metadata["type"]),
			"info",
		)
		message := firstNonEmpty(
			asString(event.Metadata["message"]),
			asString(event.Metadata["error"]),
			asString(event.Metadata["messageTemplate"]),
			"-",
		)
		method := asString(event.Metadata["trigger"])
		if strings.EqualFold(method, "http") {
			method = asString(event.Workers["eventType"])
		}
		status := asString(event.Metadata["statusCode"])
		urlValue := asString(event.Metadata["url"])
		outcome := asString(event.Workers["outcome"])
		requestID := firstNonEmpty(asString(event.Workers["requestId"]), asString(event.Metadata["requestId"]))

		line := fmt.Sprintf("%s\t%s\t%s", when, strings.ToUpper(level), message)
		if method != "" || urlValue != "" || status != "" {
			line += fmt.Sprintf("\trequest=%s %s status=%s", emptyDash(method), emptyDash(urlValue), emptyDash(status))
		}
		if outcome != "" {
			line += fmt.Sprintf("\toutcome=%s", outcome)
		}
		if requestID != "" {
			line += fmt.Sprintf("\trequest_id=%s", requestID)
		}
		fmt.Println(line)
	}
}

func printWorkerInvocations(invocations map[string][]telemetryEvent) {
	if len(invocations) == 0 {
		fmt.Println("No recent persisted logs found.")
		return
	}

	keys := make([]string, 0, len(invocations))
	for key := range invocations {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		events := invocations[key]
		if len(events) == 0 {
			continue
		}

		primary := events[0]
		when := formatEventTime(primary)
		status := asString(primary.Metadata["statusCode"])
		urlValue := asString(primary.Metadata["url"])
		outcome := asString(primary.Workers["outcome"])
		requestID := firstNonEmpty(asString(primary.Workers["requestId"]), asString(primary.Metadata["requestId"]), key)

		fmt.Printf("%s\tinvocation=%s\tstatus=%s\toutcome=%s\turl=%s\tevents=%d\n",
			when,
			requestID,
			emptyDash(status),
			emptyDash(outcome),
			emptyDash(urlValue),
			len(events),
		)

		for _, event := range events {
			level := firstNonEmpty(
				asString(event.Metadata["level"]),
				asString(event.Metadata["type"]),
				"info",
			)
			message := firstNonEmpty(
				asString(event.Metadata["message"]),
				asString(event.Metadata["error"]),
				asString(event.Metadata["messageTemplate"]),
				"-",
			)
			fmt.Printf("  %s %s\n", strings.ToUpper(level), message)
		}
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

func copyFile(src, dst string) error {
	return copyFileWithMode(src, dst, 0o644)
}

func copyFileWithMode(src, dst string, mode fs.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func migrateWranglerAuthState() error {
	srcDir, err := legacyWranglerAuthDir()
	if err != nil {
		return err
	}
	dstDir, err := wranglerAuthDir()
	if err != nil {
		return err
	}
	if err := migrateDirectoryContents(srcDir, dstDir, 0o700, 0o600); err != nil {
		return err
	}
	if _, err := os.Stat(dstDir); err == nil {
		return chmodRecursive(dstDir, 0o700, 0o600)
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	} else {
		return err
	}
}

func migrateProfileRegistry() error {
	srcPath, err := legacyProfileRegistryPath()
	if err != nil {
		return err
	}
	dstPath, err := profileRegistryPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dstPath); err == nil {
		return os.Chmod(dstPath, 0o600)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(srcPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return copyFileWithMode(srcPath, dstPath, 0o600)
}

func migrateDirectoryContents(srcDir, dstDir string, dirMode, fileMode fs.FileMode) error {
	if _, err := os.Stat(srcDir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstDir, relPath)
		if d.IsDir() {
			return os.MkdirAll(targetPath, dirMode)
		}
		if _, err := os.Stat(targetPath); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return copyFileWithMode(path, targetPath, fileMode)
	})
}

func chmodRecursive(root string, dirMode, fileMode fs.FileMode) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(path, dirMode)
		}
		return os.Chmod(path, fileMode)
	})
}

func formatEventTime(event telemetryEvent) string {
	if event.Timestamp > 0 {
		return time.UnixMilli(event.Timestamp).Format(time.RFC3339)
	}
	if start := asInt64(event.Metadata["startTime"]); start > 0 {
		return time.UnixMilli(start).Format(time.RFC3339)
	}
	if end := asInt64(event.Metadata["endTime"]); end > 0 {
		return time.UnixMilli(end).Format(time.RFC3339)
	}
	return "-"
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case float32:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func asInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		parsed, err := v.Int64()
		if err == nil {
			return parsed
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func redactDestinationConf(value string) string {
	redacted := value
	for _, secretKey := range []string{"secret-access-key", "access-key-id"} {
		needle := secretKey + "="
		index := strings.Index(redacted, needle)
		if index == -1 {
			continue
		}
		start := index + len(needle)
		end := strings.Index(redacted[start:], "&")
		if end == -1 {
			redacted = redacted[:start] + "REDACTED"
			continue
		}
		redacted = redacted[:start] + "REDACTED" + redacted[start+end:]
	}
	return redacted
}
