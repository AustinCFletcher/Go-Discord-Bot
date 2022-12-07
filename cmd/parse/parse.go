package parse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"github.com/forPelevin/gomoji"
	redis "github.com/go-redis/redis/v8"
)

var ctx = context.Background()

// todo what docker image to run this with?

const freeRealEstaustin = "<:free_real_estaustin:806997475835510874>"

var minimumViableBotEmojis = []string{freeRealEstaustin}
var austinPhrases = []string{"austin"}

func SeedEmojis(rdb *redis.Client) error {
	a, _ := json.Marshal(minimumViableBotEmojis)
	err := rdb.Set(ctx, "emojis", a, 0).Err()
	if err != nil {
		return err
	}

	a, _ = json.Marshal(austinPhrases)
	err = rdb.Set(ctx, freeRealEstaustin, a, 0).Err()
	if err != nil {
		return err
	}

	return nil
}

func GetEmojiLists(rdb *redis.Client) (map[string][]string, error) {

	list := make(map[string][]string)

	var emojisResult []string
	bytes, err := rdb.Get(ctx, "emojis").Bytes()
	if err != nil {
		return list, WrapError(err, "Error getting emojis")
	}
	if err := json.Unmarshal(bytes, &emojisResult); err != nil {
		return list, WrapError(err, "Error unmarshalling emojis")
	}

	// AUSTIN TODO: waitgroup and goroutine here as emojis list grows

	for _, emoji := range emojisResult {
		var phrases []string
		bytes, err := rdb.Get(ctx, emoji).Bytes()
		if err != nil {
			return list, WrapError(err, "Error getting phrases for emoji")
		}
		if err := json.Unmarshal(bytes, &phrases); err != nil {
			return list, WrapError(err, "Error unmarshalling phrases")
		}

		list[emoji] = phrases
	}

	fmt.Println(list)

	return list, nil
}

const botName = "<@829458107393638410>"

// <@!829458107393638410>
const botNameMentionsFormat = "FOCordAustinBot#2983"

type EmojiCommand struct {
	IsBasicEmoji bool
	Emoji        string
	Phrase       string
}

type Command struct {
	Name           string
	Args           []string
	MessageID      string //*discordgo.MessageCreate // dont love the inclusion of a dependency type here but easy for now
	ChannelID      string
	MessageContent string
}

func AddEmojiCommandHandler(command Command) (EmojiCommand, error) {
	emojiCommand := EmojiCommand{}

	// args[0] is @'ing the bot
	// args[1] is the emoji command

	if len(command.Args) != 2 {
		return emojiCommand, errors.New(fmt.Sprintf("Wrong amount of addEmoji args: %d", len(command.Args)))
	}

	emojiString := command.Args[0]

	if strings.ContainsAny(emojiString, `“”`) {
		return emojiCommand, errors.New(`Bad quotes “ or ” used, try " instead`)
	}

	if strings.HasPrefix(emojiString, "<:") && strings.HasSuffix(emojiString, ">") {
		// split emoji name from id
		trimmed := strings.TrimPrefix(emojiString, "<:")
		trimmed = strings.TrimSuffix(trimmed, ">")
		split := strings.Split(trimmed, ":")
		if len(split) != 2 {
			return emojiCommand, errors.New("Bad custom emoji format")
		}
	} else if strings.HasPrefix(emojiString, "<a:") && strings.HasSuffix(emojiString, ">") {
		//<a:abortion:895709963299729498>
		// split emoji name from id
		trimmed := strings.TrimPrefix(emojiString, "<a:")
		trimmed = strings.TrimSuffix(trimmed, ">")
		split := strings.Split(trimmed, ":")
		if len(split) != 2 {
			return emojiCommand, errors.New("Bad custom emoji animated format")
		}
	} else if _, mojiErr := gomoji.GetInfo(emojiString); mojiErr == nil {

		// if gomoji.ContainsEmoji(emojiString) && utf8.RuneCountInString(emojiString) == 1 {
		emojiCommand.IsBasicEmoji = true
	} else {
		return emojiCommand, errors.New("Emoji is not discord format or normal emoji")
	}

	if utf8.RuneCountInString(command.Args[1]) <= 2 {
		return emojiCommand, errors.New("Not enough characters in requested phrase")
	}

	emojiCommand.Emoji = emojiString
	emojiCommand.Phrase = command.Args[1]

	return emojiCommand, nil
}

