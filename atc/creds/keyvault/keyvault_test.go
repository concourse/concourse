package keyvault

import (
	"fmt"
	"testing"

	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"
)

func TestGet(t *testing.T) {
	t.Run("valid secret name should return the secret value", func(t *testing.T) {
		expected := "section31"
		var calledName string
		fake := &fakeReader{}
		kv := &keyVault{
			PathPrefix:   "ufp",
			TeamName:     "ussenterprise",
			PipelineName: "farpoint",
			reader:       fake,
		}
		t.Run("team name variable should return value", func(t *testing.T) {
			fake.GetFunc = func(name string) (string, bool, error) {
				calledName = name
				return expected, true, nil
			}
			val, found, err := kv.Get(varTemplate.VariableDefinition{Name: "lcars"})
			if err != nil {
				t.Fatalf("an error should not have occurred: %s", err)
			}
			if !found {
				t.Fatal("expected to find the secret")
			}
			if val.(string) != expected {
				t.Errorf("unexpected secret value returned. Expected %s, got %v", expected, val)
			}
			expectedName := fmt.Sprintf("%s-%s-%s-%s", kv.PathPrefix, kv.TeamName, kv.PipelineName, "lcars")
			if calledName != expectedName {
				t.Errorf("unexpected secret name generated. Expected %s, got %s", expectedName, calledName)
			}
		})
		t.Run("pipeline variable should return value", func(t *testing.T) {
			var calledOnce bool
			fake.GetFunc = func(name string) (string, bool, error) {
				// This allows us to emulate not finding it the first time
				if !calledOnce {
					calledOnce = true
					return "", false, nil
				}
				calledName = name
				return expected, true, nil
			}
			val, found, err := kv.Get(varTemplate.VariableDefinition{Name: "lcars"})
			if err != nil {
				t.Fatalf("an error should not have occurred: %s", err)
			}
			if !found {
				t.Fatal("expected to find the secret")
			}
			if val.(string) != expected {
				t.Errorf("unexpected secret value returned. Expected %s, got %v", expected, val)
			}
			expectedName := fmt.Sprintf("%s-%s-%s", kv.PathPrefix, kv.TeamName, "lcars")
			if calledName != expectedName {
				t.Errorf("unexpected secret name generated. Expected %s, got %s", expectedName, calledName)
			}
		})

	})
	t.Run("not found secret should return a false found value", func(t *testing.T) {
		var callCount int
		fake := &fakeReader{
			GetFunc: func(name string) (string, bool, error) {
				callCount++
				return "", false, nil
			},
		}
		kv := &keyVault{
			PathPrefix:   "ufp",
			TeamName:     "ussvoyager",
			PipelineName: "borg",
			reader:       fake,
		}

		val, found, err := kv.Get(varTemplate.VariableDefinition{Name: "doctor"})
		if err != nil {
			t.Fatalf("unexpected error occurred: %s", err)
		}
		if found {
			t.Error("found bool should not be true")
		}
		if val != nil {
			t.Error("secret value should be nil")
		}

		// Make sure that it tried twice to find the secret. Once for pipeline and once for the team
		if callCount != 2 {
			t.Errorf("expected Get to try and get the secret 2 times, got %d times", callCount)
		}
	})
	t.Run("an error while fetching the secret should return an error", func(t *testing.T) {
		fake := &fakeReader{
			GetFunc: func(name string) (string, bool, error) {
				return "", false, fmt.Errorf("fake error")
			},
		}
		kv := &keyVault{
			PathPrefix:   "ufp",
			TeamName:     "ds9",
			PipelineName: "bajor",
			reader:       fake,
		}

		val, found, err := kv.Get(varTemplate.VariableDefinition{Name: "defiant"})
		if err == nil {
			t.Fatal("an error should have occurred")
		}
		if found {
			t.Error("found bool should not be true")
		}
		if val != nil {
			t.Error("secret value should be nil")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("a valid list of secret names should be returned", func(t *testing.T) {
		var calledPrefix string
		fake := &fakeReader{
			ListFunc: func(prefix string) ([]string, error) {
				calledPrefix = prefix
				return []string{"bird", "of", "prey"}, nil
			},
		}
		kv := &keyVault{
			PathPrefix: "klingonempire",
			reader:     fake,
		}

		vars, err := kv.List()
		if err != nil {
			t.Fatalf("an error should not have occurred: %s", err)
		}
		if calledPrefix != kv.PathPrefix {
			t.Errorf("expected list prefix to be %s, got %s", kv.PathPrefix, calledPrefix)
		}

		for _, item := range vars {
			switch item.Name {
			case "bird", "of", "prey":
				continue
			default:
				t.Errorf("got unexpected variable name of %s", item.Name)
			}
		}
	})
	t.Run("a error should be returned if unable to list secrets", func(t *testing.T) {
		fake := &fakeReader{
			ListFunc: func(prefix string) ([]string, error) {
				return nil, fmt.Errorf("fake error")
			},
		}
		kv := &keyVault{
			PathPrefix: "klingonempire",
			reader:     fake,
		}

		vars, err := kv.List()
		if err == nil {
			t.Fatal("an error should have occurred:")
		}
		if vars != nil {
			t.Errorf("returned variable list should be nil, got %+v", vars)
		}
	})
}
