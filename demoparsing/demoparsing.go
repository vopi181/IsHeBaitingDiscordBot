package demoparsing

import (
	"bytes"
	"compress/bzip2"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"text/tabwriter"

	csgopb "github.com/13k/go-steam-resources/v2/steampb/csgo"
	dem "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs"
	events "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/events"
)

type baitStruct struct {
	name       string
	totalBaits int
}

//ders id
// 76561198128945703

// Takes file path, steam id or topbaiters bool
func IsBaitingFile(fd string, id int64, topBaiters bool) string {
	f, err := os.Open(fd)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	defer p.Close()
	totalBaits := 0
	pname := ""

	var tb map[int64]*baitStruct
	tb = make(map[int64]*baitStruct)

	p.RegisterEventHandler(func(e events.MatchStart) {
		for _, ply := range p.GameState().Participants().All() {
			if ply.SteamID64 != 0 {
				if topBaiters {
					tb[int64(ply.SteamID64)] = &baitStruct{name: ply.Name, totalBaits: 0}
					// fmt.Printf("[%v] %v\n", ply.SteamID64, tb[int64(ply.SteamID64)].name)
				} else {
					// fmt.Printf("[%v] %v\n", ply.SteamID64, ply.Name)
				}
			}
		}
	})

	p.RegisterEventHandler(func(e events.Kill) {
		if p.GameState().IsMatchStarted() {
			// fmt.Printf("[round #%v] %v killed %v\n", p.GameState().TotalRoundsPlayed(), e.Killer.Name, e.Victim.Name)
			if e.Victim.SteamID64 == uint64(id) || topBaiters {
				pname = e.Victim.Name
				// Calculate people alive on team
				aliveteam := 0

				for _, tm := range e.Victim.TeamState.Members() {
					if tm.LastAlivePosition != tm.Position() {
						aliveteam = aliveteam + 1

					} else {

					}
				}

				if aliveteam == 1 {
					// println("Potential Baiter: " + e.Victim.Name + " at " + fmt.Sprint(p.GameState().TotalRoundsPlayed()))
					totalBaits = totalBaits + 1
					if topBaiters {
						tb[int64(e.Victim.SteamID64)].totalBaits = (tb[int64(e.Victim.SteamID64)].totalBaits) + 1
					}

				}

			}
		}
	})
	ret := ""
	retBuff := bytes.NewBufferString(ret)
	p.RegisterEventHandler(func(e events.AnnouncementWinPanelMatch) {
		if !topBaiters {
			roundsPlayed := float32(p.GameState().TotalRoundsPlayed())
			fTotalBaits := float32(totalBaits)
			fmt.Println("======BAIT CALC======")
			fmt.Printf("Player: %v(%v) on %v\n", pname, id, p.Header().MapName)
			fmt.Printf("Baited %v/%v = %v percent of rounds\n", totalBaits, p.GameState().TotalRoundsPlayed(), (fTotalBaits/roundsPlayed)*100)
			fmt.Println("=====================")
		} else {
			ret = ""
			ret += "======BAIT CALC======\n"
			ret += "MAP: " + p.Header().MapName + "\n"
			retBuff = bytes.NewBufferString(ret)
			w := new(tabwriter.Writer)
			w.Init(retBuff, 30, 1, 0, '-', tabwriter.Debug)

			fmt.Fprint(w, "name\tid\tpercent of rounds baited\n")
			for _, ply := range p.GameState().Participants().All() {
				if ply.SteamID64 != 0 {
					roundsPlayed := float32(p.GameState().TotalRoundsPlayed())
					fTotalBaits := float32(tb[int64(ply.SteamID64)].totalBaits)
					fmt.Fprintf(w, "%v\t%v\t%v\n", ply.Name, ply.SteamID64, (fTotalBaits/roundsPlayed)*100)
					// fmt.Printf("Player: %v(%v) ", ply.Name, ply.SteamID64)
					// fmt.Printf("Baited %v/%v = %v percent of rounds\n", tb[int64(ply.SteamID64)].totalBaits, p.GameState().TotalRoundsPlayed(), (fTotalBaits/roundsPlayed)*100)
					w.Flush()
				}
			}
			// fmt.Println("=====================")

		}
	})
	err = p.ParseToEnd()
	if err != nil {
		panic(err)
	}
	return retBuff.String()

}

// Returns demo name
func GetDemoName(info csgopb.CDataGCCStrike15V2_MatchInfo) string {
	roundstats := info.GetRoundstatsall()
	roundstatsDeref := *roundstats[len(roundstats)-1]
	resid := *roundstatsDeref.Reservationid
	tvport := *info.Watchablematchinfo.TvPort
	serverip := *info.Watchablematchinfo.ServerIp
	url := "match730"
	url = url + "_" + fmt.Sprintf("%v", resid)
	url = url + "_" + fmt.Sprintf("%v", tvport)
	url = url + "_" + fmt.Sprintf("%v", serverip)
	return url

}

// Get Match (Should only be one if all goes well)
func GetMatchFromDemoProto(dp csgopb.CMsgGCCStrike15V2_MatchList) csgopb.CDataGCCStrike15V2_MatchInfo {
	return (*dp.Matches[0])
}

// Download and unbz2 demo archive
func DownAndExtractDemoArchive(m csgopb.CDataGCCStrike15V2_MatchInfo) {
	roundstats := (m).GetRoundstatsall()
	rs := *roundstats[len(roundstats)-1]
	log.Printf("Downloading %v", rs.GetMap())
	archiveName := "output/" + GetDemoName(m) + ".dem.bz2"
	downloadFile(archiveName, rs.GetMap())

	data, _ := ioutil.ReadFile(archiveName)
	buffer := bytes.NewBuffer(data)
	bzreader := bzip2.NewReader(buffer)
	bzbytes, _ := ioutil.ReadAll(bzreader)
	ioutil.WriteFile(archiveName[:len(archiveName)-4], bzbytes, os.ModePerm)
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
