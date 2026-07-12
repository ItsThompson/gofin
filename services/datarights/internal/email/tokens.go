package email

// BrandTokens holds the design tokens parsed from brand.json.
type BrandTokens struct {
	Colors     BrandColors     `json:"colors"`
	Typography BrandTypography `json:"typography"`
	Spacing    BrandSpacing    `json:"spacing"`
}

// BrandColors contains the brand color palette.
type BrandColors struct {
	Background        string `json:"background"`
	Foreground        string `json:"foreground"`
	Primary           string `json:"primary"`
	PrimaryForeground string `json:"primaryForeground"`
	Muted             string `json:"muted"`
	MutedForeground   string `json:"mutedForeground"`
	Border            string `json:"border"`
}

// BrandTypography contains font family and size tokens.
type BrandTypography struct {
	FontFamily string            `json:"fontFamily"`
	FontSize   map[string]string `json:"fontSize"`
}

// BrandSpacing contains spacing scale tokens.
type BrandSpacing struct {
	Sm string `json:"sm"`
	Md string `json:"md"`
	Lg string `json:"lg"`
	Xl string `json:"xl"`
}