func ListPhrasesHandler(rdb *redis.Client, command Command, s *discordgo.Session, m *discordgo.MessageCreate) error {

	// AUSTIN TODO: wrap this rds stuff in meaningful method

	if len(command.Args) != 1 {
		return errors.New("Bad number of args to command: emojiList")
	}

	var phrasesResult []string
	bytes, err := rdb.Get(ctx, command.Args[0]).Bytes()
	if err != nil {
		if err == redis.Nil {
			return errors.New("No phrases for that emoji")
		}
		return WrapError(err, "Error getting phrases for emoji")
	}

	if err := json.Unmarshal(bytes, &phrasesResult); err != nil {
		return WrapError(err, "Error unmarshalling phrases")
	}

	emojisList := ""

	for _, phrase := range phrasesResult {
		emojisList = emojisList + phrase + " \n"
	}

	_, err = s.ChannelMessageSendReply(m.ChannelID, emojisList, m.Reference())
	if err != nil {
		return err
	}

	return nil
}

// AUSTIN TODO: have a name to handler mapping
// handlers should register themselves on startup, not be a hardcoded big switch
// have them in map

func HandleCommand(rdb *redis.Client, command Command, s *discordgo.Session, m *discordgo.MessageCreate) error {
	switch command.Name {
	case "addEmoji":
		emojiCommand, err := AddEmojiCommandHandler(command)
		if err != nil {
			return err
		}

		err = WriteEmojiCommand(rdb, emojiCommand)
		if err != nil {
			fmt.Println("WRITE ERROR")
			return err
		}
		fmt.Println("WRITTEN")
	case "listPhrases":
		err := ListPhrasesHandler(rdb, command, s, m)
		if err != nil {
			return WrapError(err, "Error handling listPhrases command")
		}
	case "help":
		err := HelpHandler(s, m)
		if err != nil {
			return WrapError(err, "Error handling help command")
		}
	default:
		return errors.New(fmt.Sprintf("Bad command: %s", command.Name))
	}

	return nil
}

// todo: could have handler type, with command name, handler func, syntax, etc

func HelpHandler(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// AUSTIN TODO: fancier string-builder this for slice of supported commands
	helpText := `Welcome to FOCordBot 🥺

To run commands, syntax is: 
` +
		"`@bot command args... `" +
		`
Currently supported commands: 
` +
		"`addEmoji <emoji> <phrase to react to>` \n" +
		"`listPhrases <emoji>` \n" +
		"`help` \n"

	_, err := s.ChannelMessageSendReply(m.ChannelID, helpText, m.Reference())
	if err != nil {
		return err
	}

	return nil
}

// addReact
// rmReact

// call parse command
// switch on command name

// AUSTIN TODO: enhance to take in commands
// command should be in format: @bot commandName args...
func ParseCommand(commandStr string, channelID string, messageID string) (Command, error) {
	command := Command{}

	args := commandLineToArgv(commandStr)
	fmt.Println(args)

	// args[0] is @'ing the bot
	// args[1] is the command
	// args[2:] is the actual args

	if len(args) < 2 {
		return command, errors.New(fmt.Sprintf("Wrong amount of args: %d", len(args)))
	}

	command.Name = args[1]
	if len(args) > 2 {
		command.Args = args[2:]
	}
	command.MessageID = messageID
	command.ChannelID = channelID

	return command, nil
}

func WrapError(err error, wrapText string) error {
	return errors.New(fmt.Sprintf("%s: %s", wrapText, err.Error()))
}

// AUSTIN TODO: wrap redis in interface?
func WriteEmojiCommand(rdb *redis.Client, command EmojiCommand) error {

	// determine if emoji is already in emojis
	var emojisResult []string
	bytes, err := rdb.Get(ctx, "emojis").Bytes()
	if err != nil {
		return WrapError(err, "Error getting emojis")
	}
	if err := json.Unmarshal(bytes, &emojisResult); err != nil {
		return WrapError(err, "Error unmarshalling emojis")
	}

	found := false

	for _, emoji := range emojisResult {
		if emoji == command.Emoji {
			found = true
			break
		}
	}

	if !found {
		// write to emojis
		emojisResult = append(emojisResult, command.Emoji)
		bytes, _ := json.Marshal(emojisResult)
		err := rdb.Set(ctx, "emojis", bytes, 0).Err()
		if err != nil {
			return WrapError(err, "Error writing new emoji to emoji list")
		}

		// write to the phrases
		phrases := []string{command.Phrase}
		bytes, _ = json.Marshal(phrases)
		err = rdb.Set(ctx, command.Emoji, bytes, 0).Err()
		if err != nil {
			return WrapError(err, "Error writing new phase to emoji->phrases list")
		}
	} else {
		// emoji already existed so we need to read in the  phrases and add to it if not there
		// determine if emoji is already in emojis
		var phrasesResult []string
		bytes, err := rdb.Get(ctx, command.Emoji).Bytes()
		if err != nil {
			return WrapError(err, "Error getting phrases for emoji")
		}
		if err := json.Unmarshal(bytes, &phrasesResult); err != nil {
			return WrapError(err, "Error unmarshalling phrases")
		}

		for _, phrase := range phrasesResult {
			if phrase == command.Phrase {
				return nil // no op
			}
		}

		phrasesResult = append(phrasesResult, command.Phrase)
		bytes, _ = json.Marshal(phrasesResult)
		err = rdb.Set(ctx, command.Emoji, bytes, 0).Err()
		if err != nil {
			return WrapError(err, "Error writing new emoji to emoji list")
		}
	}

	return nil
}

