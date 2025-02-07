# autosync for news events (news.html)

import os
import shutil
import platform
import cloudscraper
from bs4 import BeautifulSoup
from datetime import datetime
from git import Repo, GitCommandError
import logging
import colorlog
import re
import requests
from tqdm import tqdm

# Constants
GITHUB_REPO = 'https://github.com/dareaquatics/dare-website'
NEWS_URL = 'https://www.gomotionapp.com/team/cadas/page/news'
REPO_NAME = 'dare-website'
NEWS_HTML_FILE = 'news.html'

# GitHub Token is expected to be in environment variable 'PAT_TOKEN'
GITHUB_TOKEN = os.getenv('PAT_TOKEN')

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


def fetch_news():
    try:
        logging.info("Fetching news from TeamUnify using bypass...")
        scraper = cloudscraper.create_scraper()
        response = scraper.get(NEWS_URL)
        response.raise_for_status()

        logging.debug(f"Fetched HTML content: {response.text[:2000]}")

        soup = BeautifulSoup(response.content, 'html.parser')

        news_items = []

        articles = soup.find_all('div', class_='Item')
        logging.debug(f"Found {len(articles)} articles in total")

        for article in articles:
            if 'Supplement' in article.get('class', []):
                logging.debug("Skipping Supplement item")
                continue

            try:
                link_element = article.find('a', href=True)
                article_url = f"https://www.gomotionapp.com{link_element['href']}" if link_element else None

                if article_url:
                    article_details = fetch_article_content(article_url)
                    if article_details:
                        news_items.append(article_details)
                else:
                    logging.warning(f"Article URL not found for article with title: {title}")

            except Exception as e:
                logging.error(f"Error processing article: {e}")

        news_items.sort(
            key=lambda x: datetime.strptime(x['date'], '%B %d, %Y') if x['date'] != 'Unknown Date' else datetime.min, reverse=True)

        logging.info("Successfully fetched and parsed news items.")
        return news_items

    except requests.exceptions.RequestException as e:
        logging.error(f"Error fetching news: {e}")
        return []


def fetch_article_content(url):
    try:
        scraper = cloudscraper.create_scraper()
        response = scraper.get(url)
        response.raise_for_status()

        soup = BeautifulSoup(response.content, 'html.parser')
        news_item = soup.find('div', class_='NewsItem')
        if not news_item:
            logging.warning(f"NewsItem not found for article URL: {url}")
            return None

        title_element = news_item.find('h1')
        date_element = news_item.find('span', class_='DateStr')
        author_element = news_item.find('div', class_='Author').find('strong')
        content_div = news_item.find('div', class_='Content')

        title = title_element.get_text(strip=True) if title_element else 'No Title'
        date_str = date_element.get('data') if date_element else None
        author = author_element.get_text(strip=True) if author_element else 'Unknown Author'

        if date_str:
            date_obj = datetime.utcfromtimestamp(int(date_str) / 1000)
            formatted_date = date_obj.strftime('%B %d, %Y')
        else:
            logging.warning(f"Date not found for article at URL: {url}")
            formatted_date = 'Unknown Date'

        if content_div:
            # Extract the content and handle images
            content_html = ''
            for element in content_div:
                if element.name == 'img':
                    src = element["src"]
                    if not src.startswith("http"):
                        src = f"http://www.gomotionapp.com{src}"
                    content_html += f'<a href="{src}" target="_blank"><img src="{src}" style="max-width:100%; height:auto;" alt="Image"/></a>'
                elif element.name and element.name.startswith('h'):
                    # Flatten all heading tags to p tags with the same class for uniform size
                    content_html += f'<p class="news-paragraph">{element.get_text(strip=True)}</p>'
                elif element.name == 'a':
                    href = element.get('href')
                    text = element.get_text(strip=True)
                    content_html += f'<a href="{href}" target="_blank">{text}</a>'
                else:
                    # Clean the element from unwanted attributes
                    element.attrs = {}
                    content_html += str(element)
        else:
            logging.warning(f"Content not found for article URL: {url}")
            content_html = "Content not available."

        return {
            'title': title,
            'date': formatted_date,
            'summary': remove_duplicate_links(content_html),
            'author': author
        }

    except requests.exceptions.RequestException as e:
        logging.error(f"Error fetching article content from {url}: {e}")
        return {
            'title': 'Error fetching title',
            'date': 'Unknown Date',
            'summary': 'Error fetching content.',
            'author': 'Unknown Author'
        }
    except Exception as e:
        logging.error(f"Unexpected error fetching article content from {url}: {e}")
        return {
            'title': 'Error fetching title',
            'date': 'Unknown Date',
            'summary': 'Unexpected error fetching content.',
            'author': 'Unknown Author'
        }


def remove_duplicate_links(text):
    soup = BeautifulSoup(text, 'html.parser')
    links = set()
    for a in soup.find_all('a', href=True):
        if a['href'] in links:
            a.decompose()
        else:
            a.string = "Click here to be redirected to the link"
            links.add(a['href'])
    return str(soup)


