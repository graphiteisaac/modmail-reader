package main

import (
	"strings"
	"testing"
	"time"
    "slices"
)

const INFO = `# Modmail thread #1 with graphiteisaac (204084691425427466) started at 2024-01-01 02:26:01. All times are in UTC+0.

[2024-01-01 02:26:01] [BOT] ACCOUNT AGE **8 years, 4 weeks**, ID **204084691425427466** (<@!204084691425427466>)
**[Overwatch 2]** NICKNAME **isaac**, JOINED **2 years, 10 months** ago, ROLES **Regular, Moderator Perms, Moderator, Veteran, 👤 He/Him**

This user has **24** previous modmail threads. Use ` + "`!logs`" + ` to see them.`

const CONTENT = `
[2024-08-15 03:26:01] [BOT] Thread was opened by graphiteisaac
[2024-01-01 02:26:10] [CHAT] [graphiteisaac] testing how this parses if i open my own thread
[2024-01-01 02:26:20] [COMMAND] [graphiteisaac] !r ok lilvro 😭
[2024-01-01 02:26:20] [TO USER] [graphiteIsaac] (Moderator) graphiteIsaac: ok lilvro 😭
[2024-01-01 02:26:23] [COMMAND] [graphiteisaac] !loglink
[2024-01-01 02:27:49] [COMMAND] [graphiteisaac] !close
[2024-01-01 02:27:50] [BOT TO USER] Thank you for contacting us, the ticket is now closed. If you need more help, feel free to send us another message!
[2024-01-01 02:27:50] [BOT] Closing thread...`

const FULL_THREAD = INFO + "\n────────────────\n" + CONTENT

func TestParseInfo(t *testing.T) {
	info, err := tokeniseInfo(INFO)
    
    if info.Servers[0].Name != "Overwatch 2" {
        t.Errorf("server name incorrect ['Overwatch 2', '%s']", info.Servers[0].Name)
    }

    if info.Servers[0].Nickname != "isaac" {
        t.Errorf("user nickname incorrect ['isaac', '%s']", info.Servers[0].Nickname)
    }
    
    if info.Servers[0].Joined != "2 years, 10 months" {
        t.Errorf("user join time incorrect ['2 years, 10 months', '%s']", info.Servers[0].Joined)
    }

    if slices.Compare(info.Servers[0].Roles, []string{"Regular", "Moderator Perms", "Moderator", "Veteran", "👤 He/Him"}) < 0 {
        t.Errorf("user roles incorrect ['Regular, Moderator Perms, Moderator, Veteran, 👤 He/Him', '%s']", strings.Join(info.Servers[0].Roles, ", "))
    }

	if err != nil {
		t.Errorf("tokeniseInfo returned an error: %+v\n", err)
	}

	if info.UserID != "204084691425427466" {
		t.Errorf("user id incorrect ['204084691425427466', '%s']\n", info.UserID)
	}

	if info.Username != "graphiteisaac" {
		t.Errorf("username incorrect ['graphiteisaac', '%s']\n", info.Username)
	}

	if info.AccountAge != "8 years, 4 weeks" {
		t.Errorf("account age incorrect ['8 years, 4 weeks', '%s']\n", info.AccountAge)
	}

	if info.NumThreads != 24 {
        t.Logf("%#v\n", info.NumThreads)
		t.Errorf("number of threads incorrect ['24', '%d']\n", info.NumThreads)
	}

    expectedTime := time.Date(2024, time.January, 01, 02, 26, 01, 00, time.UTC).Format(time.Stamp)
	if info.Opened != expectedTime {
		t.Errorf("parsed time is incorrect ['%s', '%s']\n", expectedTime, info.Opened)
	}
}

func TestParseContent(t *testing.T) {
    // TODO: Write content test
}

