package main

import "testing"

func Test_GetInletsClientImage_DefaultOSSNoOverride(t *testing.T) {

	c := InfraConfig{}
	got := c.GetInletsClientImage()
	want := "inlets/inlets:2.6.3"
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
	want := "alexellis2/inlets-pro:0.4.3"
	if got != want {
		t.Errorf("for OSS variant want %s, but got %s", want, got)
		t.Fail()
	}
}

func Test_GetInletsClientImage_DefaultProWithOverride(t *testing.T) {

	c := InfraConfig{
		ProConfig: InletsProConfig{
			License:     "non-empty",
			ClientImage: "alexellis2/inlets-pro:0.4.3-armhf",
		},
	}

	got := c.GetInletsClientImage()
	want := "alexellis2/inlets-pro:0.4.3-armhf"
	if got != want {
		t.Errorf("for OSS variant want %s, but got %s", want, got)
		t.Fail()
	}
}
