package types

type MediaType string

const (
	MediaTypeImage MediaType = "Image"
	MediaTypeFile  MediaType = "File"
	MediaTypeVoice MediaType = "Voice"
	MediaTypeVideo MediaType = "Video"
)

const (
	APIBase             = "/api/%s"
	EndpointSendText    = "/sendText"
	EndpointSendSeen    = "/sendSeen"
	EndpointStartTyping = "/startTyping"
	EndpointStopTyping  = "/stopTyping"
	EndpointSendImage   = "/sendImage"
	EndpointSendFile    = "/sendFile"
	EndpointSendVoice   = "/sendVoice"
	EndpointSendVideo   = "/sendVideo"
)
