package main

import "testing"

func Test_GetInletsClientImage_DefaultProNoOverride(t *testing.T) {

	c := InfraConfig{
		ProConfig: InletsProConfig{
			License: "non-empty",
		},
		AccessKey: "key",
	}

	got := c.GetInletsClientImage()
	want := "ghcr.io/inlets/inlets-pro:0.8.1"
	if got != want {
		t.Errorf("want %s, but got %s", want, got)
		t.Fail()
	}
}

func Test_GetInletsClientImage_DefaultProWithOverride(t *testing.T) {

	c := InfraConfig{
		ProConfig: InletsProConfig{
			License:     "non-empty",
			ClientImage: "inlets/inlets-pro:0.6.0-armhf",
		},
		AccessKey: "key",
	}

	got := c.GetInletsClientImage()
	want := "inlets/inlets-pro:0.6.0-armhf"
	if got != want {
		t.Errorf("want %s, but got %s", want, got)
		t.Fail()
	}
}
