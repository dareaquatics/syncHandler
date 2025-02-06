package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	gitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

const (
	newsURL      = "https://www.gomotionapp.com/team/cadas/page/news"
	baseURL      = "https://www.gomotionapp.com"
	newsHTMLFile = "news.html"
	startMarker  = "<!-- START UNDER HERE -->"
	endMarker    = "<!-- END AUTOMATION SCRIPT -->"
	timeFormat   = "January 2, 2006"
	concurrency  = 5
)

var (
	client = &http.Client{
		Timeout: 30 * time.Second,
	}
	log = logrus.New()
)

type Article struct {
	Title   string
	Date    string
	Author  string
	Content string
	URL     string
}

func main() {
	setupLogger()
	log.Info("starting news sync process")

	if os.Getenv("PAT_TOKEN") == "" {
		log.Fatal("missing PAT_TOKEN environment variable")
	}

	articleURLs, err := fetchArticleURLs()
	if err != nil {
		log.Fatalf("failed to fetch article urls: %v", err)
	}

	articles := processArticles(articleURLs)
	if len(articles) == 0 {
		log.Info("no articles found")
		return
	}

	htmlContent := generateHTML(articles)
	modified, err := updateNewsHTML(htmlContent)
	if err != nil {
		log.Fatalf("failed to update html: %v", err)
	}

	if modified {
		if err := gitCommitAndPush(); err != nil {
			log.Fatalf("failed to commit changes: %v", err)
		}
	}

	log.Info("sync process completed successfully")
}

func setupLogger() {
	log.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
	log.SetLevel(logrus.InfoLevel)
}

func fetchArticleURLs() ([]string, error) {
	log.Info("fetching main news page")
	req, err := http.NewRequest("GET", newsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}

	setBrowserHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("html parsing failed: %w", err)
	}

	var urls []string
	doc.Find("div.Item:not(.Supplement) a[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			urls = append(urls, baseURL+href)
		}
	})

	log.Infof("found %d articles", len(urls))
	return urls, nil
}

func processArticles(urls []string) []Article {
	var wg sync.WaitGroup
	ch := make(chan string, concurrency)
	results := make(chan Article, len(urls))

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range ch {
				article, err := fetchArticle(url)
				if err != nil {
					log.Warnf("failed to process %s: %v", url, err)
					continue
				}
				results <- article
			}
		}()
	}

	for _, url := range urls {
		ch <- url
	}
	close(ch)
	wg.Wait()
	close(results)

	var articles []Article
	for article := range results {
		articles = append(articles, article)
	}

	sortArticlesByDate(articles)
	return articles
}

func fetchArticle(articleURL string) (Article, error) {
	req, err := http.NewRequest("GET", articleURL, nil)
	if err != nil {
		return Article{}, fmt.Errorf("request creation failed: %w", err)
	}

	setBrowserHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return Article{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Article{}, fmt.Errorf("html parsing failed: %w", err)
	}

	newsItem := doc.Find("div.NewsItem")
	if newsItem.Length() == 0 {
		return Article{}, fmt.Errorf("news item not found")
	}

	title := newsItem.Find("h1").Text()
	dateStr, _ := newsItem.Find("span.DateStr").Attr("data")
	author := newsItem.Find("div.Author strong").Text()
	content, _ := newsItem.Find("div.Content").Html()

	return Article{
		Title:   strings.TrimSpace(title),
		Date:    formatDate(dateStr),
		Author:  strings.TrimSpace(author),
		Content: processContent(content),
		URL:     articleURL,
	}, nil
}

func formatDate(timestamp string) string {
	if timestamp == "" {
		return "Unknown Date"
	}

	t, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return t.Format(timeFormat)
	}
	return "Unknown Date"
}

func processContent(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html
	}

	// Process images
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src != "" && !strings.HasPrefix(src, "http") {
			src = baseURL + src
		}
		s.ReplaceWithHtml(fmt.Sprintf(`<a href="%s" target="_blank">Click to see image</a>`, src))
	})

	// Flatten headings
	doc.Find("h1,h2,h3,h4,h5,h6").Each(func(i int, s *goquery.Selection) {
		s.SetHtml(fmt.Sprintf(`<p class="news-paragraph">%s</p>`, s.Text()))
	})

	// Clean up links
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		s.SetText("Click here to be redirected to the link")
		if href != "" && !strings.HasPrefix(href, "http") {
			href = baseURL + href
		}
		s.SetAttr("href", href)
		s.SetAttr("target", "_blank")
	})

	// Clean up HTML
	html, _ = doc.Html()
	html = regexp.MustCompile(`\s+`).ReplaceAllString(html, " ")
	html = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`</li>\s*<li>`).ReplaceAllString(html, "</li><li>")

	return html
}

func sortArticlesByDate(articles []Article) {
	sort.Slice(articles, func(i, j int) bool {
		t1, _ := time.Parse(timeFormat, articles[i].Date)
		t2, _ := time.Parse(timeFormat, articles[j].Date)
		return t1.After(t2)
	})
}

func generateHTML(articles []Article) string {
	var sb strings.Builder
	sb.WriteString("\n")

	for _, article := range articles {
		sb.WriteString(fmt.Sprintf(`
		<div class="news-item">
			<h2 class="news-title"><strong>%s</strong></h2>
			<p class="news-date">Author: %s</p>
			<p class="news-date">Published on %s</p>
			<div class="news-content">%s</div>
		</div>
		`, article.Title, article.Author, article.Date, article.Content))
	}

	return sb.String()
}

func updateNewsHTML(newContent string) (bool, error) {
	file, err := os.OpenFile(newsHTMLFile, os.O_RDWR, 0644)
	if err != nil {
		return false, fmt.Errorf("file open failed: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return false, fmt.Errorf("file read failed: %w", err)
	}

	html := string(content)
	startIdx := strings.Index(html, startMarker) + len(startMarker)
	endIdx := strings.Index(html, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return false, fmt.Errorf("markers not found in html")
	}

	updated := html[:startIdx] + newContent + html[endIdx:]
	if updated == html {
		log.Info("no changes detected")
		return false, nil
	}

	if err := file.Truncate(0); err != nil {
		return false, fmt.Errorf("file truncate failed: %w", err)
	}

	if _, err := file.Seek(0, 0); err != nil {
		return false, fmt.Errorf("file seek failed: %w", err)
	}

	if _, err := file.WriteString(updated); err != nil {
		return false, fmt.Errorf("file write failed: %w", err)
	}

	log.Info("html file updated successfully")
	return true, nil
}

func gitCommitAndPush() error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("repo open failed: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree access failed: %w", err)
	}

	if _, err := wt.Add(newsHTMLFile); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	_, err = wt.Commit("automated commit: sync TeamUnify news articles [skip ci]", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "github-actions[bot]",
			Email: "github-actions[bot]@users.noreply.github.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	auth := &gitHttp.BasicAuth{
		Username: "github-actions",
		Password: os.Getenv("PAT_TOKEN"),
	}

	if err := repo.Push(&git.PushOptions{Auth: auth}); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	return nil
}

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Referer", baseURL)
}
