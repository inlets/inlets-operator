package main

import (
	"testing"
)

func Test_validateFlags_DO(t *testing.T) {
	c := InfraConfig{
		Provider:  "digitalocean",
		Region:    "lon1",
		AccessKey: "set",
	}
	err := validateFlags(c)

	if err != nil {
		t.Errorf("expected no error for valid DO config")
	}
}

func Test_validateFlags_Packet(t *testing.T) {
	c := InfraConfig{
		Provider:  "packet",
		ProjectID: "",
	}
	err := validateFlags(c)
	want := "project-id required for provider: packet"
	if err.Error() != want {
		t.Errorf("expected error: %s, got: %s", want, err)
	}
}

func Test_validateFlags_GCE_Zone(t *testing.T) {
	c := InfraConfig{
		Provider:  "gce",
		Zone:      "",
		ProjectID: "my-project",
	}
	err := validateFlags(c)
	want := "zone required for provider: gce"
	if err.Error() != want {
		t.Errorf("expected error: %s, got: %s", want, err)
	}
}

func Test_validateFlags_GCE_ProjectID(t *testing.T) {
	c := InfraConfig{
		Provider:  "gce",
		Zone:      "zone",
		ProjectID: "",
	}
	err := validateFlags(c)
	want := "project-id required for provider: gce"
	if err.Error() != want {
		t.Errorf("expected error: %s, got: %s", want, err)
	}
}

func Test_validateFlags_Azure_SubscriptionID_EmptyValue(t *testing.T) {
	c := InfraConfig{
		Provider:       "azure",
		Region:         "eastus",
		SubscriptionID: "",
	}
	err := validateFlags(c)
	want := "subscription-id required for provider: azure"
	if err.Error() != want {
		t.Errorf("expected error: %s, got: %s", want, err)
	}
}

func Test_validateFlags_Azure_SubscriptionID_GoodValue(t *testing.T) {
	c := InfraConfig{
		Provider:       "azure",
		Region:         "eastus",
		SubscriptionID: "7136bb17-a334-41e1-9543-284f4af96420",
	}
	err := validateFlags(c)
	if err != nil {
		t.Errorf("expected: nil, got: %s", err)
	}
}

func Test_validateFlags_BadMemoryValue(t *testing.T) {
	c := InfraConfig{
		Provider:        "digitalocean",
		MaxClientMemory: "hundred",
	}

	err := validateFlags(c)
	want := "invalid memory value: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'"
	if err == nil {
		t.Errorf("expected an error with bad memory format")
		return
	}
	if err.Error() != want {
		t.Errorf("expected error: %q, got: %q", want, err)
	}
}

func Test_validateFlags_GoodMemoryValue(t *testing.T) {
	c := InfraConfig{
		Provider:        "digitalocean",
		MaxClientMemory: "100Mi",
	}

	err := validateFlags(c)
	if err != nil {
		t.Errorf("expected no error, got: %s", err.Error())
	}
}

func Test_validateFlags_EmptyMemoryValue(t *testing.T) {
	c := InfraConfig{
		Provider:        "digitalocean",
		MaxClientMemory: "",
	}

	err := validateFlags(c)
	if err != nil {
		t.Errorf("expected no error, got: %s", err.Error())
	}
}
