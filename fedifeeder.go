package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ginzerolog "github.com/dn365/gin-zerolog"
	"github.com/gin-gonic/gin"
	"github.com/jasonlvhit/gocron"
	"github.com/mattn/go-mastodon"
	"github.com/rs/zerolog"
)

// 0. rename to fedi-feeder
// 1. load current follows into struct
// 2. start cron job
// 3. start gin server
// 4. continuously check public timeline, both local and non-local
// 6. if new post, check if user is already in the struct
// 7. if not, add user to struct and submit a follow request
// 8. serve a page with:
// 8.1. a count of the most recent new follows
// 8.2. a timestamp of the last run time
// 9. update cron time to 60 seconds

// ?: use streaming
// ?: maybe offer fedifeeder as a service
//   - create a limit function and rotating queue of 1000 (or something)
//   - back it to sqlite and skip the component that checks for existing follows
//   - serve as an API endpoint
//   - write a client to allow others to consume the API endpoint
var userMap = make(map[string]string)
var lastRunTime string
var logger = zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.InfoLevel)
var Port int

func executeCronJob(cRemote *mastodon.Client, cLocal *mastodon.Client) {
	gocron.Every(60).Second().Do(recordNewPosters, cRemote, cLocal)
	<-gocron.Start()
}

func main() {
	if os.Getenv("DEBUG") != "" {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.DebugLevel)
	}
	var Port string
	if os.Getenv("PORT") != "" {
		Port = os.Getenv("PORT")
	} else {
		Port = "8080"
	}

	// set up the source connection
	if os.Getenv("MS_SOURCE_SERVER") == "" {
		log.Fatal("MS_SOURCE_SERVER is not set")
	}
	if os.Getenv("MS_SOURCE_CLIENT_ID") == "" {
		log.Fatal("MS_SOURCE_CLIENT_ID is not set")
	}
	if os.Getenv("MS_SOURCE_CLIENT_SECRET") == "" {
		log.Fatal("MS_SOURCE_CLIENT_SECRET is not set")
	}
	if os.Getenv("MS_SOURCE_ACCESS_TOKEN") == "" {
		log.Fatal("MS_SOURCE_ACCESS_TOKEN is not set")
	}
	cRemote := mastodon.NewClient(&mastodon.Config{
		Server:       os.Getenv("MS_SOURCE_SERVER"),
		ClientID:     os.Getenv("MS_SOURCE_CLIENT_ID"),
		ClientSecret: os.Getenv("MS_SOURCE_CLIENT_SECRET"),
		AccessToken:  os.Getenv("MS_SOURCE_ACCESS_TOKEN"),
	})

	// set up the target connection
	if os.Getenv("MS_TARGET_PROTOCOL") == "" {
		log.Fatal("MS_TARGET_PROTOCOL is not set")
	}
	if os.Getenv("MS_TARGET_HOST") == "" {
		log.Fatal("MS_TARGET_HOST is not set")
	}
	if os.Getenv("MS_TARGET_CLIENT_ID") == "" {
		log.Fatal("MS_TARGET_CLIENT_ID is not set")
	}
	if os.Getenv("MS_TARGET_CLIENT_SECRET") == "" {
		log.Fatal("MS_TARGET_CLIENT_SECRET is not set")
	}
	if os.Getenv("MS_TARGET_ACCESS_TOKEN") == "" {
		log.Fatal("MS_TARGET_ACCESS_TOKEN is not set")
	}
	targetServer := os.Getenv("MS_TARGET_PROTOCOL") + "://" + os.Getenv("MS_TARGET_HOST")
	cLocal := mastodon.NewClient(&mastodon.Config{
		Server:       targetServer,
		ClientID:     os.Getenv("MS_TARGET_CLIENT_ID"),
		ClientSecret: os.Getenv("MS_TARGET_CLIENT_SECRET"),
		AccessToken:  os.Getenv("MS_TARGET_ACCESS_TOKEN"),
	})

	logger.Info().Msg("Starting...")
	//populate the userMap with the current follows
	getMyFollowingIds(cLocal)
	// start the background thread
	go executeCronJob(cRemote, cLocal)
	// create endpoints for health checks and debugging
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.SetTrustedProxies([]string{"::1"})
	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	r.Use(gin.Recovery())
	r.Use(ginzerolog.Logger("gin"))
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"last_count": len(userMap),
			"last_run":   lastRunTime,
		})
	})
	if os.Getenv("DEBUG") != "" {
		r.GET("/debug", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"found_users":  mapToSlice(userMap, "keys"),
				"followed_ids": mapToSlice(userMap, "values"),
			})
		})
	}
	logger.Info().Msg("Listening on port " + Port)
	r.Run(":" + Port)
}

func getMyFollowingIds(c *mastodon.Client) {
	myInfo, err := c.GetAccountCurrentUser(context.Background())
	if err != nil {
		// leaving this fatal because it could lead to duplicate follow requests
		log.Fatal(err)
	}
	getFollowingIDs(c, myInfo.ID)
}

