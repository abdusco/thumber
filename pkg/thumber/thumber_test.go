package thumber

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseColor(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name      string
		hex       string
		assertRes func(t *testing.T, res color.RGBA, err error)
	}{
		{
			name: "rgb",
			hex:  "#F0F0F0",
			assertRes: func(t *testing.T, res color.RGBA, err error) {
				assert.NoError(t, err)
				assert.Equal(t, color.RGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff}, res)
			},
		},
		{
			name: "rgba",
			hex:  "#101112cc",
			assertRes: func(t *testing.T, res color.RGBA, err error) {
				assert.NoError(t, err)
				assert.Equal(t, color.RGBA{R: 0x10, G: 0x11, B: 0x12, A: 0xcc}, res)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseColor(tt.hex)
			tt.assertRes(t, c, err)
		})
	}
}
