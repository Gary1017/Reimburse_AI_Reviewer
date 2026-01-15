package voucher

import (
	"testing"
)

func TestNumberToChinese(t *testing.T) {
	filler := &ExcelFiller{}

	tests := []struct {
		name   string
		amount float64
		want   string
	}{
		{
			name:   "zero amount",
			amount: 0,
			want:   "零元整",
		},
		{
			name:   "simple amount with no decimal",
			amount: 100,
			want:   "壹佰元整",
		},
		{
			name:   "amount with jiao",
			amount: 123.50,
			want:   "壹佰贰拾叁元伍角",
		},
		{
			name:   "amount with fen",
			amount: 123.56,
			want:   "壹佰贰拾叁元伍角陆分",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filler.numberToChinese(tt.amount)
			if got != tt.want {
				t.Errorf("numberToChinese() = %v, want %v", got, tt.want)
			}
		})
	}
}
