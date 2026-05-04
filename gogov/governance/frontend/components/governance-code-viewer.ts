import { LitElement, html } from 'lit';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { codeToHtml } from 'shiki';

type SourceResponse = {
  path: string;
  lang: string;
  code: string;
};

export class GovernanceCodeViewer extends LitElement {
  static properties = {
    file: { type: Object },
    highlighted: { state: true },
    errorMessage: { state: true },
    loading: { state: true },
  };

  declare file?: SourceResponse;

  declare protected highlighted: string;

  declare protected errorMessage: string;

  declare protected loading: boolean;

  private renderVersion = 0;

  constructor() {
    super();
    this.file = undefined;
    this.highlighted = '';
    this.errorMessage = '';
    this.loading = false;
  }

  createRenderRoot() {
    return this;
  }

  connectedCallback(): void {
    super.connectedCallback();
  }

  protected updated(changed: Map<string, unknown>): void {
    if (changed.has('file')) {
      void this.renderSource();
    }
  }

  render() {
    if (this.errorMessage) {
      return html`<div class="governance-code-error">${this.errorMessage}</div>`;
    }

    if (this.loading && !this.highlighted) {
      return html`<div class="governance-code-loading">Loading Go source...</div>`;
    }

    if (!this.file || !this.highlighted) {
      return html`<div class="governance-code-empty">No source content available.</div>`;
    }

    return html`
      <section class="governance-code-shell">
        <div class="governance-code-meta">
          <div>
            <strong>${this.file.path}</strong>
            <span>${this.file.lang.toUpperCase()} source</span>
          </div>
        </div>
        ${unsafeHTML(this.highlighted)}
      </section>
    `;
  }

  private async renderSource(): Promise<void> {
    const version = this.renderVersion + 1;
    this.renderVersion = version;

    if (!this.file) {
      this.file = undefined;
      this.highlighted = '';
      this.errorMessage = '';
      this.loading = false;
      this.requestUpdate();
      return;
    }

    this.loading = true;
    this.highlighted = '';
    this.errorMessage = '';
    this.requestUpdate();

    try {
      const highlighted = await codeToHtml(this.file.code, {
        lang: this.file.lang,
        theme: 'github-light',
      });
      if (this.renderVersion != version) {
        return;
      }
      this.highlighted = highlighted;
    } catch (error) {
      if (this.renderVersion != version) {
        return;
      }

      this.highlighted = '';
      this.errorMessage = error instanceof Error ? error.message : 'Unknown source loading error';
    } finally {
      if (this.renderVersion == version) {
        this.loading = false;
        this.requestUpdate();
      }
    }
  }
}

customElements.define('governance-code-viewer', GovernanceCodeViewer);
