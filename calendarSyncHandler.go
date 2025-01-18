import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/apognu/gocal"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)
const (
	githubRepo    = "https://github.com/dareaquatics/dare-website"
	icsURL        = "https://www.gomotionapp.com/rest/ics/system/5/Events.ics?key=l4eIgFXwqEbxbQz42YjRgg%3D%3D&enabled=false&tz=America%2FLos_Angeles"
	repoName      = "dare-website"
	eventsFile    = "calendar.html"
	timezone      = "America/Los_Angeles"
	startMarker   = "<!-- START UNDER HERE -->"
	endMarker     = "<!-- END AUTOMATION SCRIPT -->"
)
type Event struct {
	Title       string    `json:"title"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
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
		Auth: &http.BasicAuth{
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
func fetchEvents() ([]Event, error) {
	resp, err := http.Get(icsURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching ICS: %v", err)
	}
	defer resp.Body.Close()
	parser := gocal.NewParser(resp.Body)
	parser.Start = time.Now().AddDate(-1, 0, 0) // Get events from last year
	parser.End = time.Now().AddDate(1, 0, 0)    // Get events until next year
	err = parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing ICS: %v", err)
	}
	var events []Event
	for _, e := range parser.Events {
		events = append(events, Event{
			Title:       e.Summary,
			Start:      e.Start,
			End:        e.End,
			Description: e.Description,
			URL:        "#",
		})
	}
	return events, nil
}
func generateHTML(events []Event) (string, error) {
	now := time.Now()
	var upcomingEvents, pastEvents strings.Builder
	for _, event := range events {
		eventHTML := fmt.Sprintf(`
        <div class="event">
          <h2><strong>%s</strong></h2>
          <p><b>Event Start:</b> %s</p>
          <p><b>Event End:</b> %s</p>
          <br>
          <p>Click the button below for more information.</p>
          <a href="https://www.gomotionapp.com/team/cadas/controller/cms/admin/index?team=cadas#/calendar-team-events" target="_blank" rel="noopener noreferrer" class="btn btn-primary">More Details</a>
        </div>
        <br><br>
        `, event.Title, event.Start.Format("January 02, 2006"), event.End.Format("January 02, 2006"))
		if event.End.Before(now) {
			pastEvents.WriteString(eventHTML)
		} else {
			upcomingEvents.WriteString(eventHTML)
		}
	}
	var finalHTML strings.Builder
	finalHTML.WriteString(upcomingEvents.String())
	if pastEvents.Len() > 0 {
		finalHTML.WriteString(`
        <button type="button" class="collapsible">Click for Past Events</button>
        <div class="content" style="display: none;">
          ` + pastEvents.String() + `
        </div>
        <br>
        <script>
        var coll = document.getElementsByClassName("collapsible");
        for (var i = 0; i < coll.length; i++) {
          coll[i].addEventListener("click", function() {
            this.classList.toggle("active");
            var content = this.nextElementSibling;
            if (content.style.display === "block") {
              content.style.display = "none";
            } else {
              content.style.display = "block";
            }
          });
        }
        </script>
        `)
	}
	return finalHTML.String(), nil
}
func updateHTMLFile(eventHTML string) error {
	content, err := os.ReadFile(eventsFile)
	if err != nil {
		return fmt.Errorf("error reading HTML file: %v", err)
	}
	contentStr := string(content)
	startIdx := strings.Index(contentStr, startMarker) + len(startMarker)
	endIdx := strings.Index(contentStr, endMarker)
	if startIdx == -1 || endIdx == -1 {
		return fmt.Errorf("markers not found in HTML file")
	}
	updatedContent := contentStr[:startIdx] + "\n" + eventHTML + "\n" + contentStr[endIdx:]
	return os.WriteFile(eventsFile, []byte(updatedContent), 0644)
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
	_, err = w.Add(eventsFile)
	if err != nil {
		return fmt.Errorf("error staging files: %v", err)
	}
	_, err = w.Commit("automated commit: sync TeamUnify calendar [skip ci]", &git.CommitOptions{})
	if err != nil {
		return fmt.Errorf("error committing changes: %v", err)
	}
	err = repo.Push(&git.PushOptions{
		Auth: &http.BasicAuth{
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
	events, err := fetchEvents()
	if err != nil {
		log.Fatalf("Event fetching failed: %v", err)
	}
	if len(events) == 0 {
		log.Fatal("No events fetched")
	}
	eventHTML, err := generateHTML(events)
	if err != nil {
		log.Fatalf("HTML generation failed: %v", err)
	}
	if err := updateHTMLFile(eventHTML); err != nil {
		log.Fatalf("HTML file update failed: %v", err)
	}
	if err := pushToGithub(); err != nil {
		log.Fatalf("GitHub push failed: %v", err)
	}
	log.Println("Update process completed successfully")
}
