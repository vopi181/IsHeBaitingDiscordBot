package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	csgopb "github.com/13k/go-steam-resources/v2/steampb/csgo"
	"github.com/bwmarrin/discordgo"
	"github.com/golang/protobuf/proto"
	"github.com/vopi181/IsHeBaitingDiscordBot/demoparsing"
)

//CSGO-6hBft-94wkr-YPtCw-o5De7-Vp8eD
//^CSGO((-?[\w]{5}){5})$

type decodedShareCode struct {
	MatchID   uint64
	OutcomeID uint64
	TokenID   uint32
}

const DICTIONARY = "ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789"

func isProperCode(sharelink string) bool {
	// var re = regexp.MustCompile(`^CSGO((-?[\w]{5}){5})$`)

	found, _ := regexp.MatchString(`^CSGO((-?[\w]{5}){5})$`, sharelink)
	return found
}

//https://github.com/akiver/CSGO-Demos-Manager/blob/7abb325ad3663732ca585addee52383a78751314/Core/ShareCode.cs#L79
func decodeShareCode(sharelink string) (decodedShareCode, error) {
	log.Println("trying to decode: " + sharelink)
	if !isProperCode(sharelink) {
		return decodedShareCode{}, errors.New("Bad Formatting on code")
	}
	bign := new(big.Int)

	code := strings.Replace(sharelink[4:], "-", "", -1)

	for _, c := range Reverse(code) {
		bign = bign.Add(bign.Mul(bign, big.NewInt(int64(len([]rune(DICTIONARY))))), big.NewInt(int64(strings.Index(DICTIONARY, string(c)))))
	}

	// var basicSizes = [...]byte{
	// 	Bool:       1,
	// 	Int8:       1,
	// 	Int16:      2,
	// 	Int32:      4,
	// 	Int64:      8,
	// 	Uint8:      1,
	// 	Uint16:     2,
	// 	Uint32:     4,
	// 	Uint64:     8,
	// 	Float32:    4,
	// 	Float64:    8,
	// 	Complex64:  8,
	// 	Complex128: 16,
	// }
	all := bign.Bytes()
	// sometimes the number isn't unsigned, add a 00 byte at the end of the array to make sure it is
	if len(all) != 2*8+(4) {
		all = append(all, byte(0))
		all = append(all, byte(0))
	}

	MatchIDBytes := all[0:8]
	OutcomeIDBytes := all[8 : 8*2]
	TvPortIDBytes := all[8*2 : (8*2)+4]

	ret := decodedShareCode{MatchID: binary.LittleEndian.Uint64(MatchIDBytes),
		OutcomeID: binary.LittleEndian.Uint64(OutcomeIDBytes),
		TokenID:   binary.LittleEndian.Uint32(TvPortIDBytes)}

	return ret, nil
}

func downloadBinaryProtoFromCode(sharelink string, demofile string) error {
	sc, err := decodeShareCode(sharelink)
	// fmt.Printf("%v %v", sc, err)
	if err != nil {
		return err
	}
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	executableDir := filepath.Dir(ex)

	boilerPath := ""
	if runtime.GOOS == "windows" {
		boilerPath = filepath.Join(executableDir, "boilerbins_windows/boiler-writter.exe")
	} else if runtime.GOOS == "linux" {
		boilerPath = filepath.Join(executableDir, "boilerbins_linux/boiler-writter")

	}

	cmd := exec.Command(boilerPath, demofile, fmt.Sprint(sc.MatchID), fmt.Sprint(sc.OutcomeID), fmt.Sprint(sc.TokenID))
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func protobinToDem(pbFile string) (csgopb.CMsgGCCStrike15V2_MatchList, error) {
	log.Printf("Converting bin %v to demo\n", pbFile)
	f, err := os.Open(pbFile)
	if err != nil {
		log.Fatal(err)
	}
	bufs, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	msg := &csgopb.CMsgGCCStrike15V2_MatchList{}
	proto.Unmarshal(bufs, msg)
	return *msg, err
}

func GetBaitStatsFromCode(code string) string {
	df := RandomString(12)
	err := downloadBinaryProtoFromCode(code, "output/"+df+".protobin")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Downloaded " + code)

	demoproto, err := protobinToDem("output/" + df + ".protobin")
	if err != nil {
		log.Fatal(err)
	}
	match := demoparsing.GetMatchFromDemoProto(demoproto)
	demoparsing.DownAndExtractDemoArchive(match)
	demoName := "output/" + demoparsing.GetDemoName(match) + ".dem"
	return demoparsing.IsBaitingFile(demoName, 0, true)

}

// Variables used for command line parameters
var (
	Token string
)

func init() {

	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	fmt.Println("IsHeBaiting Discord Bot")
	log.Println("Starting bot...")
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// we only care about receiving message events.
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)
	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Fatalln("error opening connection,", err)
		return
	}
	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()

	// println(GetBaitStatsFromCode("CSGO-6hBft-94wkr-YPtCw-o5De7-Vp8eD"))
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}
	if m.Content[:10] == "-baitcheck" {
		msgCode := strings.Split(m.Content, " ")[1]
		if isProperCode(msgCode) {
			s.MessageReactionAdd(m.ChannelID, m.ID, "🧠")
			s.ChannelMessageSend(m.ChannelID, "```"+GetBaitStatsFromCode(msgCode)+"```")
		} else {
			s.ChannelMessageSend(m.ChannelID, "Error: Bad Formatting. Proper Formatting is\n```-baitcheck CSGO-6hBft-94wkr-YPtCw-o5De7-Vp8eD```")
		}
	}

}
