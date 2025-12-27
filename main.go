package main

import (
	"fmt"
	"html/template"
	"log"
	"macg/app/acpl"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var resultsTemplate = template.Must(template.ParseFiles("results.html"))
var maxGames = 1000
var maxResults = 100

type GameRow struct {
	Rank        int
	ACPL        float64
	Date        string
	White       string
	WhiteElo    string
	Black       string
	BlackElo    string
	ResultWhite string
	ResultBlack string
	Result      string
	Opening     string
	Moves       int
	URL         string
}

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

type RateLimiter struct {
	tokens chan struct{}
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
	rl := &RateLimiter{
		tokens: make(chan struct{}, burst),
	}

	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}

	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(rps))
		defer ticker.Stop()
		for range ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
				// bucket full
			}
		}
	}()

	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-rl.tokens:
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		}
	})
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d (%s)", e.StatusCode, e.Status)
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

	http.ServeFile(w, r, "index.html")
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

		rows = append(rows, GameRow{
			Rank:        i + 1,
			ACPL:        r.ACPL,
			Date:        acpl.TagValue(g, "Date"),
			White:       acpl.TagValue(g, "White"),
			WhiteElo:    acpl.TagValue(g, "WhiteElo"),
			Black:       acpl.TagValue(g, "Black"),
			BlackElo:    acpl.TagValue(g, "BlackElo"),
			ResultWhite: resultParts[0],
			ResultBlack: resultParts[1],
			Result:      acpl.TagValue(g, "Result"),
			Opening:     strings.SplitN(acpl.TagValue(g, "Opening"), ":", 2)[0],
			Moves:       len(g.Moves()) / 2,
			URL:         acpl.TagValue(g, "Site"),
		})
	}

	data := struct {
		Username    string
		TimeControl string
		Results     []GameRow
		Message     string
	}{
		Username:    username,
		TimeControl: timeControl,
		Results:     rows,
		Message:     message,
	}

	resultsTemplate.Execute(w, data)
}

func main() {
	println("Defining handlers")

	http.HandleFunc("/styles.css", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "styles.css") })
	http.HandleFunc("/favicon.png", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "favicon.png") })
	http.HandleFunc("/", serveForm)
	http.HandleFunc("/go", handleForm)

	println("Starting server")

	limiter := NewRateLimiter(5, 10)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      limiter.Middleware(http.DefaultServeMux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	log.Fatal(server.ListenAndServe())

	println("Server stopped")
}
