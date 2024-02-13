package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/meilisearch/meilisearch-go"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type RaindropCollection struct {
	ID            int       `json:"_id"`
	Access        Access    `json:"access"`
	Collaborators struct{}  `json:"collaborators"` // Assuming you don't need details here
	Color         string    `json:"color"`
	Count         int       `json:"count"`
	Cover         []string  `json:"cover"`
	Created       time.Time `json:"created"`
	Expanded      bool      `json:"expanded"`
	LastUpdate    time.Time `json:"lastUpdate"`
	Parent        *Parent   `json:"parent"` // Optional, hence a pointer
	Public        bool      `json:"public"`
	Sort          int       `json:"sort"`
	Title         string    `json:"title"`
	User          User      `json:"user"`
	View          string    `json:"view"`
}

type Access struct {
	Level     int  `json:"level"`
	Draggable bool `json:"draggable"`
}

type Parent struct {
	ID int `json:"$id"`
}

type User struct {
	ID int `json:"$id"`
}

type RaindropCollectionResponse struct {
	Result      bool                 `json:"result"`
	Collections []RaindropCollection `json:"items"`
}

type Raindrop struct {
	ID         int `json:"_id"`
	Collection struct {
		ID int `json:"$id"`
	} `json:"collection"`
	Cover      string    `json:"cover"`
	Created    time.Time `json:"created"`
	Domain     string    `json:"domain"`
	Excerpt    string    `json:"excerpt"`
	Note       string    `json:"note"`
	LastUpdate time.Time `json:"lastUpdate"`
	Link       string    `json:"link"`
	Media      []struct {
		Link string `json:"link"`
	} `json:"media"`
	Tags  []string `json:"tags"`
	Title string   `json:"title"`
	Type  string   `json:"type"`
	User  struct {
		ID int `json:"$id"`
	} `json:"user"`
	Broken bool `json:"broken"`
	Cache  struct {
		Status  string    `json:"status"`
		Size    int       `json:"size"`
		Created time.Time `json:"created"`
	} `json:"cache"`
	CreatorRef struct {
		ID       int    `json:"_id"`
		FullName string `json:"fullName"`
	} `json:"creatorRef"`
	File struct {
		Name string `json:"name"`
		Size int    `json:"size"`
		Type string `json:"type"`
	} `json:"file"`
	Important  bool `json:"important"`
	Highlights []struct {
		ID      string    `json:"_id"`
		Text    string    `json:"text"`
		Color   string    `json:"color"`
		Note    string    `json:"note"`
		Created time.Time `json:"created"`
	} `json:"highlights"`
}

type RaindropsResponse struct {
	Items []Raindrop `json:"items"`
}

func main() {
	indexFlag := flag.Bool("i", false, "Index bookmarks")
	flag.Parse()

	var raindropToken = os.Getenv("DROPSEARCH_RAINDROP_TOKEN")
	var searchToken = os.Getenv("DROPSEARCH_MEILISEARCH_TOKEN")

	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   "http://search",
		APIKey: searchToken,
	})

	if *indexFlag {
		indexBookmarks(client, raindropToken)
		return
	}

	searchQuery := strings.Join(flag.Args(), " ")
	if searchQuery != "" {
		searchBookmarks(client, searchQuery)
		return
	}

	fmt.Println("Usage: dropsearch [-i] [search query]")
}

func indexBookmarks(client *meilisearch.Client, raindropToken string) {
	log.Println("indexing started")
	s := spinner.New(spinner.CharSets[35], 100*time.Millisecond)
	s.Color("fgHiGreen")
	s.Prefix = color.HiCyanString("Indexing: ")
	s.Start()
	defer s.Stop()

	s.Suffix = " getting collections list"
	collections, err := getCollections(raindropToken)
	if err != nil {
		log.Fatalln(err)
	}

	var allRaindrops []Raindrop
	for _, collection := range collections {
		s.Suffix = fmt.Sprintf(" getting raindrops for '%s'", collection.Title)
		raindrops, err := getRaindropsInCollection(collection.ID, raindropToken)
		if err != nil {
			log.Fatalln(err)
		}
		allRaindrops = append(allRaindrops, raindrops...)
	}

	s.Suffix = " inserting into meilisearch index"
	index := client.Index("raindrops")
	_, err = index.AddDocuments(allRaindrops)
	if err != nil {
		log.Fatalln(err)
	}

	s.Stop()
	numDocuments := len(allRaindrops)
	log.Printf("%d documents indexed", numDocuments)
}

func getRaindropsInCollection(collectionId int, raindropToken string) ([]Raindrop, error) {
	url := fmt.Sprintf("http://api.raindrop.io/rest/v1/raindrops/%d", collectionId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+raindropToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var raindropsResponse RaindropsResponse
	err = json.Unmarshal(body, &raindropsResponse)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return raindropsResponse.Items, nil
}

func getCollections(raindropToken string) ([]RaindropCollection, error) {
	url := "http://api.raindrop.io/rest/v1/collections"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+raindropToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var collectionResponse RaindropCollectionResponse
	err = json.Unmarshal(body, &collectionResponse)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return collectionResponse.Collections, nil
}

func searchBookmarks(client *meilisearch.Client, query string) {
	searchResult, err := client.Index("raindrops").Search(query,
		&meilisearch.SearchRequest{
			Limit: 10,
		})
	if err != nil {
		log.Fatalln(err)
	}

	hitCountStr := fmt.Sprintf("%d", len(searchResult.Hits))
	hitCountColor := color.New(color.FgHiYellow).SprintfFunc()
	queryColor := color.New(color.FgHiCyan).SprintFunc()
	log.Println("found", hitCountColor(hitCountStr), "hits for", queryColor(query))

	titleColor := color.New(color.FgGreen).SprintFunc()
	linkColor := color.New(color.FgBlue).SprintFunc()
	infoColor := color.New(color.Faint).SprintFunc()
	tagColor := color.New(color.FgYellow).SprintFunc()

	for i, hit := range searchResult.Hits {
		hitBytes, err := json.Marshal(hit)
		if err != nil {
			log.Println("error marshalling bytes to json:", err)
			continue
		}

		var raindrop Raindrop
		err = json.Unmarshal(hitBytes, &raindrop)
		if err != nil {
			log.Fatalln("enmarshal error:", err)
		}

		fmt.Printf("%d. %s\n", i+1, titleColor(raindrop.Title))
		fmt.Printf("   Link: %s\n", linkColor(raindrop.Link))
		if raindrop.Excerpt != "" {
			fmt.Printf("   Excerpt: %s\n", raindrop.Excerpt)
		}
		fmt.Printf("   Domain: %s, Created: %s\n", infoColor(raindrop.Domain), infoColor(raindrop.Created.Format("2006-01-02")))
		if len(raindrop.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", tagColor(strings.Join(raindrop.Tags, ", ")))
		}
		fmt.Println()
	}

}
