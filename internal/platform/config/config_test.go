package config

import "testing"

func TestValidateAllowsMockWithoutClientID(t *testing.T) {
	cfg := Config{SwiggyProvider: "mock"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected mock config to validate, got %v", err)
	}
}

func TestValidateRequiresClientIDForNonMockProvider(t *testing.T) {
	cfg := Config{
		SwiggyProvider:         "swiggy",
		SwiggyAuthAuthorizeURL: defaultSwiggyAuthAuthorizeURL,
		SwiggyAuthTokenURL:     defaultSwiggyAuthTokenURL,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing client ID to fail validation")
	}
}

func TestValidateRequiresAuthURLsForNonMockProvider(t *testing.T) {
	cfg := Config{
		SwiggyProvider:         "swiggy",
		SwiggyClientID:         "client-1",
		SwiggyAuthAuthorizeURL: "not a url",
		SwiggyAuthTokenURL:     defaultSwiggyAuthTokenURL,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid authorize URL to fail validation")
	}

	cfg.SwiggyAuthAuthorizeURL = defaultSwiggyAuthAuthorizeURL
	cfg.SwiggyAuthTokenURL = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty token URL to fail validation")
	}
}

func TestValidateAcceptsConfiguredSwiggyProvider(t *testing.T) {
	cfg := Config{
		SwiggyProvider:         "swiggy",
		SwiggyClientID:         "client-1",
		SwiggyAuthAuthorizeURL: defaultSwiggyAuthAuthorizeURL,
		SwiggyAuthTokenURL:     defaultSwiggyAuthTokenURL,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid swiggy config, got %v", err)
	}
}
