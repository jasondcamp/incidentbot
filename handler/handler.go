package handler

import (
	"fmt"
	"strings"
	"time"
        "database/sql"
	"strconv"

	"github.com/jasondcamp/incidentbot/data"
	e "github.com/jasondcamp/incidentbot/err"
	"github.com/jasondcamp/incidentbot/models"
	"github.com/jasondcamp/incidentbot/util"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
        _ "github.com/go-sql-driver/mysql"
)

type Handler struct {
	client *slack.Client
	data   data.Manager

	admins []string
	dbConnection data.dbConnectionInfo
}

type EventAction struct {
	Event  *slackevents.MessageEvent
	Action string
}

func New(client *slack.Client, data data.Manager,  admins []string, dbConnectionInfo dbConnectionInfo) *Handler {
	return &Handler{
		client: client,
		data:   data,
		admins: admins,
		dbConnectionInfo: dbConnectionInfo,
	}
}

func (h *Handler) CallbackEvent(event slackevents.EventsAPIEvent) error {
	// First, we normalize the incoming event
	var ea *EventAction
	innerEvent := event.InnerEvent
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		ea = &EventAction{
			Event: &slackevents.MessageEvent{
				Type:            ev.Type,
				User:            ev.User,
				Text:            ev.Text,
				TimeStamp:       ev.TimeStamp,
				ThreadTimeStamp: ev.ThreadTimeStamp,
				Channel:         ev.Channel,
				EventTimeStamp:  ev.EventTimeStamp,
				UserTeam:        ev.UserTeam,
				SourceTeam:      ev.SourceTeam,
			},
		}
	case *slackevents.MessageEvent:
		if h.shouldHandle(ev) {
			ea = &EventAction{
				Event: ev,
			}
		}
	}
	if ea == nil {
		return nil
	}

	// Now we determine what to do with it
	ea.Action = h.getAction(ea.Event.Text)
	log.Infof("Parsing action: %+v", ea.Action)
	switch ea.Action {
	case "new", "start":
		return h.newIncident(ea)
	case "archive", "single_archive":
		return h.archiveIncident(ea)
	case "update-summary":
		return h.updateSummary(ea)
	case "update-severity":
		return h.updateSeverity(ea)
        case "update-commander":
                return h.updateCommander(ea)
	case "update-manager":
		return h.updateManager(ea)
	case "update-state":
		return h.updateState(ea)
	case "status", "status_inroom":
		return h.showStatus(ea)

	case "hello":
		return h.sayHello(ea)
	case "prune", "prune_dm":
		return h.prune(ea)
	case "help", "help_dm":
		return h.help(ea)
	default:
		log.Errorf("Unknown action: %+v", ea.Action)
		return h.reply(ea, "I'm sorry, I don't know what to do with that request", false)
	}
}

func (h *Handler) shouldHandle(ev *slackevents.MessageEvent) bool {
	if ev.BotID != "" {
		return false
	}
	if ev.ChannelType != "im" {
		return false
	}

	return true
}

func (h *Handler) sayHello(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Hello"+u.Name+".", false))
	return nil
}

