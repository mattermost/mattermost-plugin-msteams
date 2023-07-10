package assets

import (
	_ "embed"
)

//go:embed mm-logo-color.png
var LogoColorData []byte

//go:embed mm-logo-outline.png
var LogoOutlineData []byte
