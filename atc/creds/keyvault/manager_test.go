package keyvault

import (
	"fmt"
	"reflect"
	"testing"
)

func TestHealth(t *testing.T) {
	t.Run("a working client should return good health", func(t *testing.T) {
		fake := &fakeReader{
			GetFunc: func(_ string) (string, bool, error) {
				return "", false, nil
			},
		}
		manager := &KeyVaultManager{
			reader: fake,
		}

		resp, err := manager.Health()
		if err != nil {
			t.Fatalf("an error should not have occurred: %s", err)
		}
		if resp.Error != "" {
			t.Errorf("got unexpected error in health response: %s", resp.Error)
		}
		expected := map[string]string{
			"status": "UP",
		}
		if !reflect.DeepEqual(resp.Response, expected) {
			t.Errorf("incorrect response received:\nexpected: %+v\ngot:%+v", expected, resp.Response)
		}
	})
	t.Run("a non-functioning client should return an error", func(t *testing.T) {
		fake := &fakeReader{
			GetFunc: func(_ string) (string, bool, error) {
				return "", false, fmt.Errorf("nope")
			},
		}
		manager := &KeyVaultManager{
			reader: fake,
		}

		resp, err := manager.Health()
		if err != nil {
			t.Fatalf("an error should not have occurred: %s", err)
		}

		if resp.Error != "nope" {
			t.Errorf("expected %q as and error response, got %q", "nope", resp.Error)
		}
	})
}
