package endpoints

// Reply-keyboard button texts. Used both as display labels and filter strings.
const (
	BtnInstallGuide    = "Инструкция по установке"
	BtnCheckConnection = "проверить подключение"
	BtnSettings        = "Настройки"
	BtnInviteFriend    = "Пригласить друга"
	BtnUpgradeLevel    = "Поднять уровень"
	BtnHowEncryption   = "Как работает шифрование"
)

// Inline callback unique names used for routing.
const (
	UniqueShowContacts    = "show_contacts"
	UniqueToggleDeleted   = "toggle_deleted"
	UniqueToggleEdited    = "toggle_edited"
	UniqueToggleSelfMedia = "toggle_self_media"
	UniqueToggleExtExport = "toggle_ext_export"
	UniqueBonusSelect     = "bonus_select"
	UniqueBonusDetails    = "bonus_details"
	
	UniqueMirrorList      = "mirror_list"
	UniqueMirrorDetails   = "mirror_details"
	UniqueMirrorDelete    = "mirror_delete"
	UniqueMirrorDeleteConfirm = "mirror_delete_confirm"
	UniqueMirrorDeleteCancel = "mirror_delete_cancel"
	UniqueMirrorCreate    = "mirror_create"

	UniqueBonusBack       = "bonus_back"
	UniqueBonusMoney      = "bonus_money"
	UniqueBonusDiscount   = "bonus_discount"
	UniqueBonusLevels     = "bonus_levels"
	UniqueWhatLevels      = "what_levels"
	UniqueUpgradeLevel    = "upgrade_level"
	UniqueDeleteConfirm   = "delete_confirm"
	UniqueDeleteCancel    = "delete_cancel"
)

// Custom emoji Sticker IDs for inline button state indicators.
const (
	// EmojiSettingOn is the "enabled" state indicator (BMP, 1 UTF-16 unit).
	EmojiSettingOn = "5411197345968701560"
	// EmojiSettingOff is the "disabled" state indicator (non-BMP surrogate pair, 2 UTF-16 units).
	EmojiSettingOff = "6323476982646441555"

	// EmojiMirrorActive is the "active" state indicator (BMP, 1 UTF-16 unit).
	EmojiMirrorActive = "6237651574588445185"

	// EmojiMirrorInactive is the "inactive" state indicator (non-BMP surrogate pair, 2 UTF-16 units).
	EmojiMirrorInactive = "6269142369291995999"
)

// Telegram file IDs for static media assets.
const (
	InstallationAnimationFileID = "CgACAgIAAxkBAAIDjWnsbpjwU4FC2qaf-ddh4Spf5MqmAAKQdgACRDmhSdbGAStskkqKOwQ"
	HowEncryptionPhotoFileID    = "AgACAgIAAxkBAAIDnGnt0Oy1tmWtyTde2cxaoiy81zZyAALXFmsbBz9pS9gHPaDlZK_FAQADAgADeQADOwQ"
)

// Local static media paths used for mirror bots because Telegram file_id is bot-specific.
const (
	InstallationAnimationStaticPath = "static/setupInstruction.gif"
	HowEncryptionPhotoStaticPath    = "static/cipherExample.png"
)

// Stable keys for bot-specific cached Telegram file IDs.
const (
	MirrorFileInstallationAnimation = "installation_animation"
	MirrorFileHowEncryptionPhoto    = "how_encryption_photo"
)

// ReferralBonusRub is the cash bonus amount per referred user, in rubles.
const ReferralBonusRub = 5

const Month = 30 * 24 * 60 * 60

// Referral bonus durations
const (
	ReferralLevelsDurationSec = 6 * Month
	ReferralDiscountDurationSec = 1 * Month
)

// Referral bonus thresholds
const (
	// ReferralBonusThresholdLevels is how many referred users unlock one bonus level grant.
	ReferralBonusThresholdLevels = 2
	// ReferralBonusLevelsPerUnlock is how many user levels are stored per unlock (one row may cover several referrals).
	ReferralBonusLevelsPerUnlock = 1
)

// BotUsername is shown in several messages as a mention.
const BotUsername = "@MajorFanOfInnokentii_bot"

// Routing key for all outgoing Telegram messages via AMQP.
const OutgoingRoutingKey = "telegram.message.send"
