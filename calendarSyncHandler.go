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
	log.Info("Starting calendar sync process")

	if os.Getenv("PAT_TOKEN") == "" {
		log.Fatal("Missing PAT_TOKEN environment variable")
	}

	events, err := fetchEvents(log)
	if err != nil {
		log.Fatalf("Failed to fetch events: %v", err)
	}

	htmlContent := generateHTML(events, log)
	modified, err := updateHTMLContent(htmlContent, log)
	if err != nil {
		log.Fatalf("Failed to update HTML: %v", err)
	}

	if modified {
		if err := gitCommitAndPush(log); err != nil {
			log.Fatalf("Failed to commit changes: %v", err)
		}
	}

	log.Info("Sync process completed successfully")
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
	log.Info("Fetching ICS data")
	resp, err := http.Get(icsURL)
	if err != nil {
		return nil, fmt.Errorf("ICS fetch failed: %w", err)
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
		return nil, fmt.Errorf("ICS parse failed: %w", err)
	}

	for i := range parser.Events {
		parser.Events[i].Start = parser.Events[i].Start.In(loc)
		parser.Events[i].End = parser.Events[i].End.In(loc)
	}

	sort.Slice(parser.Events, func(i, j int) bool {
		return parser.Events[i].Start.Before(parser.Events[j].Start)
	})

	log.Infof("Processed %d events", len(parser.Events))
	return parser.Events, nil
}

func generateHTML(events []gocal.Event, log *logrus.Logger) string {
	log.Info("Generating HTML content")
	var upcoming, past strings.Builder
	now := time.Now().In(time.UTC)

	for _, event := range events {
		html := fmt.Sprintf(`
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
		)

		if event.End.Before(now) {
			past.WriteString(html)
		} else {
			upcoming.WriteString(html)
		}
	}

	var content strings.Builder
	content.WriteString(upcoming.String())

	if past.Len() > 0 {
		content.WriteString(`
		<button type="button" class="collapsible">Click for Past Events</button>
		<div class="content" style="display: none;">`)
		content.WriteString(past.String())
		content.WriteString(`
		</div>
		<br>
		<script>
		  document.querySelectorAll('.collapsible').forEach(button => {
		    button.addEventListener('click', () => {
		      const content = button.nextElementSibling;
		      content.style.display = content.style.display === 'block' ? 'none' : 'block';
		    });
		  });
		</script>`)
	}

	return content.String()
}

func updateHTMLContent(newContent string, log *logrus.Logger) (bool, error) {
	log.Info("Updating HTML file")
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
		return false, fmt.Errorf("markers not found in HTML")
	}

	updated := html[:startIdx] + "\n" + newContent + "\n" + html[endIdx:]
	if updated == html {
		log.Info("No changes detected")
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

	log.Info("HTML file updated successfully")
	return true, nil
}

func gitCommitAndPush(log *logrus.Logger) error {
	log.Info("Committing changes to Git")
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

	log.Info("Changes pushed successfully")
	return nil
}
