package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http" // Renamed to gitHttp
)

const (
	githubRepo    = "https://github.com/dareaquatics/dare-website"
	newsURL       = "https://www.gomotionapp.com/team/cadas/page/news"
	repoName      = "dare-website"
	newsFile      = "news.html"
	startMarker   = "<!-- START UNDER HERE -->"
	endMarker     = "<!-- END AUTOMATION SCRIPT -->"
)

type NewsItem struct {
	Title   string
	Date    string
	Summary string
	Author  string
}

func checkGithubToken() error {
	token := os.Getenv("PAT_TOKEN")
	if token == "" {
		return fmt.Errorf("PAT_TOKEN environment variable not set")
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("Authorization", "token "+token)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error validating token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid GitHub token")
	}
	return nil
}

func cloneRepository() error {
	token := os.Getenv("PAT_TOKEN")
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %v", err)
	}

	repoPath := filepath.Join(currentDir, repoName)
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		os.RemoveAll(repoPath)
	}

	_, err = git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:      githubRepo,
		Progress: os.Stdout,
		Auth: &gitHttp.BasicAuth{ // Renamed http to gitHttp
			Username: "git",
			Password: token,
		},
	})
	if err != nil {
		return fmt.Errorf("error cloning repository: %v", err)
	}

	if err := os.Chdir(repoPath); err != nil {
		return fmt.Errorf("error changing directory: %v", err)
	}
	return nil
}

func fetchNews() ([]NewsItem, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", newsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching news: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	var newsItems []NewsItem
	doc.Find("div.Item").Each(func(i int, s *goquery.Selection) {
		if !s.HasClass("Supplement") {
			if href, exists := s.Find("a").Attr("href"); exists {
				articleURL := "https://www.gomotionapp.com" + href
				if item, err := fetchArticleContent(articleURL); err == nil {
					newsItems = append(newsItems, item)
				}
			}
		}
	})

	sort.Slice(newsItems, func(i, j int) bool {
		date1, _ := time.Parse("January 02, 2006", newsItems[i].Date)
		date2, _ := time.Parse("January 02, 2006", newsItems[j].Date)
		return date1.After(date2)
	})

	return newsItems, nil
}

func fetchArticleContent(url string) (NewsItem, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return NewsItem{}, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return NewsItem{}, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return NewsItem{}, err
	}

	newsItem := NewsItem{
		Title:   doc.Find("div.NewsItem h1").Text(),
		Author:  doc.Find("div.Author strong").Text(),
		Date:    "Unknown Date",
	}

	if dateStr, exists := doc.Find("span.DateStr").Attr("data"); exists {
		timestamp, err := strconv.ParseInt(dateStr, 10, 64) // Fixed strconv undefined issue
		if err == nil {
			date := time.Unix(timestamp/1000, 0)
			newsItem.Date = date.Format("January 02, 2006")
		}
	}

	contentHTML := new(bytes.Buffer)
	doc.Find("div.Content").Each(func(i int, s *goquery.Selection) {
		s.Find("img").Each(func(i int, img *goquery.Selection) {
			if src, exists := img.Attr("src"); exists {
				if !strings.HasPrefix(src, "http") {
					src = "http://www.gomotionapp.com" + src
				}
				fmt.Fprintf(contentHTML, `<a href="%s" target="_blank"><img src="%s" style="max-width:100%%; height:auto;" alt="Image"/></a>`, src, src)
			}
		})
		s.Find("a").Each(func(i int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			a.SetText("Click here to be redirected to the link")
			a.SetAttr("target", "_blank")
			a.SetAttr("href", href)
		})
		html, _ := s.Html()
		contentHTML.WriteString(html)
	})

	newsItem.Summary = formatSummary(contentHTML.String())
	return newsItem, nil
}

func formatSummary(summary string) string {
	re := regexp.MustCompile(`\s*\n\s*`)
	summary = re.ReplaceAllString(summary, " ")

	re = regexp.MustCompile(`\s\s+`)
	summary = re.ReplaceAllString(summary, " ")

	re = regexp.MustCompile(`<h[1-6][^>]*>`)
	summary = re.ReplaceAllString(summary, `<p class="news-paragraph">`)

	re = regexp.MustCompile(`</h[1-6]>`)
	summary = re.ReplaceAllString(summary, "</p>")

	re = regexp.MustCompile(`style="[^"]*"`)
	summary = re.ReplaceAllString(summary, "")

	re = regexp.MustCompile(`src="/"`)
	summary = re.ReplaceAllString(summary, `src="http://www.gomotionapp.com/"`)

	return summary
}

func generateHTML(newsItems []NewsItem) string {
	var b strings.Builder
	for _, item := range newsItems {
		fmt.Fprintf(&b, `
		<div class="news-item">
			<h2 class="news-title"><strong>%s</strong></h2>
			<p class="news-date">Author: %s</p>
			<p class="news-date">Published on %s</p>
			<div class="news-content">%s</div>
		</div>
		`, item.Title, item.Author, item.Date, item.Summary)
	}
	return b.String()
}

func updateHTMLFile(newsHTML string) error {
	content, err := os.ReadFile(newsFile)
	if err != nil {
		return fmt.Errorf("error reading HTML file: %v", err)
	}

	contentStr := string(content)
	startIdx := strings.Index(contentStr, startMarker) + len(startMarker)
	endIdx := strings.Index(contentStr, endMarker)
	if startIdx == -1 || endIdx == -1 {
		return fmt.Errorf("markers not found in HTML file")
	}

	updatedContent := contentStr[:startIdx] + "\n" + newsHTML + "\n" + contentStr[endIdx:]
	return os.WriteFile(newsFile, []byte(updatedContent), 0644)
}

func pushToGithub() error {
	token := os.Getenv("PAT_TOKEN")
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("error opening repository: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("error getting worktree: %v", err)
	}

	_, err = w.Add(newsFile)
	if err != nil {
		return fmt.Errorf("error staging files: %v", err)
	}

	_, err = w.Commit("automated commit: sync TeamUnify news articles [skip ci]", &git.CommitOptions{})
	if err != nil {
		return fmt.Errorf("error committing changes: %v", err)
	}

	err = repo.Push(&git.PushOptions{
		Auth: &gitHttp.BasicAuth{ // Renamed http to gitHttp
			Username: "git",
			Password: token,
		},
	})
	if err != nil {
		return fmt.Errorf("error pushing changes: %v", err)
	}
	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting update process...")

	if err := checkGithubToken(); err != nil {
		log.Fatalf("GitHub token validation failed: %v", err)
	}

	if err := cloneRepository(); err != nil {
		log.Fatalf("Repository cloning failed: %v", err)
	}

	newsItems, err := fetchNews()
	if err != nil {
		log.Fatalf("News fetching failed: %v", err)
	}

	if len(newsItems) == 0 {
		log.Fatal("No news items fetched")
	}

	newsHTML := generateHTML(newsItems)

	if err := updateHTMLFile(newsHTML); err != nil {
		log.Fatalf("HTML file update failed: %v", err)
	}

	if err := pushToGithub(); err != nil {
		log.Fatalf("GitHub push failed: %v", err)
	}

	log.Println("Update process completed successfully")
}
