// HomeWaf click challenge UI. Original code; see the root HomeWaf license.
(() => {
  class Click {
    constructor(options = {}) { this.options = options; this.dots = []; this.events = {}; }
    setEvents(events) { this.events = events; }
    mount(root) {
      this.root = root;
      const title = document.createElement('h2');
      title.textContent = this.options.title || 'Verify';
      this.thumb = document.createElement('img'); this.thumb.alt = title.textContent;
      this.thumb.className = 'homewaf-challenge-thumb';
      this.area = document.createElement('div'); this.area.className = 'homewaf-challenge-area';
      this.image = document.createElement('img'); this.image.alt = title.textContent;
      this.image.draggable = false; this.area.append(this.image);
      this.area.addEventListener('click', event => {
        if (!this.image.naturalWidth || this.dots.length >= 12 || this.confirm.disabled) return;
        const rect = this.image.getBoundingClientRect();
        const x = Math.round((event.clientX - rect.left) * this.image.naturalWidth / rect.width);
        const y = Math.round((event.clientY - rect.top) * this.image.naturalHeight / rect.height);
        if (x < 0 || y < 0 || x >= this.image.naturalWidth || y >= this.image.naturalHeight) return;
        this.dots.push({x, y});
        const marker = document.createElement('span'); marker.className = 'homewaf-challenge-marker';
        marker.textContent = String(this.dots.length);
        marker.style.left = `${100 * x / this.image.naturalWidth}%`;
        marker.style.top = `${100 * y / this.image.naturalHeight}%`;
        this.area.append(marker); this.events.click?.(x, y);
      });
      this.confirm = document.createElement('button'); this.confirm.type = 'button';
      this.confirm.textContent = this.options.buttonText || 'Confirm';
      this.confirm.addEventListener('click', () => {
        if (!this.dots.length) return;
        this.confirm.disabled = true;
        this.events.confirm?.([...this.dots], () => this.clear());
      });
      const refresh = document.createElement('button'); refresh.type = 'button';
      refresh.textContent = '↻'; refresh.setAttribute('aria-label', 'Refresh challenge');
      refresh.addEventListener('click', () => this.events.refresh?.());
      root.replaceChildren(title, this.thumb, this.area, this.confirm, refresh);
    }
    clear() {
      this.dots = [];
      this.area?.querySelectorAll('.homewaf-challenge-marker').forEach(el => el.remove());
      if (this.confirm) this.confirm.disabled = false;
    }
    setData({image, thumb}) {
      for (const [el, value] of [[this.image, image], [this.thumb, thumb]]) {
        // The backend emits PNG/JPEG data URLs; disallow remote tracking images.
        if (typeof value !== 'string' || !/^data:image\/(png|jpeg);base64,/i.test(value)) {
          el.removeAttribute('src'); continue;
        }
        el.src = value;
      }
      this.clear();
    }
  }
  window.GoCaptcha = {Click};
})();
