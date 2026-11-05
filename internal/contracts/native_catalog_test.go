package contracts

import "testing"

func TestNativeCatalogStrictInput(t *testing.T) {
	for _, raw := range []string{`{"kind":"environments"}`, `{"kind":"agents","adapter":"orca-terminal","target":{"kind":"local","environment":""}}`, `{"kind":"models","adapter":"orca-terminal","target":{"kind":"orca_environment","environment":"one"},"agent":"custom-agent"}`} {
		if _, fault := ReadNativeCatalog([]byte(raw)); fault != nil {
			t.Fatal(raw, fault)
		}
	}
	for _, raw := range []string{`{"kind":"environments","target":null}`, `{"kind":"environments","kind":"environments"}`, `{"kind":"agents","target":{"kind":"local"}}`, `{"kind":"agents","adapter":"orca-terminal","target":{"kind":"local"},"agent":""}`, `{"kind":"models","adapter":"orca-terminal","target":{"kind":"orca_environment","environment":" "},"agent":"codex"}`, `{"kind":"models","adapter":"orca-terminal","target":{"kind":"local"},"agent":"codex","executable":"other"}`, `{"kind":"environments"} {}`} {
		if _, fault := ReadNativeCatalog([]byte(raw)); fault == nil || fault.Exit != 2 {
			t.Fatal("invalid query accepted or misclassified", raw, fault)
		}
	}
}
