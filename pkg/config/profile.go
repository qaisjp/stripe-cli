package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/stripe/stripe-cli/pkg/validators"
)

// Profile handles all things related to managing the project specific configurations
type Profile struct {
	DeviceName             string
	ProfileName            string
	APIKey                 string
	LiveModeAPIKey         string
	LiveModePublishableKey string
	TestModeAPIKey         string
	TestModePublishableKey string
	TerminalPOSDeviceID    string
	DisplayName            string
	AccountID              string
}

// config key names
const (
	AccountIDName              = "account_id"
	DeviceNameName             = "device_name"
	DisplayNameName            = "display_name"
	IsTermsAcceptanceValidName = "is_terms_acceptance_valid"
	TestModeAPIKeyName         = "test_mode_api_key"
	TestModePublishableKeyName = "test_mode_publishable_key"
	TestModeExpiresAtName      = "test_mode_key_expires_at"
	LiveModeAPIKeyName         = "live_mode_api_key"
	LiveModePublishableKeyName = "live_mode_publishable_key"
	LiveModeExpiresAtName      = "live_mode_key_expires_at"
)

// CreateProfile creates a profile when logging in
func (p *Profile) CreateProfile() error {
	writeErr := p.writeProfile(viper.GetViper())
	if writeErr != nil {
		return writeErr
	}

	return nil
}

// GetColor gets the color setting for the user based on the flag or the
// persisted color stored in the config file
func (p *Profile) GetColor() (string, error) {
	color := viper.GetString("color")
	if color != "" {
		return color, nil
	}

	color = viper.GetString(p.GetConfigField("color"))
	switch color {
	case "", ColorAuto:
		return ColorAuto, nil
	case ColorOn:
		return ColorOn, nil
	case ColorOff:
		return ColorOff, nil
	default:
		return "", fmt.Errorf("color value not supported: %s", color)
	}
}

// GetDeviceName returns the configured device name
func (p *Profile) GetDeviceName() (string, error) {
	if os.Getenv("STRIPE_DEVICE_NAME") != "" {
		return os.Getenv("STRIPE_DEVICE_NAME"), nil
	}

	if p.DeviceName != "" {
		return p.DeviceName, nil
	}

	if err := viper.ReadInConfig(); err == nil {
		return viper.GetString(p.GetConfigField(DeviceNameName)), nil
	}

	return "", validators.ErrDeviceNameNotConfigured
}

// GetAccountID returns the accountId for the given profile.
func (p *Profile) GetAccountID() (string, error) {
	if p.AccountID != "" {
		return p.AccountID, nil
	}

	if err := viper.ReadInConfig(); err == nil {
		return viper.GetString(p.GetConfigField(AccountIDName)), nil
	}

	return "", validators.ErrAccountIDNotConfigured
}

// GetAPIKey will return the existing key for the given profile
func (p *Profile) GetAPIKey(livemode bool) (string, error) {
	envKey := os.Getenv("STRIPE_API_KEY")
	if envKey != "" {
		err := validators.APIKey(envKey)
		if err != nil {
			return "", err
		}

		return envKey, nil
	}

	if p.APIKey != "" {
		err := validators.APIKey(p.APIKey)
		if err != nil {
			return "", err
		}

		return p.APIKey, nil
	}

	// If the user doesn't have an api_key field set, they might be using an
	// old configuration so try to read from secret_key
	if !livemode {
		if !viper.IsSet(p.GetConfigField("api_key")) {
			p.RegisterAlias("api_key", "secret_key")
		} else {
			p.RegisterAlias(TestModeAPIKeyName, "api_key")
		}
	}

	// Try to fetch the API key from the configuration file
	if err := viper.ReadInConfig(); err == nil {
		var key string
		fieldID := livemodeKeyField(livemode)

		if !livemode {
			key = viper.GetString(p.GetConfigField(fieldID))
		} else {
			key = p.RetrieveLivemodeValue(fieldID)
		}

		err := validators.APIKey(key)
		if err != nil {
			return "", err
		}

		return key, nil
	}

	return "", validators.ErrAPIKeyNotConfigured
}

// GetPublishableKey returns the publishable key for the user
func (p *Profile) GetPublishableKey() string {
	if err := viper.ReadInConfig(); err == nil {
		if viper.IsSet(p.GetConfigField("publishable_key")) {
			p.RegisterAlias(TestModePublishableKeyName, "publishable_key")
		}

		return viper.GetString(p.GetConfigField(TestModePublishableKeyName))
	}

	return ""
}

// GetDisplayName returns the account display name of the user
func (p *Profile) GetDisplayName() string {
	if err := viper.ReadInConfig(); err == nil {
		return viper.GetString(p.GetConfigField(DisplayNameName))
	}

	return ""
}

