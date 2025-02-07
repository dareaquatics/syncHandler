# autosync for calendar (calendar.html)

import os
import shutil
import platform
from ics import Calendar
from git import Repo, GitCommandError
import logging
import colorlog
import requests
from tqdm import tqdm
from datetime import datetime
import pytz

# Constants
GITHUB_REPO = 'https://github.com/dareaquatics/dare-website'
ICS_URL = 'https://www.gomotionapp.com/rest/ics/system/5/Events.ics?key=l4eIgFXwqEbxbQz42YjRgg%3D%3D&enabled=false&tz=America%2FLos_Angeles'
GITHUB_TOKEN = os.getenv('PAT_TOKEN')
REPO_NAME = 'dare-website'
EVENTS_HTML_FILE = 'calendar.html'
TIMEZONE = 'America/Los_Angeles'

# Setup colored logging
handler = colorlog.StreamHandler()
handler.setFormatter(colorlog.ColoredFormatter(
    '%(log_color)s%(asctime)s - %(levelname)s - %(message)s',
    log_colors={
        'DEBUG': 'cyan',
        'INFO': 'green',
        'WARNING': 'red',
        'ERROR': 'bold_red',
        'CRITICAL': 'bold_red',
    }
))
logging.basicConfig(level=logging.DEBUG, handlers=[handler])

def check_git_installed():
    git_path = shutil.which("git")
    if git_path:
        logging.info(f"Git found at {git_path}")
        return True
    else:
        logging.warning("Git not found. Attempting to download portable Git.")
        return False

def download_portable_git():
    os_name = platform.system().lower()
    git_filename = None
    git_url = None

    if os_name == 'windows':
        git_url = 'https://github.com/git-for-windows/git/releases/download/v2.45.1.windows.1/PortableGit-2.45.1-64-bit.7z.exe'
        git_filename = 'PortableGit-2.45.1-64-bit.7z.exe'
    elif os_name == 'linux':
        git_url = 'https://github.com/git/git/archive/refs/tags/v2.40.0.tar.gz'
        git_filename = 'git-2.40.0.tar.gz'
    elif os_name == 'darwin':
        git_url = 'https://sourceforge.net/projects/git-osx-installer/files/git-2.40.0-intel-universal-mavericks.dmg/download'
        git_filename = 'git-2.40.0-intel-universal-mavericks.dmg'
    else:
        logging.error(f"Unsupported OS: {os_name}")
        return False

    if not git_url or not git_filename:
        logging.error("Invalid Git download URL or filename.")
        return False

    try:
        response = requests.get(git_url, stream=True)
        if response.status_code == 200:
            with open(git_filename, 'wb') as file:
                for chunk in tqdm(response.iter_content(chunk_size=8192), desc='Downloading Git', unit='B', unit_scale=True, unit_divisor=1024):
                    file.write(chunk)
            logging.info(f"Downloaded Git: {git_filename}")
            return True
        else:
            logging.error(f"Failed to download Git. HTTP Status: {response.status_code}")
            return False
    except Exception as e:
        logging.error(f"Error downloading Git: {e}")
        return False

def is_repo_up_to_date(repo_path):
    try:
        if not os.path.exists(repo_path):
            logging.error(f"Repository path does not exist: {repo_path}")
            return False
        repo = Repo(repo_path)
        origin = repo.remotes.origin
        origin.fetch()  # Fetch latest commits

        local_commit = repo.head.commit
        remote_commit = repo.commit('origin/main')

        if local_commit.hexsha == remote_commit.hexsha:
            logging.info("Local repository is up-to-date.")
            return True
        else:
            logging.info("Local repository is not up-to-date.")
            return False
    except GitCommandError as e:
        logging.error(f"Git command error: {e}")
        return False
    except Exception as e:
        logging.error(f"Error checking repository status: {e}")
        return False

def delete_and_reclone_repo(repo_path):
    try:
        if os.path.exists(repo_path):
            for root, dirs, files in os.walk(repo_path):
                for dir in dirs:
                    os.chmod(os.path.join(root, dir), 0o777)
                for file in files:
                    os.chmod(os.path.join(root, file), 0o777)
            shutil.rmtree(repo_path)
            logging.info(f"Deleted existing repository at {repo_path}")
    except PermissionError as e:
        logging.error(f"Permission error deleting repository: {e}")
        return
    except FileNotFoundError as e:
        logging.error(f"File not found error deleting repository: {e}")
        return
    except Exception as e:
        logging.error(f"Error deleting repository: {e}")
        return

    clone_repository()

def clone_repository():
    try:
        current_dir = os.getcwd()
        repo_path = os.path.join(current_dir, REPO_NAME)
        if not os.path.exists(repo_path):
            with tqdm(total=100, desc='Cloning repository') as pbar:
                def update_pbar(op_code, cur_count, max_count=None, message=''):
                    if max_count:
                        pbar.total = max_count
                    pbar.update(cur_count - pbar.n)
                    pbar.set_postfix_str(message)

                Repo.clone_from(GITHUB_REPO, repo_path, progress=update_pbar)
            logging.info(f"Repository cloned to {repo_path}")
        else:
            if not is_repo_up_to_date(repo_path):
                delete_and_reclone_repo(repo_path)
            else:
                logging.info(f"Repository already exists at {repo_path}")
        os.chdir(repo_path)
        logging.info(f"Changed working directory to {repo_path}")
    except GitCommandError as e:
        logging.error(f"Git command error: {e}")
    except Exception as e:
        logging.error(f"Error cloning repository: {e}")