func (h *Handler) newIncident(ea *EventAction) error {
        ev := ea.Event

       // Generate a new incident ID
	db, err := sql.Open("mysql", "incidentbot:AVNS_67iDl956qEd8uYA_wNT@tcp(batchco-db-do-user-1953615-0.b.db.ondigitalocean.com:25060)/incidentbot")
	defer db.Close()

	if err != nil {
		log.Error(err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("Configuration error, please check logs", false))
		return nil
	}

	var incident_id string
	err2 := db.QueryRow("SELECT id from incidents order by id desc limit 1").Scan(&incident_id)
	switch {
	case err2 == sql.ErrNoRows:
		incident_id = "1"
	case err2 != nil:
		log.Error(err2)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("Configuration error, please check logs", false))
		return nil
	default:
       		incident_int, _ := strconv.Atoi(incident_id)
		incident_id = strconv.Itoa(incident_int + 1)	
	}

	// Communicate to client
        h.client.PostMessage(ev.Channel, slack.MsgOptionText(":rotating_light: Creating a new incident - #" + incident_id, false))

        // Create new channel for incident
        h.client.PostMessage(ev.Channel, slack.MsgOptionText(":white_check_mark: Creating a new channel - #incident-" + incident_id, false))
        response, err6 := h.client.CreateConversation("incident-" + incident_id, false)
        if err6 != nil {
                log.Error(err6)
        }
        incident_channel_id := response.ID

	// Create new incident
	user, _ := h.getUser(ea.Event.User)
	sql := "INSERT INTO incidents (id, incident_opened_by, state, chat_room) VALUES (" + incident_id + ",'" + user.ID  + "', 'new', '" + incident_channel_id + "' )"
	_, err4 := db.Exec(sql)

	if err4 != nil {
		log.Error(err4)
	}

	// Add incidentbot into new channel
        h.client.PostMessage(ev.Channel, slack.MsgOptionText(":white_check_mark: Adding users to #incident-" + incident_id, false))
	_, _, _, err7 := h.client.JoinConversation(incident_channel_id)
	if err7 != nil {
		log.Error(err7)
	}

	// Add creator into new channel
	h.client.InviteUsersToConversation(incident_channel_id, user.ID)

        util.LogEvent(incident_id, user.ID, "cmd_new", "Created new incident from slack")
        return nil
}

func (h *Handler) archiveIncident(ea *EventAction) error {
	// This function will archive a chat room
	ev := ea.Event
	user, _ := h.getUser(ea.Event.User)
	matches := h.getMatches(ea.Action, ev.Text)

	if len(matches) == 0 {
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(":sos: Usage: archive <incident_id>", false))
	} else {
        	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Archiving incident chat - incident - #" + matches[0], false))

		db, err := sql.Open("mysql", "incidentbot:AVNS_67iDl956qEd8uYA_wNT@tcp(batchco-db-do-user-1953615-0.b.db.ondigitalocean.com:25060)/incidentbot")
		defer db.Close()

		if err != nil {
			log.Error(err)
			h.client.PostMessage(ev.Channel, slack.MsgOptionText("Configuration error, please check logs", false))
		return nil
		}

		var incident_chat_room string
		db.QueryRow("select chat_room from incidents where id=" + matches[0]).Scan(&incident_chat_room)
		h.client.ArchiveConversation(incident_chat_room)
	}

	util.LogEvent(matches[0], user.ID, "cmd_archive", "Archived chat room")
	return nil
}

func (h *Handler) showStatus(ea *EventAction) error {
	// This function will show the current status of the incident
	ev := ea.Event
	matches := h.getMatches(ea.Action, ev.Text)

	incident_id := h.GetIncidentId(ea, matches)

	incident := h.GetIncident(incident_id)

	current_status := "```Incident # : " + strconv.Itoa(incident.Id) + "\n"
	current_status += "Summary    : " + incident.Summary.String + "\n"
	current_status += "Opened By  : " + incident.Openedby.String + "\n"
	current_status += "Opened On  : " + incident.Created.String + "\n"
	current_status += "Commander  : " + incident.Commander.String + "\n"
	current_status += "Manager    : " + incident.Manager.String + "\n"
	current_status += "Severity   : " + incident.Severity.String + "\n"
	current_status += "State      : " + incident.State.String + "\n"
	current_status += "Chat Room  : " + incident.Chat_room.String + "\n"
	current_status += "```"

	h.client.PostMessage(ev.Channel, slack.MsgOptionText(current_status, false))

	return nil
}

func (h *Handler) GetIncidentId(ea *EventAction, matches []string) string {
	// This function will get the incident ID from the event or Action
	ev := ea.Event

	// See if we're in an incident room
	log.Error(ev.Channel)
	if ev.Channel == "incident-30" {
		log.Error("JASON")
	} else {
		return matches[0]
	}

	return ""
}

func (h *Handler) updateSummary(ea *EventAction) error {
	// This function will update the summary of an incident
	ev := ea.Event
	user, _ := h.getUser(ea.Event.User)
	matches := h.getMatches(ea.Action, ev.Text)

	// Post update messages
	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Updating summary for incident - #" + matches[0], false))

	h.updateIncidentField(matches[0], "summary", matches[1])

	// Update incident channel
        h.client.PostMessage("incident-" + matches[0], slack.MsgOptionText("Incident summary updated: " + matches[1], false))

	util.LogEvent(matches[0], user.ID, "update_summary", "Updated summary to: " + matches[1])
	return nil
}

