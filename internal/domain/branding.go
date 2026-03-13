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
	SocialTwitter      string `json:"social_twitter"`       // X (Twitter) URL
	SocialInstagram    string `json:"social_instagram"`     // Instagram URL
	IconTwitterURL     string `json:"icon_twitter_url"`     // URL to X/Twitter icon image
	IconInstagramURL   string `json:"icon_instagram_url"`   // URL to Instagram icon image
	SocialYouTube      string `json:"social_youtube"`       // YouTube URL
	IconYouTubeURL     string `json:"icon_youtube_url"`     // URL to YouTube icon image
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
		SocialTwitter:    "https://x.com/wasbot",
		SocialInstagram:  "https://instagram.com/wasbot.app",
		IconTwitterURL:   "https://www.wasbot.app/icons/x.png",
		IconInstagramURL: "https://www.wasbot.app/icons/instagram.png",
		SocialYouTube:    "https://www.youtube.com/@wasbot_app",
		IconYouTubeURL:   "https://www.wasbot.app/icons/youtube.png",
	}
}

// MergeWithDefaults fills any empty fields in the config with default values.
func (b *BrandingConfig) MergeWithDefaults() {
	defaults := DefaultBranding()
	if b.PrimaryColor == "" {
		b.PrimaryColor = defaults.PrimaryColor
	}
	if b.SecondaryColor == "" {
		b.SecondaryColor = defaults.SecondaryColor
	}
	if b.AccentColor == "" {
		b.AccentColor = defaults.AccentColor
	}
	if b.DangerColor == "" {
		b.DangerColor = defaults.DangerColor
	}
	if b.CompanyName == "" {
		b.CompanyName = defaults.CompanyName
	}
	if b.LogoURL == "" {
		b.LogoURL = defaults.LogoURL
	}
	if b.DashboardURL == "" {
		b.DashboardURL = defaults.DashboardURL
	}
	if b.SupportEmail == "" {
		b.SupportEmail = defaults.SupportEmail
	}
	if b.WebsiteURL == "" {
		b.WebsiteURL = defaults.WebsiteURL
	}
	if b.SocialTwitter == "" {
		b.SocialTwitter = defaults.SocialTwitter
	}
	if b.SocialInstagram == "" {
		b.SocialInstagram = defaults.SocialInstagram
	}
	if b.IconTwitterURL == "" {
		b.IconTwitterURL = defaults.IconTwitterURL
	}
	if b.IconInstagramURL == "" {
		b.IconInstagramURL = defaults.IconInstagramURL
	}
	if b.SocialYouTube == "" {
		b.SocialYouTube = defaults.SocialYouTube
	}
	if b.IconYouTubeURL == "" {
		b.IconYouTubeURL = defaults.IconYouTubeURL
	}
}
