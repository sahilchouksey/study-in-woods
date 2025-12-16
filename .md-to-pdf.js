module.exports = {
  stylesheet: [
    'https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/5.5.1/github-markdown.min.css',
    'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css',
  ],
  css: `
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen', 'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue', sans-serif;
      max-width: 900px;
      margin: 0 auto;
      padding: 2rem;
      line-height: 1.6;
    }
    .markdown-body {
      box-sizing: border-box;
      min-width: 200px;
      max-width: 980px;
      margin: 0 auto;
      padding: 45px;
    }
    h1, h2, h3 {
      color: #1a1a1a;
      border-bottom: 2px solid #e1e4e8;
      padding-bottom: 0.3em;
    }
    code {
      background-color: #f6f8fa;
      padding: 0.2em 0.4em;
      border-radius: 3px;
      font-family: 'Courier New', monospace;
    }
    pre {
      background-color: #f6f8fa;
      padding: 16px;
      border-radius: 6px;
      overflow-x: auto;
    }
    blockquote {
      border-left: 4px solid #dfe2e5;
      padding-left: 1em;
      color: #6a737d;
    }
    ul, ol {
      padding-left: 2em;
    }
    li {
      margin-bottom: 0.5em;
    }
    hr {
      border: 0;
      border-top: 2px solid #e1e4e8;
      margin: 2em 0;
    }
  `,
  body_class: 'markdown-body',
  marked_options: {
    headerIds: false,
    smartypants: true,
  },
  pdf_options: {
    format: 'A4',
    margin: {
      top: '20mm',
      right: '15mm',
      bottom: '20mm',
      left: '15mm'
    },
    printBackground: true,
    displayHeaderFooter: true,
    headerTemplate: '<div></div>',
    footerTemplate: `
      <div style="font-size: 10px; text-align: center; width: 100%; padding: 0 15mm; color: #666;">
        <span>Study in Woods - Project Overview</span>
        <span style="float: right;">Page <span class="pageNumber"></span> of <span class="totalPages"></span></span>
      </div>
    `,
  },
  launch_options: {
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  }
};
