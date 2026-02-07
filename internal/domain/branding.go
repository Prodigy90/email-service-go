package domain

// BrandingConfig holds product-specific branding for email templates
type BrandingConfig struct {
	PrimaryColor    string `json:"primary_color"`    // e.g., "#10b981" (green for wasbot)
	SecondaryColor  string `json:"secondary_color"`  // e.g., "#059669"
	AccentColor     string `json:"accent_color"`     // e.g., "#047857"
	DangerColor     string `json:"danger_color"`     // e.g., "#ef4444"
	CompanyName     string `json:"company_name"`     // e.g., "WASBOT"
	LogoURL         string `json:"logo_url"`         // URL to company logo
	DashboardURL    string `json:"dashboard_url"`    // e.g., "https://www.wasbot.app/dashboard"
	SupportEmail    string `json:"support_email"`    // e.g., "support@wasbot.app"
	WebsiteURL      string `json:"website_url"`      // e.g., "https://www.wasbot.app"
	SocialTwitter   string `json:"social_twitter"`   // X (Twitter) URL
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
		LogoURL:         "https://www.wasbot.app/wasbot-icon.png",
		DashboardURL:    "https://www.wasbot.app/dashboard",
		SupportEmail:    "support@wasbot.app",
		WebsiteURL:      "https://www.wasbot.app",
		SocialTwitter:   "https://x.com/wasbot",
		SocialInstagram: "https://instagram.com/wasbot.app",
	}
}