def convert_links_to_clickable(text):
    soup = BeautifulSoup(text, 'html.parser')
    for a in soup.find_all('a', href=True):
        if not a.string or a.string.strip() == "":
            a.string = "Click here to be redirected to the link"
        else:
            a.string = "Click here to be redirected to the link"
    return str(soup)


def generate_html(news_items):
    logging.info("Generating HTML for news items...")
    news_html = ''

    for item in news_items:
        summary_with_links = convert_links_to_clickable(item["summary"])
        formatted_summary = format_summary(summary_with_links)
        news_html += f'''
        <div class="news-item">
            <h2 class="news-title"><strong>{item["title"]}</strong></h2>
            <p class="news-date">Author: {item["author"]}</p>
            <p class="news-date">Published on {item["date"]}</p>
            <div class="news-content">{formatted_summary}</div>
        </div>
        '''

    logging.info("Successfully generated HTML.")
    return news_html


def format_summary(summary):
    try:
        # Remove newlines and extra whitespace
        summary = re.sub(r'\s*\n\s*', ' ', summary)
        summary = re.sub(r'\s\s+', ' ', summary)

        # Flatten any heading tags to paragraphs
        summary = re.sub(r'<h[1-6][^>]*>', '<p class="news-paragraph">', summary)
        summary = re.sub(r'</h[1-6]>', '</p>', summary)

        # Remove any inline styles
        summary = re.sub(r'style="[^"]*"', '', summary)

        # Ensure all image links are prefixed with "www.gomotionapp.com"
        summary = re.sub(r'src="/', 'src="http://www.gomotionapp.com/', summary)

        # Convert newlines to <br> tags
        summary = summary.replace('\n', '<br>')

        # Convert numbered lists
        summary = re.sub(r'(\d+)\. ', r'<li>\1. ', summary)
        summary = re.sub(r'(<li>\d+\. [^<]+)<br>', r'\1</li>', summary)

        # Convert bulleted lists
        summary = re.sub(r'^\* ', r'<ul><li>', summary)
        summary = re.sub(r'<br>\* ', r'</li><li>', summary)
        summary = re.sub(r'(<li>[^<]+)<br>', r'\1</li>', summary)
        summary = re.sub(r'(<li>[^<]+)$', r'\1</li></ul>', summary)

        # Convert image links to "Click to see image" links
        summary = re.sub(r'<img src="([^"]+)"[^>]*>', r'<a href="\1" target="_blank">Click to see image</a>', summary)

        # Fix broken link formatting
        summary = re.sub(r'<a href="([^"]+)">([^<]+)</a>', r'<a href="\1" target="_blank">\2</a>', summary)

    except re.error as e:
        logging.error(f"Regex error while formatting summary: {e}")
        summary += "<br><em>Formatting error occurred. Displaying raw content.</em>"
    except Exception as e:
        logging.error(f"Unexpected error while formatting summary: {e}")
        summary += "<br><em>Unexpected error occurred. Displaying raw content.</em>"

    return summary


def update_html_file(news_html):
    try:
        if not os.path.exists(NEWS_HTML_FILE):
            logging.error(f"HTML file '{NEWS_HTML_FILE}' not found in the repository.")
            return

        logging.info("Updating HTML file...")
        with open(NEWS_HTML_FILE, 'r', encoding='utf-8') as file:
            content = file.read()

        start_marker = '<!-- START UNDER HERE -->'
        end_marker = '<!-- END AUTOMATION SCRIPT -->'
        start_index = content.find(start_marker) + len(start_marker)
        end_index = content.find(end_marker)

        if start_index == -1 or end_index == -1:
            logging.error("Markers not found in the HTML file.")
            return

        updated_content = content[:start_index] + '\n' + news_html + '\n' + content[end_index:]

        with open(NEWS_HTML_FILE, 'w', encoding='utf-8') as file:
            file.write(updated_content)
        logging.info("Successfully updated HTML file.")

    except IOError as e:
        logging.error(f"Error updating HTML file: {e}")


def push_to_github():
    try:
        logging.info("Pushing changes to GitHub...")
        repo = Repo(os.getcwd())

        # Set the remote URL to use the token for authentication
        remote_url = f'https://{GITHUB_TOKEN}@github.com/dareaquatics/dare-website.git'
        repo.remotes.origin.set_url(remote_url)

        if repo.is_dirty(untracked_files=True):
            with tqdm(total=100, desc='Committing changes') as pbar:
                def update_commit_pbar(cur_count, max_count=None, message=''):
                    if max_count:
                        pbar.total = max_count
                    pbar.update(cur_count - pbar.n)
                    pbar.set_postfix_str(message)

                repo.git.add(NEWS_HTML_FILE)
                repo.index.commit('automated commit: sync TeamUnify news articles [skip ci]')
                pbar.update(100)

            origin = repo.remote(name='origin')
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

        news_items = fetch_news()

        if not news_items:
            logging.error("No news items fetched. Aborting update process.")
            return

        news_html = generate_html(news_items)

        update_html_file(news_html)

        push_to_github()

        logging.info("Update process completed.")
    except Exception as e:
        logging.error(f"Update process failed: {e}")
        logging.info("Update process aborted due to errors.")


if __name__ == "__main__":
    main()
