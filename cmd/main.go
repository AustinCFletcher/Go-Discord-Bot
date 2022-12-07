package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"yeet/go-microservices/go-microservices/cmd/parse"

	"github.com/bwmarrin/discordgo"
	redis "github.com/go-redis/redis/v8"
)

// AUSTIN TODO: reply to certain commands
// AUSTIN TODO: help command
// AUSTIN TODO: dockerize
// AUSTIN TODO: cloud
// AUSTIN TODO: refactor parse
// AUSTIN TODO: command router
// AUSTIN TODO: read in token from env file

// AUSTIN TODO: go routines and concurrency investigation

// AUSTIN TODO: investigate ryan emojis (remove )

func main() {

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	var list map[string][]string
	cacheReadNeeded := true
	ctx := context.Background()
	_, err := rdb.Get(ctx, "emojis").Result()
	if err == redis.Nil {
		fmt.Println("Seeding emojis")
		err = parse.SeedEmojis(rdb)
		if err != nil {
			fmt.Println("error seeding emojis")
		}
	} else {
		fmt.Println("already seeded")
	}

	// AUSTIN TODO: read in token from env file
	discord, err := discordgo.New("Bot " + "ODI5NDU4MTA3MzkzNjM4NDEw.YG4bDw.SG9slWDUG2wu1cTu9xD1qOYOeJs")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// user post history? mentions of a user

	// AUSTIN TODO: move to actual func
	discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {

		// Ignore all messages created by the bot itself
		// This isn't required in this specific example but it's a good practice.
		if m.Author.ID == s.State.User.ID {
			return
		}

		fmt.Println(m.Author.Username)
		fmt.Println(m.Content)
		fmt.Printf("%+v", m.Message)

		if len(m.Mentions) > 0 {
			mentions := []string{}
			for _, mention := range m.Mentions {
				mentions = append(mentions, mention.String())
			}
			if parse.IsBotMention(mentions) {
				fmt.Println("bot mentioned, attempting to parse command")

				comm, parseErr := parse.ParseCommand(m.Content, m.ChannelID, m.ID)
				if parseErr != nil {
					fmt.Println("PARSE ERROR")
					fmt.Println(parseErr.Error())
					_, err := s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("There was an error parsing your command: \n `%s`", parseErr.Error()), m.Reference())
					if err != nil {
						fmt.Println("Error writing reply" + err.Error())
					}
					return
				}

				err = parse.HandleCommand(rdb, comm, s, m)
				if err != nil {
					fmt.Println("COMMAND HANLDE ERROR")
					fmt.Println(err.Error())
					_, err := s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("There was an error handling your command: \n `%s`", err.Error()), m.Reference())
					if err != nil {
						fmt.Println("Error writing reply" + err.Error())
					}
					return
				}

				// AUSTIN TODO: think through how to handle cache invalidation...
				cacheReadNeeded = true
				return
			}
		}

		var err error
		// populate emoji-to-phrase-list cache from RDS if not populated or update has happened
		if cacheReadNeeded {
			list, err = parse.GetEmojiLists(rdb)
			if err != nil {
				fmt.Println("LIST ERROR")
			}
			cacheReadNeeded = false
		}

		// fmt.Println(list)

		// AUSTIN TODO: goroutines

		loweredContent := strings.ToLower(m.Content)

		fmt.Println("===============")

		for key := range list {
			go func(emojiStr string) {
				for _, phrase := range list[emojiStr] {
					// AUSTIN TODO: sliding window alg for performant contains checking of multiple words
					if strings.Contains(loweredContent, strings.ToLower(phrase)) {
						emoji := emojiStr
						if strings.HasPrefix(emoji, "<:") {
							emoji = strings.TrimPrefix(emoji, "<:")
							emoji = strings.TrimSuffix(emoji, ">")
						} else if strings.HasPrefix(emoji, "<a:") {
							emoji = strings.TrimPrefix(emoji, "<a:")
							emoji = strings.TrimSuffix(emoji, ">")
						}
						reactErr := s.MessageReactionAdd(m.ChannelID, m.ID, emoji)
						if reactErr != nil {
							fmt.Println(reactErr.Error())
						}
						break
					}
				}
			}(key)
		}

	})

	// fmt.Println(1)
	// port := 2222
	// log.Println("Starting HTTP service at " + port)
	// err = http.ListenAndServe(":"+port, nil) // Goroutine will block here

	// if err != nil {
	// 	log.Println("An error occured starting HTTP listener at port " + port)
	// 	log.Println("Error: " + err.Error())
	// }

	// Just like the ping pong example, we only care about receiving message
	// events in this example.
	// discord.Identify.Intents
	discord.Identify.Intents = discordgo.IntentsAll
	discord.StateEnabled = true

	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Printf("%+v", discord.State)
	fmt.Println(discord.State)

	// FO guild id
	// guildMembers, gmErr := discord.GuildMembers("600391942274023440", "", 100)
	// fmt.Println(gmErr)
	// if gmErr == nil {
	// 	for _, member := range guildMembers {
	// 		fmt.Println(2)
	// 		fmt.Println(member.User.Username)
	// 		fmt.Println(member.Nick)
	// 		fmt.Printf("%+v", member)
	// 	}
	// }

	// if discord.State != nil {
	// 	fmt.Println("printing")
	// 	for _, guild := range discord.State.Guilds {
	// 		fmt.Println(1)
	// 		fmt.Println(guild.Name)
	// 		fmt.Println(guild)
	// 		// guild.
	// 		for _, member := range guild.Members {
	// 			fmt.Println(2)
	// 			fmt.Println(member.User.Username)
	// 			fmt.Println(member.Nick)
	// 		}
	// 	}
	// }

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	discord.Close()
}
