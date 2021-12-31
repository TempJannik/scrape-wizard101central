package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Item struct {
	gorm.Model
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageLink   string `json:"image_link"`
	WikiLink    string `json:"wiki_link"`
	School      string `json:"school"`
	Pack        string `json:"pack"`
}

func main() {
	if len(os.Args) != 1 {
		panic("Please provide a saving method my supplying the parameter mysql or json")
	}

	baseUrlName := "http://www.wizard101central.com"
	allTreasureCards := make([]Item, 0)
	treasureCardLinks := make([]string, 0)
	schools := []string{"Ice", "Fire", "Storm", "Myth", "Death", "Balance", "Life", "Shadow", "Castle Magic", "Sun", "Moon", "Star", "Gardening"}

	linkCollector := colly.NewCollector(
		colly.AllowedDomains("wizard101central.com", "www.wizard101central.com"),
	)

	tcCollector := linkCollector.Clone()
	linkCollector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		if strings.HasPrefix(link, "/wiki/TreasureCard:") {
			treasureCardLinks = append(treasureCardLinks, link)
		}

		if e.Text == "next 200" {
			nextURL := baseUrlName + e.Attr("href")
			linkCollector.Visit(nextURL)
		}
	})

	linkCollector.OnRequest(func(request *colly.Request) {
		fmt.Println("Visiting ", request.URL.String())
	})

	linkCollector.Visit("http://www.wizard101central.com/wiki/index.php?title=Category:Treasure_Cards")

	tcCollector.OnHTML(".mw-body", func(e *colly.HTMLElement) {
		var tc Item
		tc.Pack = ""
		tc.WikiLink = e.Request.URL.String()
		tcName := e.ChildText("span[dir=auto]")
		parsedTcName := strings.Replace(tcName, "TreasureCard:", "", -1)
		tc.Name = parsedTcName

		for _, school := range schools {
			foundSchool := e.ChildAttr("img[alt=\"(Icon) "+school+".png\"]", "alt")
			if foundSchool != "" {
				tc.School = school
			}
		}

		imageSrc := e.ChildAttr("img[alt=\"(Treasure Card) "+tc.Name+".png\"]", "src")
		tc.ImageLink = baseUrlName + imageSrc

		nextDescriptionField := false
		e.ForEach(".data-table", func(i int, table *colly.HTMLElement) {
			table.ForEach("tr", func(a int, row *colly.HTMLElement) {
				if nextDescriptionField {
					tc.Description = row.ChildText("td")
					nextDescriptionField = false
				}

				rowText := row.ChildText("b")
				if rowText == "Description" {
					nextDescriptionField = true
				} else if rowText == "Tradeable" {
					tradeable := row.ChildText("td")
					tradeable = strings.Replace(tradeable, "Tradeable", "", -1)
					if tradeable == "Yes" {
						allTreasureCards = append(allTreasureCards, tc)
					}
				}
			})
		})

		e.ForEach(".data-table.tccardpackvariation", func(i int, table *colly.HTMLElement) {
			table.ForEach("td", func(a int, variation *colly.HTMLElement) {
				pack := variation.ChildText("a")

				imageSrc := variation.ChildAttr("img", "src")
				tc.ImageLink = baseUrlName + imageSrc
				tc.Pack = pack
				allTreasureCards = append(allTreasureCards, tc)
			})
		})
	})

	tcCollector.Limit(&colly.LimitRule{
		DomainGlob:  "*wizard101central*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	for idx, link := range treasureCardLinks {
		fullLink := baseUrlName + link
		fmt.Printf("Visiting %v (%d/%d)\n", fullLink, idx+1, len(treasureCardLinks))
		tcCollector.Visit(fullLink)
	}

	if os.Args[0] == "json" {
		writeJSON(allTreasureCards)
	} else if os.Args[0] == "mysql" {
		BatchInsert(allTreasureCards)
	}

}

func writeJSON(data []Item) {
	file, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		fmt.Println("Error Marhsalling Data")
	}

	_ = ioutil.WriteFile("treasurecards.json", file, 0644)
}

func BatchInsert(data []Item) {
	var DB *gorm.DB

	DB, err := gorm.Open(mysql.Open("username:password@tcp(ip:port)/databasename?charset=utf8&parseTime=True&loc=Local"), &gorm.Config{})

	if err != nil {
		fmt.Println("DB Open Error: ", err)
		return
	}

	DB.Create(&data)
}
