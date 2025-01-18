// /assets/js/announcement.js
class EmergencyAnnouncement {
    constructor() {
        this.config = null;
        this.element = null;
        this.overlay = null;
        this.configId = null;
        this.fetchController = null;
        this.resizeObserver = null;
    }

    async init() {
        try {
            await this.fetchConfig();
            if (this.isActive() && !this.isHidden()) {
                this.render();
                this.setupEventListeners();
            }
        } catch (error) {
            console.error('Failed to initialize announcement:', error);
        }
    }

    async fetchConfig() {
        this.fetchController = new AbortController();
        const timeoutId = setTimeout(() => this.fetchController.abort(), 5000);

        try {
            const response = await fetch('/config.yaml', {
                signal: this.fetchController.signal,
                headers: { 'Cache-Control': 'no-cache' }
            });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const yamlText = await response.text();
            this.config = jsyaml.load(yamlText);

            if (!this.validateConfig()) {
                throw new Error('Invalid announcement configuration');
            }

            this.configId = this.generateConfigId();
        } finally {
            clearTimeout(timeoutId);
            this.fetchController = null;
        }
    }

    validateConfig() {
        const requiredFields = ['active', 'type', 'message', 'instance'];
        return requiredFields.every(field => field in this.config) &&
            ['popup', 'banner'].includes(this.config.type);
    }

    isActive() {
        return String(this.config.active).toLowerCase() === 'yes';
    }

    generateConfigId() {
        const { message, type, instance } = this.config;
        return btoa(`${message}|${type}|${instance}`).substr(0, 20);
    }

    isHidden() {
        const hiddenAnnouncements = this.getHiddenAnnouncements();
        const hiddenTimestamp = hiddenAnnouncements[this.configId];
        if (hiddenTimestamp) {
            const currentTimestamp = new Date(this.config.instance).getTime();
            return currentTimestamp <= hiddenTimestamp;
        }
        return false;
    }

    getHiddenAnnouncements() {
        try {
            return JSON.parse(localStorage.getItem('hiddenAnnouncements') || '{}');
        } catch (error) {
            console.error('Failed to parse hidden announcements:', error);
            return {};
        }
    }

    render() {
        this.createOverlay();
        this.element = document.createElement('div');
        this.element.className = `announcement announcement--${this.config.type}`;
        if (this.config.advanced?.custom_class) {
            this.element.classList.add(this.config.advanced.custom_class);
        }
        this.element.setAttribute('role', 'alert');
        this.element.innerHTML = this.generateAnnouncementHTML();

        this.applyStyles();
        document.body.appendChild(this.element);

        if (this.config.type === 'banner') {
            this.adjustPageLayout();
        }

        requestAnimationFrame(() => {
            this.element.classList.add('announcement--visible');
        });
    }

    generateAnnouncementHTML() {
        return `
          <div class="announcement__content">${this.sanitizeHTML(this.config.message)}</div>
          <div class="announcement__actions">
              <button class="announcement__close" aria-label="Close announcement"><span>&times;</span></button>
          </div>
      `;
    }

    sanitizeHTML(html) {
        const template = document.createElement('template');
        template.innerHTML = html.trim();
        return template.content.textContent || template.content.firstChild?.nodeValue || '';
    }

    applyStyles() {
        const style = this.config.style || {};
        const css = {
            backgroundColor: style.background_color || 'rgba(255, 0, 0, 0.9)',
            color: style.text_color || 'white',
            fontSize: style.font_size || '16px',
            ...this.parseCssString(style.custom_css)
        };
        Object.assign(this.element.style, css);
    }

    parseCssString(cssString) {
        if (!cssString) return {};
        return cssString.split(';')
            .reduce((acc, rule) => {
                const [key, value] = rule.split(':').map(str => str.trim());
                if (key && value) {
                    acc[this.camelCase(key)] = value;
                }
                return acc;
            }, {});
    }

    camelCase(str) {
        return str.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
    }

    createOverlay() {
        if (this.config.type === 'popup') {
            this.overlay = document.createElement('div');
            this.overlay.className = 'announcement-overlay';
            document.body.appendChild(this.overlay);
        }
    }

    setupEventListeners() {
        this.element.querySelector('.announcement__close').addEventListener('click', () => this.close());
        this.element.querySelector('.announcement__hide').addEventListener('click', () => this.hide());

        this.resizeObserver = new ResizeObserver(this.adjustPageLayout.bind(this));
        this.resizeObserver.observe(this.element);
    }

    adjustPageLayout() {
        if (this.config.type === 'banner' && this.element) {
            const announcementHeight = this.element.offsetHeight;
            document.body.style.paddingTop = `${announcementHeight}px`;
        }
    }

    close() {
        this.element.classList.remove('announcement--visible');
        this.element.addEventListener('transitionend', () => {
            this.cleanup();
        }, { once: true });
    }

    hide() {
        const hiddenAnnouncements = this.getHiddenAnnouncements();
        hiddenAnnouncements[this.configId] = new Date().getTime();
        try {
            localStorage.setItem('hiddenAnnouncements', JSON.stringify(hiddenAnnouncements));
        } catch (error) {
            console.error('Failed to save hidden announcements:', error);
        }
        this.close();
    }

    cleanup() {
        if (this.resizeObserver) {
            this.resizeObserver.disconnect();
        }
        this.element.remove();
        if (this.overlay) {
            this.overlay.remove();
        }
        document.body.style.paddingTop = '0';
    }
}

function initAnnouncement() {
    const announcement = new EmergencyAnnouncement();
    announcement.init();
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAnnouncement);
} else {
    initAnnouncement();
}