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
    githttp "github.com/go-git/go-git/v5/plumbing/transport/http" // Alias for Git operations
    nethttp "net/http" // Alias for HTTP requests
    "main" // Import constants from constants.go
)

// NewsItem struct to store information about news articles
type NewsItem struct {
    Title       string
    URL         string
    Description string
    PublishedAt time.Time
}

// Fetches the latest news articles from a URL
func fetchNews() ([]NewsItem, error) {
    var newsItems []NewsItem
    client := &nethttp.Client{}
    req, err := nethttp.NewRequest("GET", "https://www.gomotionapp.com/team/cadas/page/news", nil)
    if err != nil {
        return nil, fmt.Errorf("error creating request: %v", err)
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error fetching news: %v", err)
    }
    defer resp.Body.Close()

    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error parsing news page: %v", err)
    }

    // Iterate over the news articles
    doc.Find(".news-item").Each(func(i int, s *goquery.Selection) {
        title := s.Find(".title").Text()
        url, _ := s.Find("a").Attr("href")
        description := s.Find(".description").Text()
        publishedAtStr := s.Find(".date").Text()

        publishedAt, err := time.Parse("January 2, 2006", publishedAtStr)
        if err != nil {
            publishedAt = time.Now()
        }

        newsItems = append(newsItems, NewsItem{
            Title:       title,
            URL:         url,
            Description: description,
            PublishedAt: publishedAt,
        })
    })

    return newsItems, nil
}

// Clone the Git repository to the local machine
func cloneRepository() error {
    token := os.Getenv("PAT_TOKEN")
    currentDir, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("error getting current directory: %v", err)
    }

    repoPath := filepath.Join(currentDir, main.repoName)
    if _, err := os.Stat(repoPath); os.IsNotExist(err) {
        // Repository doesn't exist, so clone it
        fmt.Println("Cloning repository...")
        _, err := git.PlainClone(repoPath, false, &git.CloneOptions{
            URL:           main.githubRepo,
            ReferenceName: "refs/heads/main",
            Auth: &githttp.BasicAuth{
                Username: "token",
                Password: token,
            },
        })
        if err != nil {
            return fmt.Errorf("error cloning repository: %v", err)
        }
    } else {
        // Repository exists, pull the latest changes
        fmt.Println("Repository already exists, pulling latest changes...")
        repo, err := git.PlainOpen(repoPath)
        if err != nil {
            return fmt.Errorf("error opening repository: %v", err)
        }
        worktree, err := repo.Worktree()
        if err != nil {
            return fmt.Errorf("error getting worktree: %v", err)
        }
        err = worktree.Pull(&git.PullOptions{
            RemoteName: "origin",
            Auth: &githttp.BasicAuth{
                Username: "token",
                Password: token,
            },
        })
        if err != nil && err.Error() != "already up-to-date" {
            return fmt.Errorf("error pulling repository: %v", err)
        }
    }

    return nil
}

// Update the news content in the repository
func updateNewsContent(newsItems []NewsItem) error {
    currentDir, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("error getting current directory: %v", err)
    }

    repoPath := filepath.Join(currentDir, main.repoName)
    filePath := filepath.Join(repoPath, "news.html")

    // Open the file and read its content
    fileContent, err := os.ReadFile(filePath)
    if err != nil {
        return fmt.Errorf("error reading file: %v", err)
    }

    // Find the start and end markers for the news section
    startIndex := bytes.Index(fileContent, []byte(main.startMarker))
    endIndex := bytes.Index(fileContent, []byte(main.endMarker))

    if startIndex == -1 || endIndex == -1 {
        return fmt.Errorf("start or end marker not found in file")
    }

    // Remove the current news section between the markers
    contentToReplace := fileContent[startIndex:endIndex+len(main.endMarker)]
    updatedContent := bytes.Replace(fileContent, contentToReplace, []byte{}, 1)

    // Create the new news section
    var newsSection bytes.Buffer
    newsSection.WriteString(fmt.Sprintf("<!-- START UNDER HERE -->\n"))
    for _, news := range newsItems {
        newsSection.WriteString(fmt.Sprintf("<div class=\"news-item\">\n"))
        newsSection.WriteString(fmt.Sprintf("<h2><a href=\"%s\">%s</a></h2>\n", news.URL, news.Title))
        newsSection.WriteString(fmt.Sprintf("<p>%s</p>\n", news.Description))
        newsSection.WriteString(fmt.Sprintf("<small>Published on: %s</small>\n", news.PublishedAt.Format("January 2, 2006")))
        newsSection.WriteString(fmt.Sprintf("</div>\n"))
    }
    newsSection.WriteString(fmt.Sprintf("<!-- END AUTOMATION SCRIPT -->\n"))

    // Insert the new news section back into the content
    updatedContent = append(updatedContent[:startIndex+len(main.startMarker)], append(newsSection.Bytes(), updatedContent[endIndex:]...)...)

    // Write the updated content back to the file
    err = os.WriteFile(filePath, updatedContent, 0644)
    if err != nil {
        return fmt.Errorf("error writing updated content to file: %v", err)
    }

    return nil
}

// Run the process to fetch, clone, and update the news content
func syncNews() error {
    newsItems, err := fetchNews()
    if err != nil {
        return fmt.Errorf("error fetching news: %v", err)
    }

    err = cloneRepository()
    if err != nil {
        return fmt.Errorf("error cloning repository: %v", err)
    }

    err = updateNewsContent(newsItems)
    if err != nil {
        return fmt.Errorf("error updating news content: %v", err)
    }

    return nil
}

func main() {
    err := syncNews()
    if err != nil {
        log.Fatalf("Error syncing news: %v", err)
    }
    log.Println("News synchronization completed successfully!")
}
