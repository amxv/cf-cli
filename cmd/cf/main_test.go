package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFindDNSRecordForUpdateUsesFirstRecordForNonMX(t *testing.T) {
	records := []dnsRecord{
		{ID: "first", Content: "192.0.2.1"},
		{ID: "second", Content: "192.0.2.2"},
	}

	record, found := findDNSRecordForUpdate(records, dnsRecord{Type: "A", Content: "192.0.2.3"})
	if !found {
		t.Fatal("expected an existing A record to be selected")
	}
	if record.ID != "first" {
		t.Fatalf("expected the first A record, got %q", record.ID)
	}
}

func TestFindDNSRecordForUpdateMatchesMXByPriorityAndValue(t *testing.T) {
	records := []dnsRecord{
		{ID: "mx1", Content: "mx1.spacemail.com", Priority: 10},
		{ID: "mx2", Content: "mx2.spacemail.com", Priority: 20},
	}

	record, found := findDNSRecordForUpdate(records, dnsRecord{Type: "mx", Content: "MX2.SPACEMAIL.COM.", Priority: 20})
	if !found {
		t.Fatal("expected the matching MX record to be selected")
	}
	if record.ID != "mx2" {
		t.Fatalf("expected mx2, got %q", record.ID)
	}
}

func TestFindDNSRecordForUpdateDoesNotOverwriteDifferentMX(t *testing.T) {
	records := []dnsRecord{
		{ID: "mx1", Content: "mx1.spacemail.com", Priority: 10},
	}

	if record, found := findDNSRecordForUpdate(records, dnsRecord{Type: "MX", Content: "mx2.spacemail.com", Priority: 20}); found {
		t.Fatalf("expected no match, got %q", record.ID)
	}
}

func TestFindDNSRecordForUpdateRequiresMXPriorityMatch(t *testing.T) {
	records := []dnsRecord{
		{ID: "mx1", Content: "mx1.spacemail.com", Priority: 10},
	}

	if record, found := findDNSRecordForUpdate(records, dnsRecord{Type: "MX", Content: "mx1.spacemail.com", Priority: 20}); found {
		t.Fatalf("expected no match, got %q", record.ID)
	}
}

func TestFindDNSRecordForUpdateMatchesTXTByExactContent(t *testing.T) {
	records := []dnsRecord{
		{ID: "first", Content: "verification=one"},
		{ID: "second", Content: "verification=two"},
	}

	record, found := findDNSRecordForUpdate(records, dnsRecord{Type: "TXT", Content: "verification=two"})
	if !found || record.ID != "second" {
		t.Fatalf("expected the exact TXT match, got %q (found=%t)", record.ID, found)
	}
	if _, found := findDNSRecordForUpdate(records, dnsRecord{Type: "TXT", Content: "VERIFICATION=TWO"}); found {
		t.Fatal("expected TXT matching to remain case-sensitive")
	}
}

func TestFindDNSRecordForUpdateMatchesStructuredSRVData(t *testing.T) {
	records := []dnsRecord{
		{ID: "primary", Data: &dnsRecordData{Priority: 10, Weight: 5, Port: 5060, Target: "sip.example.com"}},
		{ID: "backup", Data: &dnsRecordData{Priority: 20, Weight: 5, Port: 5060, Target: "sip-backup.example.com"}},
	}
	desired := dnsRecord{
		Type: "SRV",
		Data: &dnsRecordData{Priority: 20, Weight: 5, Port: 5060, Target: "SIP-BACKUP.EXAMPLE.COM."},
	}

	record, found := findDNSRecordForUpdate(records, desired)
	if !found || record.ID != "backup" {
		t.Fatalf("expected the structured SRV match, got %q (found=%t)", record.ID, found)
	}
	desired.Data.Port = 5061
	if record, found := findDNSRecordForUpdate(records, desired); found {
		t.Fatalf("expected no SRV match with a different port, got %q", record.ID)
	}
}

