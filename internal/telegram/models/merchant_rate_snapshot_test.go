package models

import "testing"

func TestNormalizeMerchantChannelRates(t *testing.T) {
	rates := NormalizeMerchantChannelRates([]MerchantChannelRate{
		{ChannelCode: " B ", ChannelName: " Beta ", Rate: " 8% "},
		{ChannelCode: "a", ChannelName: "Alpha", Rate: "7"},
		{ChannelCode: "A", ChannelName: "Latest", Rate: "7.5"},
		{ChannelCode: " ", Rate: "9"},
	})

	if len(rates) != 2 {
		t.Fatalf("expected two rates, got %#v", rates)
	}
	if rates[0].ChannelCode != "A" || rates[0].ChannelName != "Latest" || rates[0].Rate != "7.5" {
		t.Fatalf("unexpected normalized first rate: %#v", rates[0])
	}
	if rates[1].ChannelCode != "B" || rates[1].ChannelName != "Beta" || rates[1].Rate != "8%" {
		t.Fatalf("unexpected normalized second rate: %#v", rates[1])
	}
}
