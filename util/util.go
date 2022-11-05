package util

import (
	"math"
	"os"
	"strconv"
	"strings"
	"database/sql"

	log "github.com/sirupsen/logrus"
	_ "github.com/go-sql-driver/mysql"
)

func Ordinalize(num int) string {

	var ordinalDictionary = map[int]string{
		0: "th",
		1: "st",
		2: "nd",
		3: "rd",
		4: "th",
		5: "th",
		6: "th",
		7: "th",
		8: "th",
		9: "th",
	}

	// math.Abs() is to convert negative number to positive
	floatNum := math.Abs(float64(num))
	positiveNum := int(floatNum)

	if ((positiveNum % 100) >= 11) && ((positiveNum % 100) <= 13) {
		return strconv.Itoa(num) + "th"
	}

	return strconv.Itoa(num) + ordinalDictionary[positiveNum]

}

func LookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func LookupEnvOrInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		v, _ := strconv.Atoi(val)
		return v
	}
	return defaultVal
}

func LookupEnvOrBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if val == "true" {
			return true
		} else {
			return false
		}
	}
	return defaultVal
}

func ParseAdmins(admins string) []string {
	// Convert admins list into slice
	var admins_ary []string
	if len(admins) > 0 {
		if strings.Contains(admins, ",") {
			admins_ary = strings.Split(admins, ",")
		} else {
			admins_ary = append(admins_ary, admins)
		}
	}

	return admins_ary
}

func InSlice(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

func LogEvent(incident_id string, user string,  action string, description string) bool {
	// This function will log an event to the incident table
	db, err := sql.Open("mysql", "incidentbot:AVNS_67iDl956qEd8uYA_wNT@tcp(batchco-db-do-user-1953615-0.b.db.ondigitalocean.com:25060)/incidentbot")
	defer db.Close()

	if err != nil {
		log.Error(err)
		return false
	}

	sql := "INSERT INTO incident_log (incident_id, user,  action, description) VALUES (" + incident_id + ", '" + user + "', '" + action + "', '" + description + "')"
	_, err2 := db.Exec(sql)

	if err2 != nil {
		log.Error(err2)
		return false
	}

	return true
}

