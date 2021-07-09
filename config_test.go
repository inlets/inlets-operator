package main

import (
	"fmt"
	"os"
	"testing"
)

func Test_GetLicenseKey_FromLiteral(t *testing.T) {
	want := "static.key.text"

	c := InletsProConfig{
		License: want,
	}

	key, err := c.GetLicenseKey()

	if err != nil {
		t.Fatalf("no error wanted for a valid key")
	}

	if want != key {
		t.Fatalf("want %s but got %s", want, key)
	}
}

func Test_GetLicenseKey_FromFile(t *testing.T) {
	want := "static.key.text"

	tmp := os.TempDir()
	f, err := os.CreateTemp(tmp, "test-license.txt")
	if err != nil {
		t.Fatal(err)
	}

	name := f.Name()
	f.Write([]byte(want))
	f.Close()
	defer os.Remove(name)

	c := InletsProConfig{
		LicenseFile: name,
	}

	key, err := c.GetLicenseKey()

	if err != nil {
		t.Fatalf("no error wanted for a valid key")
	}

	if want != key {
		t.Fatalf("want %s but got %s", want, key)
	}
}

func Test_GetLicenseKey_FromFileTrimsWhitespace(t *testing.T) {
	want := `static.key.text`

	tmp := os.TempDir()
	f, err := os.CreateTemp(tmp, "test-license-with-newline.txt")
	if err != nil {
		t.Fatal(err)
	}

	name := f.Name()

	// Add a new line at the beginning of the text, and one
	// at the end
	f.Write([]byte(fmt.Sprintf("\n%s\n", want)))
	f.Close()
	defer os.Remove(name)

	c := InletsProConfig{
		LicenseFile: name,
	}

	key, err := c.GetLicenseKey()

	if err != nil {
		t.Fatalf("no error wanted for a valid key")
	}

	if want != key {
		t.Fatalf("want %q but got %q", want, key)
	}
}
