package mpris

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestPropertyChangedIncludesInvalidatedProperties(t *testing.T) {
	changed := map[string]dbus.Variant{
		"PlaybackStatus": dbus.MakeVariant("Playing"),
	}
	invalidated := []string{"Metadata", "Rate"}

	for _, property := range []string{"PlaybackStatus", "Metadata", "Rate"} {
		if !propertyChanged(changed, invalidated, property) {
			t.Errorf("propertyChanged(%q) = false, want true", property)
		}
	}
	if propertyChanged(changed, invalidated, "Position") {
		t.Error("propertyChanged(Position) = true, want false")
	}
}
