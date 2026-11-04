package registry

import (
	"os"
	"strings"
	"sync"
)

// Environment isolation for the shared Consul agent.
//
// jh-test and jh-prod share one Consul. To keep their service discovery apart,
// instances register (and look up) "<base name><suffix>". The suffix comes from:
//  1. env CONSUL_SERVICE_SUFFIX if set (may be set to "" to force no suffix);
//  2. otherwise the pod namespace (service account namespace file): jh-test -> "-jh-test";
//  3. any other namespace (including jh-prod) or no namespace file -> "" (unchanged names).
//
// KV keys are not affected.
const k8sNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

var (
	serviceSuffixOnce  sync.Once
	serviceSuffixValue string
)

// EnvServiceName returns the Consul service name for base in the current environment.
func EnvServiceName(base string) string {
	serviceSuffixOnce.Do(func() {
		serviceSuffixValue = resolveServiceSuffix(os.LookupEnv, os.ReadFile)
	})
	if base == "" || (serviceSuffixValue != "" && strings.HasSuffix(base, serviceSuffixValue)) {
		return base
	}
	return base + serviceSuffixValue
}

func resolveServiceSuffix(lookupEnv func(string) (string, bool), readFile func(string) ([]byte, error)) string {
	if v, ok := lookupEnv("CONSUL_SERVICE_SUFFIX"); ok {
		return strings.TrimSpace(v)
	}
	b, err := readFile(k8sNamespaceFile)
	if err != nil {
		return ""
	}
	return suffixForNamespace(strings.TrimSpace(string(b)))
}

func suffixForNamespace(ns string) string {
	if ns == "jh-test" {
		return "-jh-test"
	}
	return ""
}
