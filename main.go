package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"strings"
	"sync"
	"github.com/PuerkitoBio/goquery"
	"github.com/jedib0t/go-pretty/v6/table"
)

const searchProviderDomain = "1337x.to"
const defaultSearchTermsFilePath = "default_search_terms.txt"

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func main() {
	run(&http.Client{})
}

type torrent struct {
	Name      string
	Seeders   string
	Leechers  string
	CreatedAt string
	Size      string
}

func newTorrent(values ...string) *torrent {
	return &torrent{
		Name:      values[0],
		Seeders:   values[1],
		Leechers:  values[2],
		CreatedAt: values[3],
		Size:      values[4],
	}
}

type searchResult struct {
	Term     string
	Torrents []*torrent
}

type arguments struct {
	searchTerms []string
	resultsPerTerm int
	searchTermSuffix string
}

func run(client HTTPClient) {
	arguments := parseArguments()

	resultsPerTerm := arguments.resultsPerTerm
	searchTerms := determineFinalSearchTerms(
		arguments.searchTerms,
		arguments.searchTermSuffix,
	)

	resultsChan := make(chan *searchResult, len(searchTerms))
	results := []*searchResult{}

	var wg sync.WaitGroup

	for _, searchTerm := range searchTerms {
		wg.Add(1)

		// copying is needed
		st := make([]byte, len(searchTerm))
		copy(st, searchTerm)

		go func() {
			defer wg.Done()
			sr, err := fetchSearchResult(client, string(st), resultsPerTerm, resultsChan)

			if err != nil {
				log.Print(err)
			} else {
				resultsChan <- sr
			}
		}()
	}

	wg.Wait()
	close(resultsChan)

	for result := range resultsChan {
		results = append(results, result)
	}

	printAsTable(results)
}

func parseArguments() *arguments {
	searchTermsPtr := flag.String("terms", "", "Comma-separated search terms")
	resultsPerTermPtr := flag.Int("number", 2, "Number of results per search term")
	searchTermsSuffixPtr := flag.String("suffix", "", "Suffix to be appended to each search term")

	flag.Parse()

	if (*resultsPerTermPtr < 1) {
		log.Fatal("Number of results per search term must be at least 1")
	}

	var searchTerms []string

	if *searchTermsPtr == "" {
		searchTerms = getDefaultSearchTerms()
	} else {
		searchTerms = strings.Split(*searchTermsPtr, ",")
		if len(searchTerms) == 0 {
			log.Fatal("Search terms must be provided")
		}
	}

	return &arguments{
		searchTerms: searchTerms,
		resultsPerTerm: *resultsPerTermPtr,
		searchTermSuffix: *searchTermsSuffixPtr,
	}
}

func getDefaultSearchTerms() []string {
	fileContents, err := os.ReadFile(defaultSearchTermsFilePath)

	if err != nil {
		log.Fatalf("Error while reading default search terms from file: %s", err)
	}

	searchTerms := strings.Split(string(fileContents), "\n")

	if len(searchTerms) == 0 {
		log.Fatal("No search terms in default search terms file")
	}

	return searchTerms
}

func determineFinalSearchTerms(originalSearchTerms []string, suffix string) []string {
	finalSearchTerms := []string{}

	for _, ost := range originalSearchTerms {
		finalSearchTerms = append(finalSearchTerms, ost + " " + suffix)
	}

	return finalSearchTerms
}

func fetchSearchResult(
	client HTTPClient,
	searchTerm string,
	resultsPerTerm int,
	resultsChan chan *searchResult,
) (*searchResult, error) {
	targetUrl := "https://" + searchProviderDomain + "/sort-search/" + searchTerm + "/time/desc/1/"

	request, err := http.NewRequest(http.MethodGet, targetUrl, nil)

	if err != nil {
		return nil, err
	}

	res, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)

	if err != nil {
		return nil, err
	}

	torrents := extractTorrents(doc, resultsPerTerm)

	return &searchResult{
		Term:     searchTerm,
		Torrents: torrents,
	}, nil
}

func extractTorrents(doc *goquery.Document, torrentsPerTerm int) []*torrent {
	torrents := []*torrent{}

	doc.Find("tr").Each(func(i int, row *goquery.Selection) {
		if i == 0 || i > torrentsPerTerm {
			return
		}

		row.Each(func(j int, col *goquery.Selection) {
			allCols := strings.Split(col.Text(), "\n")

			filledCols := []string{}

			for _, val := range allCols {
				if val != "" {
					filledCols = append(filledCols, val)
				}
			}

			torrents = append(torrents, newTorrent(filledCols...))
		})
	})

	return torrents
}

func printAsTable(searchResults []*searchResult) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendHeader(table.Row{"Name", "Seeders", "Leechers", "Created At", "Size"})

	for _, result := range searchResults {
		t.AppendRow(table.Row{"Search term: " + result.Term})
		t.AppendRow(table.Row{""})

		for _, torrent := range result.Torrents {
			t.AppendRow(table.Row{
				torrent.Name, torrent.Seeders, torrent.Leechers, torrent.CreatedAt, torrent.Size,
			})
		}

		t.AppendSeparator()
	}

	t.SetStyle(table.StyleRounded)
	t.Render()
}
