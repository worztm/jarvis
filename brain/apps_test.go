package brain

import "testing"

// Regression: open-commands must bypass intent prototypes entirely.
func TestOpenBypassesPrototypes(t *testing.T) {
	text := sanitize("open notepad")
	if !hasOpenTriggerFuzzy(text) {
		t.Fatal("open trigger missed")
	}
	m := FindTarget(text)
	if m.Canonical != "notepad" || !m.IsApp {
		t.Errorf("notepad not matched as app: %+v", m)
	}
}

func TestNativeAppTargets(t *testing.T) {
	for _, name := range []string{"whatsapp", "telegram", "brave", "brave browser", "telegram desktop"} {
		m := FindTarget(sanitize("open " + name))
		if m.Canonical == "" {
			t.Errorf("%s not found in index", name)
			continue
		}
		if _, ok := Apps[m.Canonical]; !ok {
			t.Errorf("%s resolved to %q which is not an app target", name, m.Canonical)
		}
	}
	if u, ok := Sites["whatsapp web"]; !ok || u != "https://web.whatsapp.com" {
		t.Errorf("whatsapp web missing: %v %v", u, ok)
	}
}