func getFollowingIDs(c *mastodon.Client, id mastodon.ID) []string {
	var ids []string
	myFollows, err := c.GetAccountFollowing(context.Background(), id, nil)
	if err != nil {
		// leaving this fatal because it could lead to duplicate follow requests
		log.Fatal(err)
	}
	if len(myFollows) == 0 {
		logger.Error().Msg("No follows found")
		return ids
	}
	if myFollows == nil {
		logger.Error().Msg("No follows found")
		return ids
	}
	for _, follow := range myFollows {
		ids = append(ids, string(follow.ID))
		p := strings.Split(string(follow.Acct), "@")
		var userUrl string
		if len(p) != 2 {
			userUrl = fmt.Sprintf("https://%s/@%s", os.Getenv("MS_TARGET_HOST"), p[0])
		} else {
			userUrl = fmt.Sprintf("https://%s/@%s", p[1], p[0])
		}
		userMap[userUrl] = string(follow.ID)
		loggerMsg := fmt.Sprintf("following user: %s, id: %s", follow.Acct, follow.ID)
		logger.Debug().Msg(loggerMsg)
	}
	return ids
}

func recordNewPosters(cRemote *mastodon.Client, cLocal *mastodon.Client) {
	np := getNewPosters(cRemote, cLocal)
	// rs := fmt.Sprintf("New follows: %d", np)
	logger.Debug().Int("follows_found", np).Send()
}

func getNewPosters(cRemote *mastodon.Client, cLocal *mastodon.Client) int {
	// Get the non-local public timeline
	timeline, err := cRemote.GetTimelinePublic(context.Background(), false, nil)
	if err != nil {
		logger.Err(err).Msg("Error getting remote non-local public timeline")
	}
	processTimeline(timeline, cLocal)
	// Get the local public timeline
	timeline, err = cRemote.GetTimelinePublic(context.Background(), true, nil)
	if err != nil {
		logger.Err(err).Msg("Error getting remote local public timeline")
	}
	processTimeline(timeline, cLocal)
	lastRunTime = time.Now().Format("2006-01-02 15:04:05")
	return len(userMap)
}

func processTimeline(timeLine []*mastodon.Status, cLocal *mastodon.Client) {
	for i := len(timeLine) - 1; i >= 0; i-- {
		p := strings.Split(timeLine[i].URL, "/")
		// remove the last element
		p = p[:len(p)-1]
		URI := strings.Join(p[1:], "/")
		postUrl := p[0] + "/" + URI
		_, isPresent := userMap[postUrl]
		if isPresent == false {
			// follow the user
			userID, err := userToID(cLocal, postUrl)
			if err != nil {
				logger.Err(err).Msg("Error getting user id for " + postUrl)
			} else {
				cLocal.AccountFollow(context.Background(), userID)
				userMap[postUrl] = string(userID)
				logger.Debug().Msg(fmt.Sprintf("FOLLOWING -- %s", postUrl))
			}
		} else {
			logger.Debug().Msg(fmt.Sprintf("SKIP -- Already following user: %s", postUrl))
		}
	}
}

func usersToIDs(c *mastodon.Client, users []string) []string {
	var ids []string
	for _, user := range users {
		id, err := userToID(c, user)
		if err != nil {
			logger.Err(err).Msg("Error getting user id for " + user)
		} else {
			ids = append(ids, string(id))
		}
	}
	return ids
}

func userToID(c *mastodon.Client, user string) (mastodon.ID, error) {
	mID, err := c.Search(context.Background(), user, true)
	if err != nil {
		logger.Err(err).Msg("Error getting user id for " + user)
	}
	logger.Debug().Msg(fmt.Sprintf("Processing user: %s", user))
	if mID == nil {
		errMsg := "Got a nil results for " + user
		logger.Warn().Msg(errMsg)
		// return a unique value so we know we've seen this user before but can filter
		// so we can skip the lookup again but not try to follow
		return "NaN", errors.New(errMsg)
	}
	logger.Debug().Msg(fmt.Sprintf("Accounts found: %d", len(mID.Accounts)))
	if len(mID.Accounts) == 0 {
		errMsg := "No results for " + user
		logger.Warn().Msg(errMsg)
		// return a unique value so we know we've seen this user before but can filter
		// so we can skip the lookup again but not try to follow
		return "NaN", errors.New(errMsg)
	} else {
		accountId := mID.Accounts[0].ID
		logMsg := fmt.Sprintf("ADDING user: %s, id: %s\n", user, accountId)
		logger.Info().Msg(logMsg)
		return accountId, nil
	}
}

func mapToSlice(m map[string]string, t string) []string {
	var s []string
	if t == "keys" {
		for key := range m {
			cleanedKey := strings.TrimSuffix(key, "\n")
			s = append(s, cleanedKey)
		}
	}
	if t == "values" {
		for _, value := range m {
			cleanedValue := strings.TrimSuffix(value, "\n")
			s = append(s, cleanedValue)
		}
	}
	return s
}
