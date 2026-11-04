package registry

import (
	"errors"
	"testing"
)

func TestResolveServiceSuffix(t *testing.T) {
	noEnv := func(string) (string, bool) { return "", false }
	ns := func(v string) func(string) ([]byte, error) {
		return func(string) ([]byte, error) { return []byte(v), nil }
	}
	noFile := func(string) ([]byte, error) { return nil, errors.New("not found") }
	cases := []struct {
		name string
		env  func(string) (string, bool)
		file func(string) ([]byte, error)
		want string
	}{
		{"test namespace", noEnv, ns("jh-test\n"), "-jh-test"},
		{"prod namespace keeps old names", noEnv, ns("jh-prod"), ""},
		{"other namespace", noEnv, ns("default"), ""},
		{"no k8s", noEnv, noFile, ""},
		{"env override", func(string) (string, bool) { return "-x", true }, ns("jh-test"), "-x"},
		{"env empty forces none", func(string) (string, bool) { return "", true }, ns("jh-test"), ""},
	}
	for _, c := range cases {
		if got := resolveServiceSuffix(c.env, c.file); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestEnvServiceNameNoDoubleSuffix(t *testing.T) {
	serviceSuffixOnce.Do(func() {})
	old := serviceSuffixValue
	defer func() { serviceSuffixValue = old }()
	serviceSuffixValue = "-jh-test"
	if got := EnvServiceName("jh_app_service"); got != "jh_app_service-jh-test" {
		t.Fatal(got)
	}
	if got := EnvServiceName("jh_app_service-jh-test"); got != "jh_app_service-jh-test" {
		t.Fatal(got)
	}
	serviceSuffixValue = ""
	if got := EnvServiceName("jh_app_service"); got != "jh_app_service" {
		t.Fatal(got)
	}
}
