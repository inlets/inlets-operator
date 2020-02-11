package main

import "testing"

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
