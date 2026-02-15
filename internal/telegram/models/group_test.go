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
				InterfaceBindings: []InterfaceBinding{
					{Name: "test", ID: "iface-1"},
				},
			},
			wantTier: GroupTierUpstream,
		},
		{
			name: "error when both present",
			settings: GroupSettings{
				MerchantID: 1002,
				InterfaceBindings: []InterfaceBinding{
					{Name: "test", ID: "iface-2"},
				},
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

func TestNormalizeInterfaceBindings(t *testing.T) {
	input := []InterfaceBinding{
		{Name: " Foo ", ID: "  Foo  ", Rate: " 5% "},
		{Name: "bar", ID: "bar"},
		{Name: "duplicate", ID: "foo"},
		{Name: "dup-2", ID: "BAR"},
		{Name: "empty", ID: " "},
	}
	got := NormalizeInterfaceBindings(input)

	expected := []InterfaceBinding{
		{Name: "bar", ID: "bar", Rate: ""},
		{Name: "Foo", ID: "Foo", Rate: "5%"},
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d bindings, got %d (%v)", len(expected), len(got), got)
	}
	for i, binding := range expected {
		if got[i].ID != binding.ID || got[i].Name != binding.Name || got[i].Rate != binding.Rate {
			t.Fatalf("expected binding %+v at index %d, got %+v", binding, i, got[i])
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

	if !IsCascadeReplyEnabled(GroupSettings{}) {
		t.Fatalf("expected cascade reply to be enabled by default")
	}

	if IsCascadeReplyEnabled(GroupSettings{
		CascadeReplyEnabled:    false,
		CascadeReplyConfigured: true,
	}) {
		t.Fatalf("expected configured cascade reply switch to be honored")
	}
}
