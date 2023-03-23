package fonts

import (
	_ "embed"
	"fmt"

	"github.com/BurntSushi/freetype-go/freetype"
	"github.com/BurntSushi/freetype-go/freetype/truetype"
)

//go:embed RobotoMono-Medium.ttf
var robotoBytes []byte

var RobotoMonoMedium *truetype.Font = func() *truetype.Font {
	f, err := freetype.ParseFont(robotoBytes)
	if err != nil {
		panic(fmt.Errorf("failed to parse font: %w", err))
	}
	return f
}()
