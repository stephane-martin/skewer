package utils

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAtoi32(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    int32
		wantErr bool
	}{
		{"zero", "0", 0, false},
		{"one", "1", 1, false},
		{"minus one", "-1", -1, false},
		{"max", fmt.Sprintf("%d", math.MaxInt32), math.MaxInt32, false},
		{"min", fmt.Sprintf("%d", math.MinInt32), math.MinInt32, false},
		{"too large", fmt.Sprintf("%d", math.MaxInt64), 0, true},
		{"too large negative", fmt.Sprintf("%d", math.MinInt64), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Atoi32(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Atoi32() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Atoi32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTime2Bytes(t *testing.T) {
	a := assert.New(t)
	var zero time.Time
	now := time.Now()
	a.NotEmpty(Time2Bytes(zero, nil))
	a.NotEmpty(Time2Bytes(now, nil))
	a.NotEmpty(Time2Bytes(now, Time2Bytes(zero, nil)))
}

func tostr(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

func TestBytes2Time(t *testing.T) {
	a := assert.New(t)
	_, n := Bytes2Time(nil)
	a.True(n <= 0)
	now := time.Now()
	now2, n := Bytes2Time(Time2Bytes(now, nil))
	a.True(n > 0)
	a.True(now.Equal(now2), "%s != %s", tostr(now), tostr(now2))
}