func TestMakeRecordPayloadUsesStructuredSRVDataAndDisablesProxy(t *testing.T) {
	originalPriority, originalWeight, originalPort := priority, srvWeight, srvPort
	originalPrioritySet, originalWeightSet, originalPortSet := prioritySet, srvWeightSet, srvPortSet
	originalProxied := proxied
	t.Cleanup(func() {
		priority, srvWeight, srvPort = originalPriority, originalWeight, originalPort
		prioritySet, srvWeightSet, srvPortSet = originalPrioritySet, originalWeightSet, originalPortSet
		proxied = originalProxied
	})

	priority, srvWeight, srvPort = 0, 5, 5060
	prioritySet, srvWeightSet, srvPortSet = true, true, true
	proxied = true
	payload, err := makeRecordPayload("SRV", "_sip._tcp.example.com", "sip.example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if payload["proxied"] != false {
		t.Fatalf("expected SRV to be DNS-only, got proxied=%v", payload["proxied"])
	}
	if _, exists := payload["content"]; exists {
		t.Fatal("expected structured SRV payload not to include content")
	}
	data, ok := payload["data"].(dnsRecordData)
	if !ok {
		t.Fatalf("expected dnsRecordData, got %T", payload["data"])
	}
	if data.Priority != 0 || data.Weight != 5 || data.Port != 5060 || data.Target != "sip.example.com" {
		t.Fatalf("unexpected SRV data: %+v", data)
	}
}

func TestMakeRecordPayloadAllowsZeroMXPriorityAndDisablesProxy(t *testing.T) {
	originalPriority, originalPrioritySet, originalProxied := priority, prioritySet, proxied
	t.Cleanup(func() {
		priority, prioritySet, proxied = originalPriority, originalPrioritySet, originalProxied
	})

	priority, prioritySet, proxied = 0, true, true
	payload, err := makeRecordPayload("MX", "example.com", "mx.example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if payload["priority"] != uint16(0) {
		t.Fatalf("expected priority zero, got %v", payload["priority"])
	}
	if payload["proxied"] != false {
		t.Fatalf("expected MX to be DNS-only, got proxied=%v", payload["proxied"])
	}
}

func TestMakeRecordPayloadForcesTXTToDNSOnly(t *testing.T) {
	originalProxied := proxied
	t.Cleanup(func() { proxied = originalProxied })
	proxied = true

	payload, err := makeRecordPayload("TXT", "example.com", "verification=value", "")
	if err != nil {
		t.Fatal(err)
	}
	if payload["proxied"] != false {
		t.Fatalf("expected TXT to be DNS-only, got proxied=%v", payload["proxied"])
	}
}

func TestMakeRecordPayloadRejectsIncompleteSRVData(t *testing.T) {
	originalPrioritySet, originalWeightSet, originalPortSet := prioritySet, srvWeightSet, srvPortSet
	t.Cleanup(func() {
		prioritySet, srvWeightSet, srvPortSet = originalPrioritySet, originalWeightSet, originalPortSet
	})
	prioritySet, srvWeightSet, srvPortSet = true, true, false

	_, err := makeRecordPayload("SRV", "_sip._tcp.example.com", "sip.example.com", "")
	if err == nil || !strings.Contains(err.Error(), "cf dns srv") {
		t.Fatalf("expected actionable SRV usage error, got %v", err)
	}
}

func TestDNSRecordJSONPreservesStructuredSRVData(t *testing.T) {
	record := dnsRecord{
		ID:   "record-id",
		Type: "SRV",
		Name: "_sip._tcp.example.com",
		Data: &dnsRecordData{Priority: 10, Weight: 5, Port: 5060, Target: "sip.example.com"},
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"data"`, `"priority":10`, `"weight":5`, `"port":5060`, `"target":"sip.example.com"`} {
		if !strings.Contains(string(encoded), expected) {
			t.Fatalf("expected JSON to contain %s, got %s", expected, encoded)
		}
	}
}

func TestParseDNSUint16RejectsTrailingText(t *testing.T) {
	if _, err := parseDNSUint16("SRV port", "5060oops"); err == nil {
		t.Fatal("expected trailing text to be rejected")
	}
}
