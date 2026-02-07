package service

import (
	"fmt"
	"testing"

	"go_bot/internal/payment/sifang"
)

func TestIsOrderNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "api code 404",
			err:  fmt.Errorf("wrapped: %w", &sifang.APIError{Code: 404, Message: "server says missing"}),
			want: true,
		},
		{
			name: "api chinese order not found",
			err:  fmt.Errorf("wrapped: %w", &sifang.APIError{Code: 1, Message: "订单不存在。"}),
			want: true,
		},
		{
			name: "api english not found",
			err:  fmt.Errorf("wrapped: %w", &sifang.APIError{Code: 1, Message: "not found"}),
			want: true,
		},
		{
			name: "api other error",
			err:  fmt.Errorf("wrapped: %w", &sifang.APIError{Code: 500, Message: "server error"}),
			want: false,
		},
		{
			name: "non api error",
			err:  fmt.Errorf("some io error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOrderNotFoundError(tt.err); got != tt.want {
				t.Fatalf("IsOrderNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}
