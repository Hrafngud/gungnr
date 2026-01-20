const copyButtons = document.querySelectorAll('[data-copy-target]');

copyButtons.forEach((button) => {
  const original = button.textContent;
  button.addEventListener('click', async () => {
    const target = button.getAttribute('data-copy-target');
    if (!target) return;
    const block = document.querySelector(target);
    if (!block) return;

    const text = block.textContent || '';
    let copied = false;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      try {
        await navigator.clipboard.writeText(text.trim());
        copied = true;
      } catch (err) {
        copied = false;
      }
    }

    if (!copied) {
      const range = document.createRange();
      range.selectNodeContents(block);
      const selection = window.getSelection();
      if (selection) {
        selection.removeAllRanges();
        selection.addRange(range);
      }
      try {
        copied = document.execCommand('copy');
      } catch (err) {
        copied = false;
      }
      if (selection) {
        selection.removeAllRanges();
      }
    }

    button.textContent = copied ? 'Copied' : 'Copy';
    button.setAttribute('data-copy-state', copied ? 'copied' : 'idle');
    window.setTimeout(() => {
      button.textContent = original || 'Copy';
      button.setAttribute('data-copy-state', 'idle');
    }, 1200);
  });
});

const navLinks = Array.from(document.querySelectorAll('[data-nav-link]'));
const sections = navLinks
  .map((link) => document.querySelector(link.getAttribute('href') || ''))
  .filter(Boolean);

const docSections = Array.from(document.querySelectorAll('[data-doc-section]'));

const setDocsVisibility = () => {
  const hash = window.location.hash;
  if (!hash) {
    document.body.dataset.docsVisible = 'false';
    return;
  }

  const isDocs =
    hash === '#docs' || docSections.some((section) => `#${section.id}` === hash);
  document.body.dataset.docsVisible = isDocs ? 'true' : 'false';
};

setDocsVisibility();
window.addEventListener('hashchange', setDocsVisibility);

const heroLogoMark = document.querySelector('.hero-logo-mark');
if (heroLogoMark) {
  const heroSpear = heroLogoMark.querySelector('.hero-spear');
  if (heroSpear) {
    let spearRunning = false;

    const triggerSpearPulse = () => {
      if (spearRunning) return;
      spearRunning = true;
      heroLogoMark.classList.remove('spear-pulse');
      void heroLogoMark.offsetWidth;
      heroLogoMark.classList.add('spear-pulse');
    };

    heroSpear.addEventListener('animationend', (event) => {
      if (event.animationName !== 'hero-spear-wobble') return;
      spearRunning = false;
      heroLogoMark.classList.remove('spear-pulse');
    });

    document.addEventListener('mousemove', triggerSpearPulse);
  }
}

if (sections.length > 0) {
  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) return;
        const id = entry.target.getAttribute('id');
        navLinks.forEach((link) => {
          const active = link.getAttribute('href') === `#${id}`;
          link.setAttribute('data-active', active ? 'true' : 'false');
        });
      });
    },
    {
      rootMargin: '-40% 0px -50% 0px',
      threshold: 0.1,
    }
  );

  sections.forEach((section) => observer.observe(section));
}

const docSearch = document.querySelector('#doc-search');
const searchResults = document.querySelector('#doc-search-results');

if (docSearch && searchResults) {
  const searchField = docSearch.closest('.search-field');
  const triggerSearchPulse = () => {
    if (!searchField) return;
    searchField.classList.remove('search-field-pulse');
    void searchField.offsetWidth;
    searchField.classList.add('search-field-pulse');
  };

  if (searchField) {
    searchField.addEventListener('animationend', (event) => {
      if (event.animationName === 'search-pulse') {
        searchField.classList.remove('search-field-pulse');
      }
    });
  }

  const searchForm = docSearch.closest('form');
  if (searchForm) {
    searchForm.addEventListener('submit', (event) => {
      event.preventDefault();
    });
  }

  const searchItems = docSections.map((section) => {
    const title =
      section.dataset.docTitle ||
      section.querySelector('h2, h3')?.textContent?.trim() ||
      section.id;
    const group = section.dataset.docGroup || '';
    const tags = section.dataset.docTags || '';
    const groupLabel = group
      ? group.replace(/-/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase())
      : '';
    const label =
      groupLabel && groupLabel !== title ? `${groupLabel} / ${title}` : title;
    const searchable = `${title} ${groupLabel} ${tags} ${section.textContent || ''}`.toLowerCase();

    return { id: section.id, title, group, groupLabel, label, searchable };
  });

  const renderResults = (query) => {
    const trimmed = query.trim().toLowerCase();
    searchResults.innerHTML = '';

    if (!trimmed) {
      searchResults.hidden = true;
      return;
    }

    const matches = searchItems
      .filter((item) => item.searchable.includes(trimmed))
      .slice(0, 6);

    if (matches.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'search-result-empty';
      empty.textContent = 'No matches. Try "install" or "tunnel".';
      searchResults.appendChild(empty);
      searchResults.hidden = false;
      return;
    }

    matches.forEach((item) => {
      const link = document.createElement('a');
      link.href = `#${item.id}`;
      link.className = 'search-result';

      const title = document.createElement('strong');
      title.textContent = item.title;

      const meta = document.createElement('span');
      meta.textContent =
        item.groupLabel && item.groupLabel !== item.title
          ? item.groupLabel
          : 'Section';

      link.appendChild(title);
      link.appendChild(meta);
      searchResults.appendChild(link);
    });

    searchResults.hidden = false;
  };

  docSearch.addEventListener('input', () => renderResults(docSearch.value));

  docSearch.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
      const first = searchResults.querySelector('a.search-result');
      if (first) {
        event.preventDefault();
        window.location.hash = first.getAttribute('href') || '#docs';
        searchResults.hidden = true;
      }
    }
  });

  window.addEventListener('hashchange', () => {
    searchResults.hidden = true;
  });

  document.addEventListener('click', (event) => {
    if (event.target === docSearch || searchResults.contains(event.target)) {
      return;
    }
    searchResults.hidden = true;
  });

  document.addEventListener('keydown', (event) => {
    if (event.key === '/' && document.activeElement !== docSearch) {
      event.preventDefault();
      docSearch.focus();
    }
  });

  document.addEventListener('keydown', (event) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') {
      event.preventDefault();
      docSearch.focus();
      docSearch.select();
      triggerSearchPulse();
    }
  });
}