// stole from go itself
func commandLineToArgv(cmd string) []string {
	var args []string
	for len(cmd) > 0 {
		if cmd[0] == ' ' || cmd[0] == '\t' {
			cmd = cmd[1:]
			continue
		}
		var arg []byte
		arg, cmd = readNextArg(cmd)
		args = append(args, string(arg))
	}
	return args
}

func readNextArg(cmd string) (arg []byte, rest string) {
	var b []byte
	var inquote bool
	var nslash int
	for ; len(cmd) > 0; cmd = cmd[1:] {
		c := cmd[0]
		switch c {
		case ' ', '\t':
			if !inquote {
				return appendBSBytes(b, nslash), cmd[1:]
			}
		case '"':
			b = appendBSBytes(b, nslash/2)
			if nslash%2 == 0 {
				// use "Prior to 2008" rule from
				// http://daviddeley.com/autohotkey/parameters/parameters.htm
				// section 5.2 to deal with double double quotes
				if inquote && len(cmd) > 1 && cmd[1] == '"' {
					b = append(b, c)
					cmd = cmd[1:]
				}
				inquote = !inquote
			} else {
				b = append(b, c)
			}
			nslash = 0
			continue
		case '\\':
			nslash++
			continue
		}
		b = appendBSBytes(b, nslash)
		nslash = 0
		b = append(b, c)
	}
	return appendBSBytes(b, nslash), ""
}

// appendBSBytes appends n '\\' bytes to b and returns the resulting slice.
func appendBSBytes(b []byte, n int) []byte {
	for ; n > 0; n-- {
		b = append(b, '\\')
	}
	return b
}

func IsBotMention(mentions []string) bool {
	for _, a := range mentions {
		if a == botNameMentionsFormat {
			return true
		}
	}
	return false
}

func main() {

	return

	fmt.Println(2)

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err := rdb.Get(ctx, "emojis").Result()
	if err == redis.Nil {
		fmt.Println("Seeding emojis")
		err = SeedEmojis(rdb)
		if err != nil {
			fmt.Println("error seeding emojis")
			panic(err)
		}
	} else {
		fmt.Println("already seeded")
	}

	list, err := GetEmojiLists(rdb)
	if err != nil {
		panic(err)
	}
	fmt.Println(list)

	// AUSTIN TODO: consider validating emojis etc
	//

	// type EmojiBag struct {
	// 	Emojis []string
	// }

	// emojis := []string{"one", "two", "three"}
	// fmt.Println(emojis)

	// //bag := EmojiBag{Emojis: emojis}

	// a, _ := json.Marshal(emojis)
	// fmt.Println(string(a)) // 20192

	// err = rdb.Set(ctx, "emojis", a, 0).Err()
	// if err != nil {
	// 	panic(err)
	// }

	// err = rdb.Set(ctx, "emojis2", bag, 0).Err()
	// if err != nil {
	// 	panic(err)
	// }
	var emojisResult []string
	bytes, err := rdb.Get(ctx, "emojis").Bytes()
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(bytes, &emojisResult); err != nil {
		panic(err)
	}

	fmt.Println(emojisResult)

	// err = rdb.Set(ctx, "key", "value", 0).Err()
	// if err != nil {
	// 	panic(err)
	// }

	// val, err := rdb.Get(ctx, "key").Result()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println("key", val)

	// val2, err := rdb.Get(ctx, "key2").Result()
	// if err == redis.Nil {
	// 	fmt.Println("key2 does not exist")
	// } else if err != nil {
	// 	panic(err)
	// } else {
	// 	fmt.Println("key2", val2)
	// }
	// Output: key value
	// key2 does not exist
}
