package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/apognu/gocal"
	"github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	gitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

const (
	icsURL        = "https://www.gomotionapp.com/rest/ics/system/5/Events.ics?key=l4eIgFXwqEbxbQz42YjRgg%3D%3D&enabled=false&tz=America%2FLos_Angeles"
	timezone      = "America/Los_Angeles"
	eventsHTML    = "calendar.html"
	startMarker   = "<!-- START UNDER HERE -->"
	endMarker     = "<!-- END AUTOMATION SCRIPT -->"
	commitMessage = "automated commit: sync TeamUnify calendar [skip ci]"
)

func main() {
	log := setupLogger()
	log.Info("starting calendar sync process")

	if os.Getenv("PAT_TOKEN") == "" {
		log.Fatal("missing PAT_TOKEN environment variable")
	}

	events, err := fetchEvents(log)
	if err != nil {
		log.Fatalf("failed to fetch events: %v", err)
	}

	htmlContent := generateHTML(events, log)
	modified, err := updateHTMLContent(htmlContent, log)
	if err != nil {
		log.Fatalf("failed to update html: %v", err)
	}

	if modified {
		if err := gitCommitAndPush(log); err != nil {
			log.Fatalf("failed to commit changes: %v", err)
		}
	}

	log.Info("sync process completed successfully")
}

func setupLogger() *logrus.Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
	log.SetLevel(logrus.InfoLevel)
	return log
}

func fetchEvents(log *logrus.Logger) ([]gocal.Event, error) {
	log.Info("fetching ics data")
	resp, err := http.Get(icsURL)
	if err != nil {
		return nil, fmt.Errorf("ics fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("timezone load failed: %w", err)
	}

	parser := gocal.NewParser(resp.Body)
	if err := parser.Parse(); err != nil {
		return nil, fmt.Errorf("ics parse failed: %w", err)
	}

	for i := range parser.Events {
		start := parser.Events[i].Start.In(loc)
		end := parser.Events[i].End.In(loc)
		parser.Events[i].Start = &start
		parser.Events[i].End = &end
	}

	sort.Slice(parser.Events, func(i, j int) bool {
		return parser.Events[i].Start.Before(*parser.Events[j].Start)
	})

	log.Infof("processed %d events", len(parser.Events))
	return parser.Events, nil
}

func generateHTML(events []gocal.Event, log *logrus.Logger) string {
	log.Info("generating html content")
	
	if len(events) == 0 {
		return `<div class="event"><p>No upcoming events published.</p></div>`
	}

	var content strings.Builder
	now := time.Now().In(time.UTC)
	hasUpcoming := false

	for _, event := range events {
		// Skip past events
		if event.End.Before(now) {
			continue
		}

		hasUpcoming = true
		content.WriteString(fmt.Sprintf(`
		<div class="event">
		  <h2><strong>%s</strong></h2>
		  <p><b>Event Start:</b> %s</p>
		  <p><b>Event End:</b> %s</p>
		  <br>
		  <p>Click the button below for more information.</p>
		  <a href="https://www.gomotionapp.com/team/cadas/controller/cms/admin/index?team=cadas#/calendar-team-events" 
		     target="_blank" 
		     rel="noopener noreferrer" 
		     class="btn btn-primary">
		    More Details
		  </a>
		</div>
		<br><br>`,
			event.Summary,
			event.Start.Format("January 02, 2006"),
			event.End.Format("January 02, 2006"),
		))
	}

	if !hasUpcoming {
		content.WriteString(`<div class="event"><p>No upcoming events published.</p></div>`)
	}

	return content.String()
}

func updateHTMLContent(newContent string, log *logrus.Logger) (bool, error) {
	log.Info("updating html file")
	file, err := os.OpenFile(eventsHTML, os.O_RDWR, 0644)
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

	updated := html[:startIdx] + "\n" + newContent + "\n" + html[endIdx:]
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

func gitCommitAndPush(log *logrus.Logger) error {
	log.Info("committing changes to git")
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("repo open failed: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree access failed: %w", err)
	}

	if _, err := wt.Add(eventsHTML); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	_, err = wt.Commit(commitMessage, &git.CommitOptions{
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

	log.Info("changes pushed successfully")
	return nil
}
