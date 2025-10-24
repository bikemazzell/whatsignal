package types

type MediaType string

const (
	MediaTypeImage MediaType = "Image"
	MediaTypeFile  MediaType = "File"
	MediaTypeVoice MediaType = "Voice"
	MediaTypeVideo MediaType = "Video"
)

const (
	APIBase             = "/api"
	EndpointSendText    = "/sendText"
	EndpointSendSeen    = "/sendSeen"
	EndpointStartTyping = "/startTyping"
	EndpointStopTyping  = "/stopTyping"
	EndpointSendImage   = "/sendImage"
	EndpointSendFile    = "/sendFile"
	EndpointSendVoice   = "/sendVoice"
	EndpointSendVideo   = "/sendVideo"
	EndpointReaction    = "/reaction"

	// Contact endpoints
	EndpointContactsAll = "/contacts/all"
	EndpointContacts    = "/contacts"

	// Group endpoints
	EndpointGroups    = "/groups"
	EndpointGroupsAll = "/groups"
)
