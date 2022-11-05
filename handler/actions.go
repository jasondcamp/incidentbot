package handler

import (
	"regexp"
	"database/sql"

	log "github.com/sirupsen/logrus"
	_ "github.com/go-sql-driver/mysql"
)

const TICK = "`"

var (
	actions = map[string]regexp.Regexp{
		"new":		  *regexp.MustCompile(`new`),
		"start":	  *regexp.MustCompile(`start`),
		"archive":	  *regexp.MustCompile(`archive\s(.+)`),
		"single_archive": *regexp.MustCompile(`archive`),

		"update-summary":        *regexp.MustCompile(`update-summary\s(\d+)\s(.+)`),
		"update-severity":        *regexp.MustCompile(`update-severity\s(\d+)\s(.+)`),
		"update-commander":        *regexp.MustCompile(`update-commander\s(\d+)\s\<\@(.+)\>`),
		"update-manager":        *regexp.MustCompile(`update-manager\s(\d+)\s\<\@(.+)\>`),
		"update-state":        *regexp.MustCompile(`update-state\s(\d+)\s(.+)`),

		"status": 	  *regexp.MustCompile(`status\s(\d+)`),
//		"status_inroom":   *regexp.MustCompile(`status`),

//		"hello":          *regexp.MustCompile(`hello.+`),
//		"reserve":        *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sreserve\s(.+)`),
//		"release":        *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\srelease\s(.+)`),
//		"reserve_dm":        *regexp.MustCompile(`(?m)^reserve\s(.+)`),
//		"release_dm":        *regexp.MustCompile(`(?m)^release\s(.+)`),
		"help_dm":           *regexp.MustCompile(`(?m)^help$`),
	}
)

var (
	msgAlreadyInAllQueues           = "Bruh, you are already in all specified queues"
	msgIDontKnow                    = "I don't know what happened, but it wasn't good"
	msgMustSpecifyResource          = "You must specify a resource"
	msgMustSpecifyUser              = "You must specify a user to kick"
	msgMustSpecifyValidResource     = "You must specify a valid resource"
	msgMustUseReleaseForY           = "You cannot remove yourself from the queue for `%s` because you currently have it. Please use `release` instead."
	msgMustUseRemoveForY            = "You cannot release `%s` because you do not currently have it. Please use `remove me from` instead."
	msgNoReservations               = "Like Anthony Bourdain :rip:, there are _no reservations_. Lose yourself in the freedom of a world waiting on your next move."
	msgPeriodItIsNowFree            = ". It is now free."
	msgPeriodXHasItCurrently        = ". %s has it currently."
	msgPeriodXStillHasIt            = ". %s still has it."
	msgQueuesPruned                 = "I have removed all unreserved resources. Hope that's what you wanted. If not, it's too late now. Fool."
	msgRemoveResourceNotFound       = "Resource cannot be removed, it was not found."
	msgRemoveResourceReserved       = "Resource cannot be removed, it currently has active reservations."
	msgRemoveResourceSuccess        = "Resource removed."
	msgReservedButNotInQueue        = "%s reserved `%s`, but is currently not in the queue"
	msgResourceDoesNotExistY        = "Resource `%s` does not exist"
	msgResourceImproperlyFormatted  = "LOL u serious? Resources must be formatted as `<env>|<name>`. Example: `your_family|mom`"
	msgUknownUser                   = "I'm sorry, I don't know who that is. Do _you_ know that is?"
	msgXClearedY                    = "%s cleared `%s`"
	msgXCurrentlyHas                = "%s currently has `%s`"
	msgXHasBeenKickedFromNResources = "%s has been kicked from %d resource(s)"
	msgXHasBeenRemovedFromY         = "%s has been kicked from `%s`. It's all yours. Get weird."
	msgXHasBeenRemovedFromYZ        = "%s has been removed from the queue for `%s`%s"
	msgXHasReleasedYItIsYours       = "%s has released `%s`. It's all yours. Get weird."
	msgXHasReleasedYZ               = "%s has released `%s`%s"
	msgXHasRemovedThemselvesFromYZ  = "%s has removed themselves from the queue for `%s`%s"
	msgXItIsYours                   = "%s it's all yours. Get weird."
	msgXKickedYouFromY              = "%s kicked you from `%s`"
	msgXNukedQueue                  = "%s nuked the whole thing. Yikes."
	msgYHasBeenCleared              = "`%s` has been cleared"
	msgYouAreNInLineForY            = "You are %s in line for `%s`%s"
	msgYouAreNotInLineForY          = "You are not in line for `%s`"
	msgYouCurrentlyHave             = "You currently have `%s`"
	msgYouHaveNoReservations        = "You have no reservations"
	msgYouHaveReleasedY             = "You have released `%s`"
	msgYouHaveRemovedXFromY         = "You have removed %s from `%s`"
	msgYouHaveRemovedYourselfFromY  = "You have removed yourself from `%s`"
)

