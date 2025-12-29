package domain

// BrandingConfig holds product-specific branding for email templates
type BrandingConfig struct {
	PrimaryColor    string `json:"primary_color"`    // e.g., "#10b981" (green for wasbot)
	SecondaryColor  string `json:"secondary_color"`  // e.g., "#059669"
	AccentColor     string `json:"accent_color"`     // e.g., "#047857"
	DangerColor     string `json:"danger_color"`     // e.g., "#ef4444"
	CompanyName     string `json:"company_name"`     // e.g., "WASBOT"
	LogoURL         string `json:"logo_url"`         // URL to company logo
	DashboardURL    string `json:"dashboard_url"`    // e.g., "https://wasbot.ng/dashboard"
	SupportEmail    string `json:"support_email"`    // e.g., "support@wasbot.ng"
	WebsiteURL      string `json:"website_url"`      // e.g., "https://wasbot.ng"
	SocialTwitter   string `json:"social_twitter"`   // Twitter URL
	SocialInstagram string `json:"social_instagram"` // Instagram URL
}

// DefaultBranding returns the default WASBOT branding (for backward compatibility)
func DefaultBranding() *BrandingConfig {
	return &BrandingConfig{
		PrimaryColor:    "#10b981",
		SecondaryColor:  "#059669",
		AccentColor:     "#047857",
		DangerColor:     "#ef4444",
		CompanyName:     "WASBOT",
		LogoURL:         "",
		DashboardURL:    "https://wasbot.ng/dashboard",
		SupportEmail:    "support@wasbot.ng",
		WebsiteURL:      "https://wasbot.ng",
		SocialTwitter:   "https://twitter.com/AskWasBot",
		SocialInstagram: "https://instagram.com/AskWasBot",
	}
}
