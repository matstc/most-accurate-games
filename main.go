package main

import (
	"log"
	"html/template"
	"net/http"
	"macg/app/acpl"
	"strconv"
	"strings"
)

var resultsTemplate = template.Must(template.ParseFiles("results.html"))
var maxGames = 100
var maxResults = 100

type GameRow struct {
	Rank   int
	ACPL   float64
	Date   string
	White  string
	Black  string
	ResultWhite string
	ResultBlack string
	Result string
	Opening string
	Moves	int
	URL    string
}
func retrieveResults(username string, timeControl string) (error, []acpl.GameACPL) {
	url := "https://lichess.org/api/games/user/" + username + "?analysed=true&tags=true&clocks=false&evals=true&opening=true&literate=false&max=" + strconv.Itoa(maxGames) + "&perfType=" + timeControl

	resp, err := http.Get(url)

	if err != nil {
		return err, nil
	}

	defer resp.Body.Close()

	results, err := acpl.RankByACPL(resp.Body, username)

	if (err != nil) {
		return err, nil
	}

	return nil, results
}

func serveForm(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	timeControl := r.FormValue("time_control")
	err, results := retrieveResults(username, timeControl)

	if err != nil {
		http.Error(w, "Failed to retrieve games: " + err.Error(), http.StatusInternalServerError)
		return
	}

	limit := maxResults

	if limit > len(results) {
		limit = len(results)
	}

	rows := make([]GameRow, 0, limit)

	for i := 0; i < limit; i++ {
		r := results[i]
		g := r.Game
		resultParts := strings.SplitN(acpl.TagValue(g, "Result"), "-", 2)

		rows = append(rows, GameRow{
			Rank:   i + 1,
			ACPL:   r.ACPL,
			Date: acpl.TagValue(g, "Date"),
			White:  acpl.TagValue(g, "White"),
			Black:  acpl.TagValue(g, "Black"),
			ResultWhite: resultParts[0],
			ResultBlack: resultParts[1],
			Result: acpl.TagValue(g, "Result"),
			Opening: strings.SplitN(acpl.TagValue(g, "Opening"), ":", 2)[0],
			Moves:   len(g.Moves()),
			URL:    acpl.TagValue(g, "Site"),
		})
	}

	data := struct {
		Username string
		TimeControl string
		Results []GameRow
	}{
		Username: username,
		TimeControl: timeControl,
		Results: rows,
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

	log.Fatal(http.ListenAndServe(":8080", nil))
	println("Server stopped")
}
