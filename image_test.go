package main

import "testing"

func Test_GetInletsClientImage_DefaultOSSNoOverride(t *testing.T) {

	c := InfraConfig{}
	got := c.GetInletsClientImage()
	want := "inlets/inlets:2.6.4"
	if got != want {
		t.Errorf("for OSS variant want %s, but got %s", want, got)
		t.Fail()
	}
}

func Test_GetInletsClientImage_DefaultProNoOverride(t *testing.T) {

	c := InfraConfig{
		ProConfig: InletsProConfig{
			License: "non-empty",
		},
	}

	got := c.GetInletsClientImage()
	want := "inlets/inlets-pro:0.5.6"
	if got != want {
		t.Errorf("for OSS variant want %s, but got %s", want, got)
		t.Fail()
	}
}

func Test_GetInletsClientImage_DefaultProWithOverride(t *testing.T) {

	c := InfraConfig{
		ProConfig: InletsProConfig{
			License:     "non-empty",
			ClientImage: "inlets/inlets-pro:0.5.6-armhf",
		},
	}

	got := c.GetInletsClientImage()
	want := "inlets/inlets-pro:0.5.6-armhf"
	if got != want {
		t.Errorf("for OSS variant want %s, but got %s", want, got)
		t.Fail()
	}
}
