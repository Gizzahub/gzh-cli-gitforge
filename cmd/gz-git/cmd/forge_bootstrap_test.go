package cmd

import (
	"slices"
	"testing"
)

func TestForgeBootstrapFlagsAndEffects(t *testing.T) {
	for _, item := range []struct {
		path  []string
		flags []string
	}{{[]string{"forge", "bootstrap", "plan"}, []string{"input", "output", "seed-dir"}}, {[]string{"forge", "bootstrap", "apply"}, []string{"plan", "confirm", "commit-model"}}} {
		command, _, err := rootCmd.Find(item.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range item.flags {
			if command.Flags().Lookup(flag) == nil {
				t.Fatalf("missing flag %s", flag)
			}
		}
	}
	if len(commandEffects["forge bootstrap plan"]) != 0 {
		t.Fatal("plan changes retained state")
	}
	for _, effect := range []string{mutatesFilesystem, mutatesRefs, mutatesRemote, mutatesForge} {
		if !slices.Contains(commandEffects["forge bootstrap apply"], effect) {
			t.Fatalf("missing apply mutation %s", effect)
		}
	}
}
