package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/jasondcamp/incidentbot/data"
	"github.com/jasondcamp/incidentbot/handler"
	"github.com/jasondcamp/incidentbot/util"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var (
	token          string
	challenge      string
	listenPort     int
	debug          bool
	admins         string
	pruneEnabled   bool
	pruneInterval  int
	pruneExpire    int
	dbUsername	string
	dbPassword	string
	dbHost		string
	dbPort		string
	dbDatabase	string
)

func main() {
	flag.StringVar(&token, "token", util.LookupEnvOrString("SLACK_TOKEN", ""), "Slack API Token")
	flag.StringVar(&challenge, "challenge", util.LookupEnvOrString("SLACK_CHALLENGE", ""), "Slack verification token")
	flag.IntVar(&listenPort, "listen-port", util.LookupEnvOrInt("LISTEN_PORT", 4512), "Listen port")
	flag.BoolVar(&debug, "debug", util.LookupEnvOrBool("DEBUG", false), "Debug mode")
	flag.StringVar(&admins, "admins", util.LookupEnvOrString("SLACK_ADMINS", ""), "Turn on administrative commands for specific admins, comma separated list")
	flag.BoolVar(&pruneEnabled, "prune-enabled", util.LookupEnvOrBool("PRUNE_ENABLED", true), "Enable pruning available resources automatically")
	flag.IntVar(&pruneInterval, "prune-interval", util.LookupEnvOrInt("PRUNE_INTERVAL", 1), "Automatic pruning interval in hours")
	flag.IntVar(&pruneExpire, "prune-expire", util.LookupEnvOrInt("PRUNE_EXPIRE", 168), "Automatic prune expiration time in hours")

	flag.StringVar(&dbUsername, "db-username", util.LookupEnvOrString("DB_USERNAME", "incidentbot"), "Database Username")
	flag.StringVar(&dbPassword, "db-password", util.LookupEnvOrString("DB_PASSWORD", ""), "Database Password")
	flag.StringVar(&dbHost, "db-host", util.LookupEnvOrString("DB_HOST", "localhost"), "Database Host")
	flag.StringVar(&dbPort, "db-port", util.LookupEnvOrString("DB_PORT", "3306"), "Database Port")
	flag.StringVar(&dbDatabase, "db-database", util.LookupEnvOrString("DB_DATABASE", "incidentbot"), "Database Database")
	
	flag.Parse()

	// Make sure required vars are set
	if token == "" {
		log.Error("token is required")
		return
	}
	if challenge == "" {
		log.Error("challenge is required")
		return
	}

	dbConnectionInfo := new(data.DbConnectionInfo)
	if dbUsername == "" {
		log.Error("db-username is required")
		return
	} else {
		dbConnectionInfo.Username = dbUsername
	}

	if dbPassword == "" {
		log.Error("db-password is required")
		return
	} else {
		dbConnectionInfo.Password = dbPassword
	}

	if dbHost == "" {
		log.Error("db-host is required")
		return
	} else {
		dbConnectionInfo.Host = dbHost
	}

	if dbPort == "" {
		log.Error("db-port is required")
		return
	} else {
		dbConnectionInfo.Port = dbPort
	}

	if dbDatabase == "" {
		log.Error("db-database is required")
		return
	} else {
		dbConnectionInfo.Database = dbDatabase
	}

	api := slack.New(token, slack.OptionDebug(debug))

	if pruneEnabled {
		// Prune inactive resources
		log.Infof("Automatic Pruning is enabled.")
		go func() {
			for {
				time.Sleep(time.Duration(pruneInterval) * time.Hour)
//				err := data.PruneInactiveResources(pruneExpire)
//				if err != nil {
//					log.Errorf("Error pruning resources: %+v", err)
//				} else {
//					log.Infof("Pruned resources")
//				}
			}
		}()

	} else {
		log.Infof("Automatic pruning is disabled.")
	}

	handler := handler.New(api, util.ParseAdmins(admins), dbConnectionInfo)

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		body := buf.String()

		api.Debugf("Request: %s", body)

		eventsAPIEvent, err := slackevents.ParseEvent(
			json.RawMessage(body),
			slackevents.OptionVerifyToken(
				&slackevents.TokenComparator{VerificationToken: challenge},
			),
		)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorf("%+v", err)
			return
		}

		switch eventsAPIEvent.Type {
		case slackevents.URLVerification:
			var r *slackevents.ChallengeResponse
			err := json.Unmarshal([]byte(body), &r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.Header().Set("Content-Type", "text")
			w.Write([]byte(r.Challenge))
		case slackevents.CallbackEvent:
			err := handler.CallbackEvent(eventsAPIEvent)
			if err != nil {
				log.Errorf("%+v", err)
			}
		default:
		}
	})

	log.Infof("Server listening on port %d", listenPort)

	http.ListenAndServe(fmt.Sprintf(":%v", listenPort), nil)

}