func (h *Handler) updateSeverity(ea *EventAction) error {
	// This function will update the summary of an incident
	ev := ea.Event
	user, _ := h.getUser(ea.Event.User)
	matches := h.getMatches(ea.Action, ev.Text)

	// Make sure it's a valid severity
	severities := []string{"SEV0", "SEV1", "SEV2", "SEV3", "SEV4"}
        if !util.InSlice(severities, matches[1]) {
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("Error, severity type is not valid", false))
		return nil
	}

	// Post update messages
	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Updating severity for incident - #" + matches[0], false))

	h.updateIncidentField(matches[0], "severity", matches[1])

	// Update incident channel
	h.client.PostMessage("incident-" + matches[0], slack.MsgOptionText("Incident severity updated: " + matches[1], false))

	util.LogEvent(matches[0], user.ID, "update_severity", "Updated severity to: " + matches[1])
	return nil
}

func (h *Handler) updateCommander(ea *EventAction) error {
	// This function will update the commander of an incident
	ev := ea.Event
	user, _ := h.getUser(ea.Event.User)
	matches := h.getMatches(ea.Action, ev.Text)

	// Post update messages
	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Updating incident commander for incident - #" + matches[0], false))
	h.updateIncidentField(matches[0], "incident_commander", matches[1])

	// Update incident channel
	h.client.PostMessage("incident-" + matches[0], slack.MsgOptionText("Incident commander updated: <@" + matches[1] + ">", false))

	util.LogEvent(matches[0], user.ID, "update_commander", "Updated commander to: " + matches[1])
	return nil
}

func (h *Handler) updateManager(ea *EventAction) error {
	// This function will update the manager of an incident
	ev := ea.Event
	user, _ := h.getUser(ea.Event.User)
	matches := h.getMatches(ea.Action, ev.Text)

	// Post update messages
	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Updating incident manager for incident - #" + matches[0], false))
	h.updateIncidentField(matches[0], "incident_manager", matches[1])

	// Update incident channel
	h.client.PostMessage("incident-" + matches[0], slack.MsgOptionText("Incident manager updated: <@" + matches[1] + ">", false))

	util.LogEvent(matches[0], user.ID, "update_manager", "Updated manager to: " + matches[1])
	return nil
}

func (h *Handler) updateState(ea *EventAction) error {
        // This function will update the summary of an incident
        ev := ea.Event
        user, _ := h.getUser(ea.Event.User)
        matches := h.getMatches(ea.Action, ev.Text)

        // Make sure it's a valid state
        states := []string{"new", "inprogress", "stalled", "resolved"}
        if !util.InSlice(states, matches[1]) {
                h.client.PostMessage(ev.Channel, slack.MsgOptionText("Error, state type is not valid", false))
                return nil
        }

        // Post update messages
        h.client.PostMessage(ev.Channel, slack.MsgOptionText("Updating state for incident - #" + matches[0], false))

        h.updateIncidentField(matches[0], "state", matches[1])

        // Update incident channel
        h.client.PostMessage("incident-" + matches[0], slack.MsgOptionText("Incident state updated: " + matches[1], false))

        util.LogEvent(matches[0], user.ID, "update_state", "Updated state to: " + matches[1])
        return nil
}

func (h *Handler) getUserDisplay(user *models.User, mention bool) string {
	ret := fmt.Sprintf("*%s*", user.Name)
	if mention {
		ret = fmt.Sprintf("<@%s>", user.ID)
	}
	return ret
}

func (h *Handler) getUserDisplayWithDuration(reservation *models.Reservation, mention bool) string {
	user := reservation.User
	dur := getDuration(reservation.Time)

	ret := fmt.Sprintf("*%s* (%s)", user.Name, dur)
	if mention {
		ret = fmt.Sprintf("<@%s> (%s)", user.ID, dur)
	}
	return ret
}

func getDuration(t time.Time) string {
	duration := time.Since(t).Round(time.Minute)

	if duration < 1 {
		return "0m"
	}

	d := duration.String()

	return d[:len(d)-2]
}

