package main

import (
	"fmt"
	"html/template"
	"log"
	"macg/app/acpl"
	"macg/app/rate_limiter"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var templates = template.Must(template.ParseFiles("index.html", "results.html", "footer.html"))
var maxGames = 1000
var maxResults = 50

type GameRow struct {
	GameId        string
	Rank          int
	ACPL          float64
	FormattedDate string
	White         string
	WhiteElo      string
	Black         string
	BlackElo      string
	ResultWhite   string
	ResultBlack   string
	Result        string
	Opening       string
	Moves         int
	URL           string
}

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d (%s)", e.StatusCode, e.Status)
}

func setCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
}

func retrieveResults(username string, timeControl string, ratedOnly bool, minPlies int) ([]acpl.GameACPL, error) {
	url := "https://lichess.org/api/games/user/" + username + "?analysed=true&tags=true&clocks=false&evals=true&opening=true&literate=false&max=" + strconv.Itoa(maxGames) + "&perfType=" + timeControl

	if ratedOnly {
		url += "&rated=true"
	}

	resp, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	results, err := acpl.RankByACPL(resp.Body, username, minPlies)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func serveForm(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving form to %s", r.RemoteAddr)

	setCacheHeaders(w)
	if err := templates.ExecuteTemplate(w, "index.html", struct{}{}); err != nil {
		log.Printf("Error rendering index template: %v", err)
	}
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling form for %s", r.RemoteAddr)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Printf("Received form from %s: %+v", r.RemoteAddr, r.Form)

	username := r.FormValue("username")
	timeControl := r.FormValue("time_control")
	ratedOnly := r.FormValue("rated_only")
	excludeMiniatures := r.FormValue("exclude_miniatures")
	message := ""
	minPlies := 0

	if excludeMiniatures == "true" {
		minPlies = 40
	}

	results, err := retrieveResults(username, timeControl, ratedOnly == "true", minPlies)

	if err != nil {
		log.Printf("Error retrieving results for %s: %v", r.RemoteAddr, err)
		message = "Failed to retrieve games: " + err.Error()
		results = []acpl.GameACPL{}
	}

	limit := maxResults

	if limit > len(results) {
		limit = len(results)
	}

	if len(results) == 0 {
		message += "\n\nNo games found. Make sure the username is correct and that games with computer analysis are available."
	}

	rows := make([]GameRow, 0, limit)

	for i := 0; i < limit; i++ {
		r := results[i]
		g := r.Game
		resultParts := strings.SplitN(acpl.TagValue(g, "Result"), "-", 2)
		t, _ := time.Parse("2006.01.02", acpl.TagValue(g, "Date"))

		rows = append(rows, GameRow{
			GameId:        acpl.TagValue(g, "GameId"),
			Rank:          i + 1,
			ACPL:          r.ACPL,
			FormattedDate: t.Format("Jan 2, 2006"),
			White:         acpl.TagValue(g, "White"),
			WhiteElo:      acpl.TagValue(g, "WhiteElo"),
			Black:         acpl.TagValue(g, "Black"),
			BlackElo:      acpl.TagValue(g, "BlackElo"),
			ResultWhite:   resultParts[0],
			ResultBlack:   resultParts[1],
			Opening:       strings.SplitN(acpl.TagValue(g, "Opening"), ",", 2)[0],
			Moves:         len(g.Moves()) / 2,
			URL:           acpl.TagValue(g, "Site"),
		})
	}

	timeControlCharacter := ""

	switch timeControl {
	case "bullet":
		timeControlCharacter = "➤"
	case "blitz":
		timeControlCharacter = "🔥"
	case "rapid":
		timeControlCharacter = "🐇"
	case "classical":
		timeControlCharacter = "🐢"
	}

	data := struct {
		Username             string
		TimeControl          string
		TimeControlCharacter string
		Results              []GameRow
		Message              string
	}{
		Username:             username,
		TimeControl:          timeControl,
		TimeControlCharacter: timeControlCharacter,
		Results:              rows,
		Message:              message,
	}

	setCacheHeaders(w)
	if err := templates.ExecuteTemplate(w, "results.html", data); err != nil {
		log.Printf("Error rendering results template: %v", err)
	}
}

func main() {
	println("Defining handlers")

	http.HandleFunc("/Atkinson-Hyperlegible-SIL-OPEN-FONT-LICENSE-Version%201.1-v2%20ACC.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "Atkinson-Hyperlegible-SIL-OPEN-FONT-LICENSE-Version 1.1-v2 ACC.pdf")
	})
	http.HandleFunc("/AtkinsonHyperlegibleNext-Regular.otf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "AtkinsonHyperlegibleNext-Regular.otf")
	})
	http.HandleFunc("/AtkinsonHyperlegibleNext-Bold.otf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "AtkinsonHyperlegibleNext-Bold.otf")
	})
	http.HandleFunc("/styles.css", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "styles.css") })
	http.HandleFunc("/favicon.png", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "favicon.png") })
	http.HandleFunc("/", serveForm)
	http.HandleFunc("/go", handleForm)

	println("Starting server")

	server := &http.Server{
		Addr:         ":8080",
		Handler:      rate_limiter.NewRateLimiter(5, 10).Middleware(http.DefaultServeMux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	log.Fatal(server.ListenAndServe())

	println("Server stopped")
}
