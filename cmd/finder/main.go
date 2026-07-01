package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/farizakb/ad-finder/internal/finder"
)

func main() {
	record := flag.String("record", "", "Path to recording MP3 file")
	advert := flag.String("advert", "", "Path to reference ad MP3 file")
	output := flag.String("output", "text", "Output format: json or text")

	recordsDir := flag.String("records-dir", "", "Directory of recording MP3 files (batch mode)")
	advertsDir := flag.String("adverts-dir", "", "Directory of reference ad MP3 files (batch mode)")
	workers := flag.Int("workers", 4, "Number of parallel workers (batch mode)")

	flag.Parse()

	if *recordsDir != "" && *advertsDir != "" {
		runBatch(*recordsDir, *advertsDir, *workers, *output)
	} else if *record != "" && *advert != "" {
		runSingle(*record, *advert, *output)
	} else {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  Single: finder --record FILE --advert FILE [--output json|text]")
		fmt.Fprintln(os.Stderr, "  Batch:  finder --records-dir DIR --adverts-dir DIR [--workers N] [--output json|text]")
		os.Exit(1)
	}
}

func runSingle(recordPath, advertPath, outputFmt string) {
	matches, err := finder.FindAdvertInRecord(recordPath, advertPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	printMatches(recordPath, advertPath, matches, outputFmt)
}

type BatchResult struct {
	Record  string         `json:"record"`
	Advert  string         `json:"advert"`
	Matches []finder.Match `json:"matches"`
}

func runBatch(recordsDir, advertsDir string, numWorkers int, outputFmt string) {
	records := listMP3s(recordsDir)
	adverts := listMP3s(advertsDir)

	if len(records) == 0 {
		log.Fatalf("No MP3 files found in %s", recordsDir)
	}
	if len(adverts) == 0 {
		log.Fatalf("No MP3 files found in %s", advertsDir)
	}

	fmt.Fprintf(os.Stderr, "Batch: %d recordings × %d adverts, %d workers\n",
		len(records), len(adverts), numWorkers)

	var mu sync.Mutex
	var actualPairs int

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	start := time.Now()

	for i, rec := range records {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", i+1, len(records), filepath.Base(rec))

		fp, err := finder.FingerprintRecord(rec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip: %v\n", err)
			continue
		}

		work := make(chan string, len(adverts))
		for _, adv := range adverts {
			work <- adv
		}
		close(work)

		var recResults []BatchResult
		var wg sync.WaitGroup
		for j := 0; j < numWorkers; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for adv := range work {
					matches, err := finder.FindAdvertWithFingerprint(fp, adv)
					if err != nil {
						fmt.Fprintf(os.Stderr, "  error %s: %v\n", filepath.Base(adv), err)
						continue
					}
					mu.Lock()
					recResults = append(recResults, BatchResult{
						Record:  rec,
						Advert:  adv,
						Matches: matches,
					})
					actualPairs++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		// Flush this recording's results immediately so progress is not lost on crash.
		if outputFmt == "json" {
			for _, r := range recResults {
				enc.Encode(r)
			}
		} else {
			for _, r := range recResults {
				printMatches(r.Record, r.Advert, r.Matches, "text")
			}
		}
	}

	elapsed := time.Since(start)
	pairsPerSec := float64(actualPairs) / elapsed.Seconds()
	fmt.Fprintf(os.Stderr, "\nCompleted %d pairs in %s (%.1f pairs/sec)\n",
		actualPairs, elapsed.Round(time.Millisecond), pairsPerSec)
}

func listMP3s(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("Cannot read directory %s: %v", dir, err)
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp3") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths
}

func printMatches(record, advert string, matches []finder.Match, format string) {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(matches)
		return
	}

	fmt.Printf("Record: %s\n", filepath.Base(record))
	fmt.Printf("Advert: %s\n", filepath.Base(advert))
	if len(matches) == 0 {
		fmt.Println("Result: No matches found")
	} else {
		fmt.Printf("Result: %d match(es)\n", len(matches))
		for i, m := range matches {
			mins := int(m.TimeSec) / 60
			secs := m.TimeSec - float64(mins*60)
			fmt.Printf("  %d) %02d:%05.2f  duration=%.1fs  confidence=%.4f\n",
				i+1, mins, secs, m.DurationSec, m.Confidence)
		}
	}
	fmt.Println()
}