func (h *Handler) getAction(text string) string {
	for a, r := range actions {
		if r.MatchString(text) {
			return a
		}
	}
	return ""
}

func (h *Handler) prune(ea *EventAction) error {
	resources := h.data.GetResources()
	for _, res := range resources {
		q, err := h.data.GetQueueForResource(res.Name, res.Env)
		if err != nil {
			// this shouldn't happen, but there's nothing to alert the user to
			log.Errorf("%+v", err)
			continue
		}

		if q.HasReservations() {
			continue
		}

		err = h.data.RemoveResource(res.Name, res.Env)
		if err != nil {
			log.Errorf("%+v", err)
			continue
		}
	}

	h.reply(ea, msgQueuesPruned, false)

	return nil
}


func (h *Handler) updateIncidentField(incident_id string, field string, val string) bool {
	db, err := sql.Open("mysql", "incidentbot:AVNS_67iDl956qEd8uYA_wNT@tcp(batchco-db-do-user-1953615-0.b.db.ondigitalocean.com:25060)/incidentbot")
	defer db.Close()

	if err != nil {
		log.Error(err)
		return false
	}

	sql := "UPDATE incidents SET " + field + "='" + val + "' WHERE id=" + incident_id
	_, err4 := db.Exec(sql)

	if err4 != nil {
		log.Error(err4)
		return false
	}

return true

}

func (h *Handler) help(ea *EventAction) error {

	var helpText = "Hello! I can be used via any channel that I have been added to or via DM.\n\n"
	helpText += "*Commands*\n\n"
	helpText += "When invoking within a channel, you must @-mention me by adding " + TICK + "@incidentbot" + TICK + " to the _beginning_ of your command.\n\n"

	helpText += TICK + "new|start" + TICK + " This will open a new incident. It will assign an incident number, create an incident slack room, open a ticket, and allow the incident opener to set details about the incident.\n\n"
	helpText += TICK + "archive <incident_id>" + TICK + " This will archive the incident chat room. It will not update the incident status.\n\n"
	helpText += TICK + "update-summary <incident_id> <summary>" + TICK + " This will update the summary of an incident\n\n"
        helpText += TICK + "update-severity <incident_id> <SEV0|SEV1|SEV2|SEV3|SEV4>" + TICK + " This will update the severity of an incident\n\n"
        helpText += TICK + "update-commander <incident_id> <@commander>" + TICK + " This will update the commander of an incident\n\n"
        helpText += TICK + "update-manager <incident_id> <@manager>" + TICK + " This will update the manager of an incident\n\n"
        helpText += TICK + "update-state <incident_id> <new|inprogress|stalled|resolved>" + TICK + " This will update the state of an incident\n\n"
	helpText += TICK + "audit-log <incident_id>" + TICK + " Display audit log from the detailed log table\n\n"
	helpText += TICK + "invite <incident_id> <username|usergroup>" + TICK + "Invite user or group of users to incident\n\n"
	h.reply(ea, helpText, false)
	return nil
}
