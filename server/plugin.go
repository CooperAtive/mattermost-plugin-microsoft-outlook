package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sync"

	graph "github.com/jkrecek/msgraph-go"
	"github.com/mattermost/mattermost-server/mlog"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"

	"golang.org/x/oauth2"
)

const (
	GRAPH_TOKEN_KEY   = "_graphtoken"
	OUTLOOK_EMAIL_KEY = "_outlookemail"
)

type Plugin struct {
	plugin.MattermostPlugin

	// BotId of the created bot account.
	BotUserID string

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

func (p *Plugin) graphConnect(token oauth2.Token) *graph.Client {
	client := graph.NewClient(p.getOAuthConfig(), &token)

	return client
}

func (p *Plugin) OnActivate() error {
	config := p.getConfiguration()

	if err := config.IsValid(); err != nil {
		return err
	}

	user, err := p.API.GetUserByUsername(config.Username)
	if err != nil {
		mlog.Error(err.Error())
		return fmt.Errorf("Unable to find user with configured username: %v", config.Username)
	}

	p.BotUserID = user.Id

	return nil
}

func (p *Plugin) getOAuthConfig() *oauth2.Config {
	config := p.getConfiguration()

	authURL, _ := url.Parse("https://login.microsoftonline.com/")
	tokenURL, _ := url.Parse("https://login.microsoftonline.com/")

	authURL.Path = path.Join(authURL.Path, "common", "oauth2", "v2.0", "authorize")
	tokenURL.Path = path.Join(tokenURL.Path, "common", "oauth2", "v2.0", "token")

	conf := oauth2.Config{
		ClientID:     config.AADClientID,
		ClientSecret: config.AADClientSecret,
		Scopes:       []string{"calendars.readwrite", "mail.readwrite", "mail.send", "user.read"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL.String(),
			TokenURL: tokenURL.String(),
		},
		RedirectURL: "http://localhost:8065/plugins/outlook/oauth/complete",
	}

	return &conf
}

// OutlookUserInfo stores a token and other metadata about a connected outlook account
type OutlookUserInfo struct {
	UserID string
	Email  string
	Token  *oauth2.Token
}

func (p *Plugin) storeOutlookUserInfo(info *OutlookUserInfo) error {
	config := p.getConfiguration()

	encryptedToken, err := encrypt([]byte(config.EncryptionKey), info.Token.AccessToken)
	if err != nil {
		return err
	}

	info.Token.AccessToken = encryptedToken

	jsonInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}

	if err := p.API.KVSet(info.UserID+GRAPH_TOKEN_KEY, jsonInfo); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) storeEmailToUserIDMapping(email, userID string) error {
	if err := p.API.KVSet(email+OUTLOOK_EMAIL_KEY, []byte(userID)); err != nil {
		return fmt.Errorf("Encountered error saving outlook email mapping")
	}
	return nil
}

func (p *Plugin) getEmailToUserIDMapping(email string) string {
	userID, _ := p.API.KVGet(email + OUTLOOK_EMAIL_KEY)
	return string(userID)
}

func (p *Plugin) CreateBotDMPost(userID, message, postType string) *model.AppError {
	channel, err := p.API.GetDirectChannel(userID, p.BotUserID)
	if err != nil {
		mlog.Error("Couldn't get bot's DM channel", mlog.String("user_id", userID))
		return err
	}

	post := &model.Post{
		UserId:    p.BotUserID,
		ChannelId: channel.Id,
		Message:   message,
		Type:      postType,
	}

	if _, err := p.API.CreatePost(post); err != nil {
		mlog.Error(err.Error())
		return err
	}

	return nil
}
