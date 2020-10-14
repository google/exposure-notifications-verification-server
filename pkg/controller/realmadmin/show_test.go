package realmadmin

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestFormatStats(t *testing.T) {
	now := time.Now().Truncate(24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour).Truncate(24 * time.Hour)
	tests := []struct {
		data    []*database.RealmUserStats
		names   []string
		numDays int
	}{
		{[]*database.RealmUserStats{}, []string{}, 0},
		{
			[]*database.RealmUserStats{
				{UserID: 1, Name: "Rocky", CodesIssued: 10, Date: now},
				{UserID: 1, Name: "Bullwinkle", CodesIssued: 1, Date: now},
			},
			[]string{"Rocky", "Bullwinkle"},
			1,
		},
		{
			[]*database.RealmUserStats{
				{UserID: 1, Name: "Rocky", CodesIssued: 10, Date: yesterday},
				{UserID: 1, Name: "Rocky", CodesIssued: 10, Date: now},
			},
			[]string{"Rocky"},
			2,
		},
	}

	for i, test := range tests {
		names, format := formatData(test.data)
		sort.Strings(test.names)
		sort.Strings(names)
		if !reflect.DeepEqual(test.names, names) {
			t.Errorf("[%d] %v != %v", i, names, test.names)
		}
		if len(format) != test.numDays {
			t.Errorf("[%d] len(format) = %d, expected %d", i, len(format), test.numDays)
		}
		for _, f := range format {
			if len(f) != len(test.names)+1 {
				t.Errorf("[%d] len(codesIssued) = %d, expected %d", i, len(f), len(test.names)+1)
			}
		}
	}
}
