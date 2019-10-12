package postmark

import (
	"fmt"
)

// Server represents a server registered in your Postmark account
type Server struct {
	// ID of server
	ID int64
	// Name of server
	Name string
	// APITokens associated with server.
	APITokens []string `json:"ApiTokens"`
	// ServerLink to your server overview page in Postmark.
	ServerLink string
	// Color of the server in the rack screen. Purple Blue Turquoise Green Red Yellow Grey
	Color string
	// SMTPAPIActivated specifies whether or not SMTP is enabled on this server.
	SMTPAPIActivated bool `json:"SmtpApiActivated"`
	// RawEmailEnabled allows raw email to be sent with inbound.
	RawEmailEnabled bool
	// InboundAddress is the inbound email address
	InboundAddress string
	// InboundHookURL to POST to every time an inbound event occurs.
	InboundHookURL string `json:"InboundHookUrl"`
	// BounceHookURL to POST to every time a bounce event occurs.
	BounceHookURL string `json:"BounceHookUrl"`
	// OpenHookURL to POST to every time an open event occurs.
	OpenHookURL string `json:"OpenHookUrl"`
	// PostFirstOpenOnly - If set to true, only the first open by a particular recipient will initiate the open webhook. Any
	// subsequent opens of the same email by the same recipient will not initiate the webhook.
	PostFirstOpenOnly bool
	// TrackOpens indicates if all emails being sent through this server have open tracking enabled.
	TrackOpens bool
	// InboundDomain is the inbound domain for MX setup
	InboundDomain string
	// InboundHash is the inbound hash of your inbound email address.
	InboundHash string
	// InboundSpamThreshold is the maximum spam score for an inbound message before it's blocked.
	InboundSpamThreshold int64
}

///////////////////////////////////////
///////////////////////////////////////

// GetServer fetches a specific server via serverID
func (client *Client) GetServer(serverID string) (Server, error) {
	res := Server{}
	err := client.doRequest(parameters{
		Method:    "GET",
		Path:      fmt.Sprintf("servers/%s", serverID),
		TokenType: accountToken,
	}, &res)
	return res, err
}

///////////////////////////////////////
///////////////////////////////////////

// EditServer updates details for a specific server with serverID
func (client *Client) EditServer(serverID string, server Server) (Server, error) {
	res := Server{}
	err := client.doRequest(parameters{
		Method:    "PUT",
		Path:      fmt.Sprintf("servers/%s", serverID),
		TokenType: accountToken,
	}, &res)
	return res, err
}
