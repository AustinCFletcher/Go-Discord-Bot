package parse

import (
	"fmt"
	"testing"

	"encoding/json"

	"github.com/forPelevin/gomoji"
	redis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func TestIntMinBasic(t *testing.T) {
	testCasesToFail := []string{
		`<@829458107393638410> addEmoji 👆 "🍑"`,
		// `<@829458107393638410> addEmoji 👆 "🍑 based"`,
		`<@829458107393638410> addEmoji 👆 “Aust10 based”`,
		`<@829458107393638410> addEmoji 👆 'Aust10' based`,
		`<@829458107393638410> addEmoji 👆 yers yedfhdh fdhdsjhj`,
	}

	for _, test := range testCasesToFail {

		_, err := ParseCommand(test, "cid", "mid")
		assert.Nil(t, err)
	}
}

func Test_parse_happyPath_customEmoji(t *testing.T) {
	commandString := `<@829458107393638410> addEmoji <:free_real_estaustin:806997475835510874>  "bruh"`

	comm, err := ParseCommand(commandString, "cid", "mid")
	assert.Nil(t, err)

	emojiComm, err := AddEmojiCommandHandler(comm)
	assert.Nil(t, err)

	assert.Equal(t, "<:free_real_estaustin:806997475835510874>", emojiComm.Emoji)
	assert.Equal(t, "bruh", emojiComm.Phrase)
	assert.Equal(t, false, emojiComm.IsBasicEmoji)
}

func Test_parse_happyPath_basicEmoji(t *testing.T) {
	commandString := `<@829458107393638410> addEmoji 👍  "thumbs up"`

	comm, err := ParseCommand(commandString, "cid", "mid")
	assert.Nil(t, err)

	emojiComm, err := AddEmojiCommandHandler(comm)
	assert.Nil(t, err)

	assert.Equal(t, "👍", emojiComm.Emoji)
	assert.Equal(t, "thumbs up", emojiComm.Phrase)
	assert.Equal(t, true, emojiComm.IsBasicEmoji)
}

func Test_parse_happyPath_basicEmoji_writeToRedis(t *testing.T) {
	// commandString := `<@829458107393638410> 👍  "thumbs up"`
	commandString := `<@829458107393638410> addEmoji <:free_real_estaustin:806997475835510874>  "real estate"`

	comm, err := ParseCommand(commandString, "cid", "mid")
	assert.Nil(t, err)
	// assert.Equal(t, "👍", comm.Emoji)
	// assert.Equal(t, "thumbs up", comm.Phrase)
	// assert.Equal(t, true, comm.IsBasicEmoji)

	emojiComm, err := AddEmojiCommandHandler(comm)
	assert.Nil(t, err)

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	err = WriteEmojiCommand(rdb, emojiComm)

	var emojisResult []string
	bytes, err := rdb.Get(ctx, "emojis").Bytes()
	assert.Nil(t, err)

	err = json.Unmarshal(bytes, &emojisResult)
	assert.Nil(t, err)

	assert.True(t, contains(emojisResult, emojiComm.Emoji))

	var phrasesResult []string
	bytes, err = rdb.Get(ctx, emojiComm.Emoji).Bytes()
	assert.Nil(t, err)
	err = json.Unmarshal(bytes, &phrasesResult)
	assert.Nil(t, err)

	assert.True(t, contains(phrasesResult, emojiComm.Phrase))
}

func Test_rds_basically(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	list, err := GetEmojiLists(rdb)
	assert.Nil(t, err)
	fmt.Println(list)
}

func Test_emoji(t *testing.T) {
	// cord one
	moji, mojiErr := gomoji.GetInfo("🗺️")
	fmt.Printf("%+v \n", moji)

	assert.Nil(t, mojiErr)

	// base iphone one
	moji, mojiErr = gomoji.GetInfo("🗺")
	fmt.Printf("%+v \n", moji)

	moji, mojiErr = gomoji.GetInfo("🗺 ")
	fmt.Printf("%+v \n", moji)

	assert.Nil(t, mojiErr)
	// fmt.Println(list)
}

func Test_rds_delete_phrase(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	phraseToDelete := "son"
	fromEmoji := "<a:abortion:895709963299729498>"

	// phraseToDelete := " FO"
	// fromEmoji := "<:FOBAD:895698729825370152>"

	var phrasesResult []string
	bytes, err := rdb.Get(ctx, fromEmoji).Bytes()
	assert.Nil(t, err)
	err = json.Unmarshal(bytes, &phrasesResult)
	assert.Nil(t, err)
	newList := []string{}
	for _, phrase := range phrasesResult {
		if phrase != phraseToDelete {
			newList = append(newList, phrase)
		}
	}

	bytes, _ = json.Marshal(newList)
	err = rdb.Set(ctx, fromEmoji, bytes, 0).Err()
	assert.Nil(t, err)
}

// AUSTIN TODO: put somewhere
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