def check_github_token_validity():
    try:
        headers = {
            'Authorization': f'token {GITHUB_TOKEN}'
        }
        repo_path = GITHUB_REPO.replace("https://github.com/", "")
        api_url = f'https://api.github.com/repos/{repo_path}'
        response = requests.get(api_url, headers=headers)
        if response.status_code == 200:
            logging.info("GitHub token is valid.")
        else:
            logging.error("Invalid GitHub token.")
            exit(1)
    except Exception as e:
        logging.error(f"Error validating GitHub token: {e}")
        exit(1)

def fetch_events():
    try:
        logging.info("Fetching events from .ics file...")
        response = requests.get(ICS_URL)
        response.raise_for_status()

        calendar = Calendar(response.text)

        event_items = []

        for event in calendar.events:
            event_items.append({
                'title': event.name,
                'start': event.begin,
                'end': event.end,
                'description': event.description if event.description else '',
                'url': event.url if event.url else '#'
            })

        # Sort events by start date
        event_items.sort(key=lambda x: x['start'])
        logging.info("Successfully fetched and sorted event items.")
        return event_items

    except requests.exceptions.RequestException as e:
        logging.error(f"Error fetching events: {e}")
        return []

def generate_html(event_items):
    logging.info("Generating HTML for event items...")
    current_date = datetime.now(pytz.timezone(TIMEZONE))  # Convert current_date to timezone-aware datetime
    upcoming_events_html = ''
    past_events_html = ''

    for item in event_items:
        event_html = f'''
        <div class="event">
          <h2><strong>{item["title"]}</strong></h2>
          <p><b>Event Start:</b> {item["start"].strftime('%B %d, %Y')}</p>
          <p><b>Event End:</b> {item["end"].strftime('%B %d, %Y')}</p>
          <br>
          <p>Click the button below for more information.</p>
          <a href="https://www.gomotionapp.com/team/cadas/controller/cms/admin/index?team=cadas#/calendar-team-events" target="_blank" rel="noopener noreferrer" class="btn btn-primary">More Details</a>
        </div>
        <br><br>
        '''

        # Categorize based on event end date
        if item['end'] < current_date:
            past_events_html += event_html
        else:
            upcoming_events_html += event_html

    # Create collapsible section for past events
    if past_events_html:
        past_events_html = f'''
        <button type="button" class="collapsible">Click for Past Events</button>
        <div class="content" style="display: none;">
          {past_events_html}
        </div>
        <br>
        <script>
        var coll = document.getElementsByClassName("collapsible");
        for (var i = 0; i < coll.length; i++) {{
          coll[i].addEventListener("click", function() {{
            this.classList.toggle("active");
            var content = this.nextElementSibling;
            if (content.style.display === "block") {{
              content.style.display = "none";
            }} else {{
              content.style.display = "block";
            }}
          }});
        }}
        </script>
        '''
        
    html_content = upcoming_events_html + past_events_html
    logging.info("Successfully generated HTML.")
    return html_content

def update_html_file(event_html):
    try:
        if not os.path.exists(EVENTS_HTML_FILE):
            logging.error(f"HTML file '{EVENTS_HTML_FILE}' not found in the repository.")
            return

        logging.info("Updating HTML file...")
        with open(EVENTS_HTML_FILE, 'r', encoding='utf-8') as file:
            content = file.read()

        start_marker = '<!-- START UNDER HERE -->'
        end_marker = '<!-- END AUTOMATION SCRIPT -->'
        start_index = content.find(start_marker) + len(start_marker)
        end_index = content.find(end_marker)

        if start_index == -1 or end_index == -1:
            logging.error("Markers not found in the HTML file.")
            return

        updated_content = content[:start_index] + '\n' + event_html + '\n' + content[end_index:]

        with open(EVENTS_HTML_FILE, 'w', encoding='utf-8') as file:
            file.write(updated_content)
        logging.info("Successfully updated HTML file.")

    except IOError as e:
        logging.error(f"Error updating HTML file: {e}")

def push_to_github():
    try:
        logging.info("Pushing changes to GitHub...")
        repo = Repo(os.getcwd())
        origin = repo.remote(name='origin')
        origin.set_url(f'https://{GITHUB_TOKEN}@github.com/dareaquatics/dare-website.git')

        if repo.is_dirty(untracked_files=True):
            with tqdm(total=100, desc='Committing changes') as pbar:
                def update_commit_pbar(cur_count, max_count=None, message=''):
                    if max_count:
                        pbar.total = max_count
                    pbar.update(cur_count - pbar.n)
                    pbar.set_postfix_str(message)

                repo.git.add(EVENTS_HTML_FILE)
                repo.index.commit('automated commit: sync TeamUnify calendar [skip ci]')
                pbar.update(100)

            with tqdm(total=100, desc='Pushing changes') as pbar:
                def update_push_pbar(op_code, cur_count, max_count=None, message=''):
                    if max_count:
                        pbar.total = max_count
                    pbar.update(cur_count - pbar.n)
                    pbar.set_postfix_str(message)

                origin.push(progress=update_push_pbar)
            logging.info("Successfully pushed changes to GitHub.")
        else:
            logging.info("No changes to commit.")

    except GitCommandError as e:
        logging.error(f"Git command error: {e}")
    except Exception as e:
        logging.error(f"Error pushing changes to GitHub: {e}")

def main():
    try:
        logging.info("Starting update process...")

        check_github_token_validity()

        if not check_git_installed():
            if not download_portable_git():
                logging.error("Unable to install Git. Aborting process.")
                return

        clone_repository()

        event_items = fetch_events()

        if not event_items:
            logging.error("No event items fetched. Aborting update process.")
            return

        event_html = generate_html(event_items)

        update_html_file(event_html)

        push_to_github()

        logging.info("Update process completed.")
    except Exception as e:
        logging.error(f"Update process failed: {e}")
        logging.info("Update process aborted due to errors.")

if __name__ == "__main__":
    main()