// GetTerminalPOSDeviceID returns the device id from the config for Terminal quickstart to use
func (p *Profile) GetTerminalPOSDeviceID() string {
	if err := viper.ReadInConfig(); err == nil {
		return viper.GetString(p.GetConfigField("terminal_pos_device_id"))
	}

	return ""
}

// GetConfigField returns the configuration field for the specific profile
func (p *Profile) GetConfigField(field string) string {
	return p.ProfileName + "." + field
}

// RegisterAlias registers an alias for a given key.
func (p *Profile) RegisterAlias(alias, key string) {
	viper.RegisterAlias(p.GetConfigField(alias), p.GetConfigField(key))
}

// WriteConfigField updates a configuration field and writes the updated
// configuration to disk.
func (p *Profile) WriteConfigField(field, value string) error {
	viper.Set(p.GetConfigField(field), value)
	return viper.WriteConfig()
}

// DeleteConfigField deletes a configuration field.
func (p *Profile) DeleteConfigField(field string) error {
	v, err := removeKey(viper.GetViper(), p.GetConfigField(field))
	if err != nil {
		return err
	}

	return p.writeProfile(v)
}

func (p *Profile) writeProfile(runtimeViper *viper.Viper) error {
	profilesFile := viper.ConfigFileUsed()

	err := makePath(profilesFile)
	if err != nil {
		return err
	}

	if p.DeviceName != "" {
		runtimeViper.Set(p.GetConfigField(DeviceNameName), strings.TrimSpace(p.DeviceName))
	}

	if p.LiveModeAPIKey != "" {
		expiresAt := getKeyExpiresAt()

		// store redacted key in config
		runtimeViper.Set(p.GetConfigField(LiveModeAPIKeyName), RedactAPIKey(strings.TrimSpace(p.LiveModeAPIKey)))
		runtimeViper.Set(p.GetConfigField(LiveModeExpiresAtName), expiresAt)

		// store actual key in secure keyring
		p.storeLivemodeValue(LiveModeAPIKeyName, strings.TrimSpace(p.LiveModeAPIKey), "Live mode API key")
		p.storeLivemodeValue(LiveModeExpiresAtName, expiresAt, "Live mode API key expirary")
	}

	if p.LiveModePublishableKey != "" {
		// store redacted key in config
		runtimeViper.Set(p.GetConfigField(LiveModePublishableKeyName), RedactAPIKey(strings.TrimSpace(p.LiveModePublishableKey)))

		// store actual key in secure keyring
		p.storeLivemodeValue(LiveModePublishableKeyName, strings.TrimSpace(p.LiveModePublishableKey), "Live mode publishable key")
	}

	if p.TestModeAPIKey != "" {
		runtimeViper.Set(p.GetConfigField(TestModeAPIKeyName), strings.TrimSpace(p.TestModeAPIKey))
		runtimeViper.Set(p.GetConfigField(TestModeExpiresAtName), getKeyExpiresAt())
	}

	if p.TestModePublishableKey != "" {
		runtimeViper.Set(p.GetConfigField(TestModePublishableKeyName), strings.TrimSpace(p.TestModePublishableKey))
	}

	if p.DisplayName != "" {
		runtimeViper.Set(p.GetConfigField(DisplayNameName), strings.TrimSpace(p.DisplayName))
	}

	if p.AccountID != "" {
		runtimeViper.Set(p.GetConfigField(AccountIDName), strings.TrimSpace(p.AccountID))
	}

	runtimeViper.MergeInConfig()

	// Do this after we merge the old configs in
	if p.TestModeAPIKey != "" {
		runtimeViper = p.safeRemove(runtimeViper, "secret_key")
		runtimeViper = p.safeRemove(runtimeViper, "api_key")
	}

	if p.TestModePublishableKey != "" {
		runtimeViper = p.safeRemove(runtimeViper, "publishable_key")
	}

	runtimeViper.SetConfigFile(profilesFile)

	// Ensure we preserve the config file type
	runtimeViper.SetConfigType(filepath.Ext(profilesFile))

	err = runtimeViper.WriteConfig()
	if err != nil {
		return err
	}

	return nil
}

func (p *Profile) safeRemove(v *viper.Viper, key string) *viper.Viper {
	if v.IsSet(p.GetConfigField(key)) {
		newViper, err := removeKey(v, p.GetConfigField(key))
		if err == nil {
			// I don't want to fail the entire login process on not being able to remove
			// the old secret_key field so if there's no error
			return newViper
		}
	}

	return v
}

func livemodeKeyField(livemode bool) string {
	if livemode {
		return LiveModeAPIKeyName
	}

	return TestModeAPIKeyName
}

func getKeyExpiresAt() string {
	return time.Now().AddDate(0, 0, KeyValidInDays).UTC().Format(DateStringFormat)
}
