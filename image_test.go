package main

import "testing"

func Test_GetInletsReleaseDefault(t *testing.T) {

	c := InfraConfig{
		TunnelConfig: TunnelConfig{
			License: "non-empty",
		},
		AccessKey: "key",
	}

	got := c.GetInletsRelease()

	want := "0.9.40"
	if got != want {
		t.Fatalf("want %s, but got %s", want, got)
	}
}

func Test_GetInletsReleaseOverride(t *testing.T) {

	c := InfraConfig{
		TunnelConfig: TunnelConfig{
			License:       "non-empty",
			InletsRelease: "0.9.40",
		},
		AccessKey: "key",
	}

	got := c.GetInletsRelease()

	want := "0.9.40"

	if got != want {
		t.Fatalf("want %s, but got %s", want, got)
	}
}

func Test_InletsClientImageDefault(t *testing.T) {

	c := InfraConfig{
		TunnelConfig: TunnelConfig{
			License: "non-empty",
		},
		AccessKey: "key",
	}

	got := c.GetInletsClientImage()
	want := "ghcr.io/inlets/inlets-pro:0.9.40"
	if got != want {
		t.Fatalf("want %s, but got %s", want, got)
	}
}

func Test_InletsClientImageOverride(t *testing.T) {

	c := InfraConfig{
		TunnelConfig: TunnelConfig{
			License:     "non-empty",
			ClientImage: "alexellis2/inlets-pro:0.9.40",
		},
		AccessKey: "key",
	}

	got := c.GetInletsClientImage()
	want := "alexellis2/inlets-pro:0.9.40"
	if got != want {

		t.Fatalf("want %s, but got %s", want, got)
	}
}
