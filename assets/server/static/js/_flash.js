(() => {
  // Flash is a class that manages flash alerts and messages.
  class Flash {
    #container;
    #template;

    constructor(container, template) {
      this.#container = container;
      this.#template = template;
    }

    clear() {
      while (this.#container && this.#container.firstChild) {
        this.#container.removeChild(this.#container.firstChild);
      }
    }

    error(message) {
      this.flash('error', message);
    }

    warning(message) {
      this.flash('warning', message);
    }

    alert(message) {
      this.flash('alert', message);
    }

    flash(level, message) {
      const alert = this.#template.cloneNode(true);

      const body = alert.querySelector('.alert-message');
      body.textContent = message;

      const icon = alert.querySelector('.bi');

      switch (level) {
        case 'error':
          alert.classList.add('alert-danger');
          icon.classList.add('bi-exclamation-octagon-fill');
          break;
        case 'warning':
          alert.classList.add('alert-warning');
          icon.classList.add('bi-exclamation-square-fill');
          break;
        case 'alert':
          alert.classList.add('alert-success');
          icon.classList.add('bi-check-square-fill');
          break;
        default:
          throw `invalid flash level ${level}`;
      }

      this.#container.appendChild(alert).focus();
      alert.classList.remove('d-none');
    }
  }

  // Add proper classes to all links inside alerts that were rendered with the
  // page (server-side).
  window.addEventListener('DOMContentLoaded', () => {
    // alertsContainer is the container div for all alerts. alertTemplate is the
    // cloneable HTML fragement from which other alerts are generated.
    const alertsContainer = document.querySelector('body div#alerts-container');
    if (!alertsContainer) {
      return;
    }

    const alertTemplate = alertsContainer.querySelector('div#alert-template');
    if (!alertTemplate) {
      return;
    }

    // Make flash available everywhere.
    const flash = new Flash(alertsContainer, alertTemplate);
    window.flash = flash;
  });
})();
