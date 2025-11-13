package models

import "testing"

func TestDetermineGroupTier(t *testing.T) {
	tests := []struct {
		name      string
		settings  GroupSettings
		wantTier  GroupTier
		wantError bool
	}{
		{
			name:     "basic when nothing bound",
			settings: GroupSettings{},
			wantTier: GroupTierBasic,
		},
		{
			name: "merchant when merchant ID > 0",
			settings: GroupSettings{
				MerchantID: 1001,
			},
			wantTier: GroupTierMerchant,
		},
		{
			name: "upstream when interface id present",
			settings: GroupSettings{
				InterfaceIDs: []string{"iface-1"},
			},
			wantTier: GroupTierUpstream,
		},
		{
			name: "error when both present",
			settings: GroupSettings{
				MerchantID:   1002,
				InterfaceIDs: []string{"iface-2"},
			},
			wantTier:  GroupTierBasic,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetermineGroupTier(tt.settings)
			if tt.wantError && err == nil {
				t.Fatalf("expected error, got none")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.wantTier {
				t.Fatalf("expected tier %s, got %s", tt.wantTier, got)
			}
		})
	}
}

func TestNormalizeInterfaceIDs(t *testing.T) {
	input := []string{"  Foo  ", "bar", "foo", "BAR", "", "baz"}
	got := NormalizeInterfaceIDs(input)

	expected := []string{"bar", "baz", "Foo"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d ids, got %d (%v)", len(expected), len(got), got)
	}
	for i, id := range expected {
		if got[i] != id {
			t.Fatalf("expected %s at index %d, got %s", id, i, got[i])
		}
	}
}

func TestTierHelpers(t *testing.T) {
	if NormalizeGroupTier("") != GroupTierBasic {
		t.Fatalf("expected empty tier to normalize to basic")
	}

	if !IsTierAllowed(GroupTierMerchant, []GroupTier{GroupTierMerchant, GroupTierUpstream}) {
		t.Fatalf("expected merchant tier to be allowed")
	}

	if IsTierAllowed(GroupTierBasic, []GroupTier{GroupTierMerchant}) {
		t.Fatalf("expected basic tier to be disallowed")
	}

	list := FormatAllowedTierList([]GroupTier{GroupTierMerchant, GroupTierUpstream})
	expected := "商户群 / 上游群"
	if list != expected {
		t.Fatalf("expected %s, got %s", expected, list)
	}
}
