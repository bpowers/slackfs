package main

type TimestampS int64
type TimestampNS string

type BotID string
type ChannelID string
type CreatorID string
type GroupID string
type UserID string
type TeamID string
type IMID string

type Bot struct {
	ID        BotID             `json:"id"`
	Name      string            `json:"name"`
	Icons     map[string]string `json:"icons"`
	IsDeleted bool              `json:"deleted"`
}

type MessageType string
type MessageSubtype string

type Attachment struct {
	ID          uint64   `json:"id"`
	Title       string   `json:"title"`
	TitleLink   string   `json:"title_link"`
	FromURL     string   `json:"from_url"`
	Fallback    string   `json:"fallback"`
	Text        string   `json:"text"`
	Pretext     string   `json:"pretext"`
	MarkdownIn  []string `json:"mrkdwn_in"`
	ImageBytes  string   `json:"image_bytes"`
	ImageHeight int      `json:"image_height"`
	ImageWidth  int      `json:"image_width"`
	ImageURL    string   `json:"image_url"`
}

type Message struct {
	Type      MessageType    `json:"type"`
	Subtype   MessageSubtype `json:"subtype"`
	User      UserID         `json:"user"`
	Text      string         `json:"text"`
	Timestamp TimestampNS    `json:"ts"`

	// optional

	BotID       BotID        `json:"bot_id"`
	Attachments []Attachment `json:"attachments"`
	Permalink   string       `json:"permalink"`
	PinnedTo    []ChannelID  `json:"pinned_to"`
}

type Topic struct {
	Creator string     `json:"creator"`
	LastSet TimestampS `json:"last_set"`
	Value   string     `json:"value"`
}

type PinnedItem struct {
	Channel ChannelID `json:"channel"`
}

type Channel struct {
	ID         ChannelID  `json:"id"`
	Name       string     `json:"name"`
	Created    TimestampS `json:"created"`
	CreatorID  CreatorID  `json:"creator"`
	IsArchived bool       `json:"is_archived"`
	IsChannel  bool       `json:"is_channel"`
	IsGeneral  bool       `json:"is_general"`
	IsMember   bool       `json:"is_member"`

	// only available when IsMember is true

	LastRead           TimestampNS `json:"last_read"`
	Latest             Message     `json:"latest"`
	Members            []UserID    `json:"members"`
	Purpose            Topic       `json:"purpose"`
	Topic              Topic       `json:"topic"`
	UnreadCount        int         `json:"unread_count"`
	UnreadCountDisplay int         `json:"unread_count_display"`
}

type Group struct {
	ID                 GroupID     `json:"id"`
	Name               string      `json:"name"`
	Created            TimestampS  `json:"created"`
	CreatorID          CreatorID   `json:"creator"`
	IsArchived         bool        `json:"is_archived"`
	IsGroup            bool        `json:"is_group"`
	IsOpen             bool        `json:"is_open"`
	LastRead           TimestampNS `json:"last_read"`
	Latest             Message     `json:"latest"`
	Members            []UserID    `json:"members"`
	Purpose            Topic       `json:"purpose"`
	UnreadCount        int         `json:"unread_count"`
	UnreadCountDisplay int         `json:"unread_count_display"`
}

type IM struct {
	ID                 IMID        `json:"id"`
	User               UserID      `json:"user"`
	Created            TimestampS  `json:"created"`
	IsIM               bool        `json:"is_im"`
	IsOpen             bool        `json:"is_open"`
	LastRead           TimestampNS `json:"last_read"`
	UnreadCount        int         `json:"unread_count"`
	UnreadCountDisplay int         `json:"unread_count_display"`
}

type Self struct {
	ID             UserID                 `json:"id"`
	Name           string                 `json:"name"`
	Created        TimestampS             `json:"created"`
	ManualPresence string                 `json:"manual_presence"`
	Prefs          map[string]interface{} `json:"prefs"`
}

type Team struct {
	ID                   TeamID                 `json:"id"`
	Name                 string                 `json:"name"`
	Domain               string                 `json:"domain"`
	EmailDomain          string                 `json:"email_domain"`
	Icon                 map[string]string      `json:"icon"`
	MsgEditWindowMins    int                    `json:"msg_edit_window_mins"`
	OverStoragePlanLimit bool                   `json:"over_storage_plan_limit"`
	Plan                 string                 `json:"plan"`
	Prefs                map[string]interface{} `json:"prefs"`
}

type User struct {
	ID       UserID `json:"id"`
	Name     string `json:"name"`
	RealName string `json:"real_name"`
	Presence string `json:"presence"`
	Color    string `json:"color"`
	Deleted  bool   `json:"deleted"`
	HasFiles bool   `json:"has_files"`
	Profile  struct {
		Email              string `json:"email"`
		FirstName          string `json:"first_name"`
		LastName           string `json:"last_name"`
		RealName           string `json:"real_name"`
		RealNameNormalized string `json:"real_name_normalized"`
		Skype              string `json:"skype"`
		Title              string `json:"title"`
		Image24            string `json:"image_24"`
		Image32            string `json:"image_32"`
		Image48            string `json:"image_48"`
		Image72            string `json:"image_72"`
		Image192           string `json:"image_192"`
		ImageOriginal      string `json:"image_original"`
	} `json:"profile"`
	IsAdmin           bool       `json:"is_admin"`
	IsBot             bool       `json:"is_bot"`
	IsOwner           bool       `json:"is_owner"`
	IsPrimaryOwner    bool       `json:"is_primary_owner"`
	IsRestricted      bool       `json:"is_restricted"`
	IsUltraRestricted bool       `json:"is_ultra_restricted"`
	TimeZone          string     `json:"tz"`
	TimeZoneLabel     string     `json:"tz_label"`
	TimeZoneOffset    TimestampS `json:"tz_offset"`
}

type RTMStartResponse struct {
	OK   bool   `json:"ok"`
	URL  string `json:"url"`
	Self Self   `json:"self"`
	Team Team   `json:"team"`

	Users    []*User    `json:"users"`
	Channels []*Channel `json:"channels"`
	Groups   []*Group   `json:"groups"`
	IMs      []*IM      `json:"ims"`
	Bots     []*Bot     `json:"bots"`

	Error         string      `json:"error"`
	CacheVersion  string      `json:"cache_version"`
	LatestEventTS TimestampNS `json:"latest_event_ts"`
}