// getMatches retrieves all capture group values from a given text for regex action
func (h *Handler) getMatches(action, text string) []string {
	ret := []string{}
	r := actions[action]
	matches := r.FindStringSubmatch(text)
	if len(matches) > 1 {
		for _, m := range matches[1:] {
			ret = append(ret, m)
		}
	}
	return ret
}

func (h *Handler) getUser(uid string) (*models.User, error) {
	u, err := h.client.GetUserInfo(uid)
	if err != nil {
		return nil, err
	}
	return &models.User{
		Name: u.Name,
		ID:   u.ID,
	}, nil
}

func (h *Handler) handleGetResourceError(ea *EventAction, err error) {
	msg := msgMustSpecifyResource
	if err == e.InvalidResourceFormat {
		msg = msgResourceImproperlyFormatted
	}
	h.errorReply(ea.Event.Channel, msg)
}

func (h *Handler) errorReply(channel, msg string) {
	if msg == "" {
		msg = msgIDontKnow
	}
	h.client.PostMessage(channel, slack.MsgOptionText(msg, false))
}

func (h *Handler) reply(ea *EventAction, msg string, address bool) error {
	// If message is in DM or does not start with addressing a user, capitalize the first letter
	if !address || ea.Event.ChannelType == "im" {
		msg = fmt.Sprintf("%s%s", strings.ToUpper(msg[:1]), msg[1:])
	}

	if ea.Event.ChannelType != "im" {
		user, err := h.getUser(ea.Event.User)
		if err != nil {
			return err
		}
		if address {
			msg = fmt.Sprintf("%s %s", h.getUserDisplay(user, true), msg)
		}
	}

	_, _, err := h.client.PostMessage(ea.Event.Channel, slack.MsgOptionText(msg, false))
	return err
}

func (h *Handler) announce(ea *EventAction, user *models.User, msg string) error {
	if user != nil {
		return h.sendDM(user, msg)
	}

	_, _, err := h.client.PostMessage(ea.Event.Channel, slack.MsgOptionText(msg, false))
	return err
}

func (h *Handler) sendDM(user *models.User, msg string) error {
	_, _, c, err := h.client.OpenIMChannel(user.ID)
	if err != nil {
		return err
	}
	_, _, err = h.client.PostMessage(c, slack.MsgOptionText(msg, false))
	return err
}

func (h *Handler) GetIncident(incident_id string)  *data.Incident {
        // This function will retrieve an incident from the database
        db, err := sql.Open("mysql", "incidentbot:AVNS_67iDl956qEd8uYA_wNT@tcp(batchco-db-do-user-1953615-0.b.db.ondigitalocean.com:25060)/incidentbot")
        defer db.Close()

        if err != nil {
                log.Error(err)
                return nil
        }

        sql := "SELECT summary, incident_opened_by, incident_commander, incident_manager, severity, state, chat_room, incident_created, incident_start, incident_end FROM incidents WHERE id=" + incident_id
        incident := new(data.Incident)
        incident.Id, _  = strconv.Atoi(incident_id)
        row := db.QueryRow(sql)

        err2 := row.Scan(&incident.Summary, &incident.Openedby, &incident.Commander, &incident.Manager, &incident.Severity,  &incident.State, &incident.Chat_room, &incident.Created, &incident.Start, &incident.End)
	if err2 != nil {
		log.Error(err2)
	}

	incident.Openedby.String = h.GetUserName(incident.Openedby.String)
	incident.Commander.String = h.GetUserName(incident.Commander.String)
	incident.Manager.String = h.GetUserName(incident.Manager.String)

	if incident.Severity.String == "" {
		incident.Severity.String = "Not Set"
	}

        return incident
}

func (h *Handler) GetUserName(user_id string)  string {
	if user_id == "" {
		return "Not Set"
	} else {
		user_data, _ := h.getUser(user_id)
		// TODO Add error checking
		return user_data.Name
	}
}

func (h *Handler) ConnectDB(dbConnectionInfo dbConnectionInfo) *sql.DB {

	connection_string := dbConnectionInfo.Username + ":" + dbConnectionInfo.Password + "@tcp(" + dbConnectionInfo.Host + ":" + dbConnectionInfo.Port + ")/" + dbConnectionInfo.Database
	db, err := sql.Open("mysql", connection_string)
	if err != nil {
		log.Error(err)
	}

	return db
}
