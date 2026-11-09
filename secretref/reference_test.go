package secretref

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestReferenceMetadataRejectsRawPayloadContainers(t *testing.T) {
	var inspect func(reflect.Type)
	inspect = func(value reflect.Type) {
		switch value.Kind() {
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				inspect(value.Field(index).Type)
			}
		case reflect.String, reflect.Bool:
		default:
			t.Errorf("scoped reference metadata acquired a raw or arbitrary payload: %s", value)
		}
	}
	for _, value := range []any{Reference{}, Actor{}, Access{}, View{}} {
		inspect(reflect.TypeOf(value))
	}
}

func TestReferenceAllowsOnlyMaskedOwnerUseAndPresence(t *testing.T) {
	access := Access{Reference: NewReference("telegram", "telegramBotToken"), Actor: Actor{Component: "telegram"}, Operation: UseForOwner, IntentID: "fixture-intent"}
	if err := access.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Access){
		func(a *Access) { a.Actor.Component = "other" },
		func(a *Access) { a.Operation = "REVEAL" },
		func(a *Access) { a.Reference.Projection = "RAW" },
		func(a *Access) { a.Reference.Policy = "ARBITRARY" },
		func(a *Access) { a.IntentID = strings.Repeat("x", 97) },
	} {
		candidate := access
		mutate(&candidate)
		if candidate.Validate() != ErrDenied {
			t.Fatal("unauthorized secret access accepted")
		}
	}
	view := access.View(true, "deterministic fake plaintext")
	encoded, _ := json.Marshal(view)
	if view.RotationRevision != "" || strings.Contains(string(encoded), "plaintext") || view.Correlation.JobID == "" {
		t.Fatal("unsafe reference metadata")
	}
}
