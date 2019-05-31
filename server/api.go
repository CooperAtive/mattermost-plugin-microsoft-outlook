package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	graph "github.com/jkrecek/msgraph-go"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"

	"golang.org/x/oauth2"
)

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	config := p.getConfiguration()

	if err := config.IsValid(); err != nil {
		http.Error(w, "This plugin is not configured.", http.StatusNotImplemented)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch path := r.URL.Path; path {
	case "/oauth/connect":
		p.connectUserToGraph(w, r)
	case "/oauth/complete":
		p.completeConnectUserToGraph(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) connectUserToGraph(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	conf := p.getOAuthConfig()

	state := fmt.Sprintf("%v_%v", model.NewId()[0:15], userID)

	p.API.KVSet(state, []byte(state))

	url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)

	http.Redirect(w, r, url, http.StatusFound)
}

func (p *Plugin) completeConnectUserToGraph(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	conf := p.getOAuthConfig()

	code := r.URL.Query().Get("code")
	if len(code) == 0 {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")

	if storedState, err := p.API.KVGet(state); err != nil {
		fmt.Println(err.Error())
		http.Error(w, "missing stored state", http.StatusBadRequest)
		return
	} else if string(storedState) != state {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	userID := strings.Split(state, "_")[1]

	p.API.KVDelete(state)

	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	graphClient := &graph.Client{}
	graphClient.SetVersion("2.0")
	graphClient = p.graphConnect(*tok)

	me, err := graphClient.GetMe()
	if err != nil {
		log.Println(err)
	}

	userEmail := me.UserPrincipalName

	userInfo := &OutlookUserInfo{
		UserID: userID,
		Email:  userEmail,
		Token:  tok,
	}

	if err := p.storeOutlookUserInfo(userInfo); err != nil {
		fmt.Println(err.Error())
		http.Error(w, "Unable to connect user to Outlook", http.StatusInternalServerError)
		return
	}

	if err := p.storeEmailToUserIDMapping(userInfo.Email, userID); err != nil {
		fmt.Println(err.Error())
	}

	message := fmt.Sprintf("### Welcome to the Outlook plugin!\n"+
		"Here is some info to prove we got you logged in\n"+
		"Email: %s \n"+
		"Name: %s \n", userEmail, me.DisplayName)
	p.CreateBotDMPost(userID, message, "custom_outlook_welcome")

	html := `
		<!DOCTYPE html>
		<html>
			<head>
				<script>
					window.close();
				</script>
			</head>
			<body>
				<p>Completed connecting to Outlook. Please close this window.</p>
			</body>
		</html>
		`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
