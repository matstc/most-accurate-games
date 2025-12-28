package acpl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/notnil/chess"
)

type GameACPL struct {
	Game *chess.Game
	ACPL float64
}

func splitPGN(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// PGN games are separated by two newlines
	if i := bytes.Index(data, []byte("\n\n\n")); i >= 0 {
		return i + 3, data[:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}

// parse [%eval X] from comment
func parseEval(comment string) (float64, bool) {
	const key = "%eval "
	i := strings.Index(comment, key)
	if i == -1 {
		return 0, false
	}

	s := comment[i+len(key):]

	// Lichess formats mates like: "#3", "#-1"
	if strings.HasPrefix(s, "#") {
		// check sign
		if strings.HasPrefix(s, "#-") {
			return -1000, true
		}
		return 1000, true
	}

	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	if err != nil {
		return 0, false
	}

	return v * 100, true // convert to centipawns
}

func computeACPL(game *chess.Game, username string) (float64, bool) {
	var white, black string

	for _, t := range game.TagPairs() {
		switch t.Key {
		case "White":
			white = t.Value
		case "Black":
			black = t.Value
		}
	}

	isWhite := strings.EqualFold(white, username)
	isBlack := strings.EqualFold(black, username)
	if !isWhite && !isBlack {
		return 0, false
	}

	moves := game.Moves()
	comments := game.Comments()

	var (
		totalLoss float64
		count     int
		prevEval  float64
		hasPrev   bool
	)

	for i := 0; i < len(moves) && i < len(comments); i++ {
		if len(comments[i]) == 0 {
			continue
		}

		// use last comment for the move
		comment := comments[i][len(comments[i])-1]

		eval, ok := parseEval(comment)
		if !ok {
			continue
		}

		if eval > 1000 {
			eval = 1000
		} else if eval < -1000 {
			eval = -1000
		}

		whiteMove := i%2 == 0
		playerMove := (whiteMove && isWhite) || (!whiteMove && isBlack)

		if playerMove && hasPrev {
			loss := prevEval - eval

			// normalize from player's perspective
			if isBlack {
				loss = -loss
			}
			if loss < 0 {
				loss = 0
			}

			totalLoss += loss
			count++
		}

		// update baseline for next ply (always)
		prevEval = eval
		hasPrev = true
	}

	if count == 0 {
		return 0, false
	}

	return totalLoss / float64(count), true
}

func RankByACPL(r io.Reader, username string, minPlies int) ([]GameACPL, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitPGN)

	var out []GameACPL

	for scanner.Scan() {
		pgn := scanner.Text()
		if strings.TrimSpace(pgn) == "" {
			continue
		}

		opt, err := chess.PGN(strings.NewReader(pgn))
		if err != nil {
			continue // malformed PGN
		}

		game := chess.NewGame(opt)

		if len(game.Moves()) < minPlies {
			continue
		}

		acpl, ok := computeACPL(game, username)
		if !ok {
			continue
		}

		out = append(out, GameACPL{
			Game: game,
			ACPL: acpl,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ACPL < out[j].ACPL
	})

	return out, scanner.Err()
}

func TagValue(g *chess.Game, key string) string {
	for _, t := range g.TagPairs() {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}
