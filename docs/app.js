const copyButtons = document.querySelectorAll('[data-copy-target]');

const normalizeBaseUrl = (value) => value.trim().replace(/\/+$/, '');
const appBaseUrl =
  typeof window !== 'undefined' && window.APP_BASE_URL
    ? normalizeBaseUrl(String(window.APP_BASE_URL))
    : '';

const updateBaseLinks = () => {
  if (!appBaseUrl) return;

  const baseLinks = [
    { selector: '[data-home-link]', path: '/index.html' },
    { selector: '[data-docs-link]', path: '/docs.html' },
    { selector: '[data-errors-link]', path: '/errors.html' },
    { selector: '[data-changelog-link]', path: '/changelog.html' },
    { selector: '[data-legal-notice-link]', path: '/legal-notice.html' },
  ];

  baseLinks.forEach(({ selector, path }) => {
    document.querySelectorAll(selector).forEach((link) => {
      if (link instanceof HTMLAnchorElement) {
        link.href = `${appBaseUrl}${path}`;
      }
    });
  });
};

updateBaseLinks();

// Anchor offsets follow the actual header when its navigation wraps on mobile.
const siteHeader = document.querySelector('.site-header');
if (siteHeader && typeof ResizeObserver !== 'undefined') {
  new ResizeObserver(([entry]) => {
    document.documentElement.style.setProperty('--header-height', `${entry.target.offsetHeight}px`);
  }).observe(siteHeader);
}

const sidebarToggle = document.querySelector('[data-sidebar-toggle]');
if (sidebarToggle) {
  const sidebar = sidebarToggle.closest('.sidebar');
  const mobileNavigation = window.matchMedia('(max-width: 1023px)');
  const setNavigationExpanded = (expanded) => {
    sidebarToggle.setAttribute('aria-expanded', String(expanded));
    sidebar.dataset.collapsed = String(!expanded);
  };
  sidebarToggle.hidden = false;
  setNavigationExpanded(!mobileNavigation.matches);
  mobileNavigation.addEventListener('change', () => setNavigationExpanded(!mobileNavigation.matches));
  sidebarToggle.addEventListener('click', () => {
    setNavigationExpanded(sidebarToggle.getAttribute('aria-expanded') !== 'true');
  });
  sidebar.querySelectorAll('[data-nav-link]').forEach((link) => {
    link.addEventListener('click', () => {
      if (mobileNavigation.matches) setNavigationExpanded(false);
    });
  });
}

copyButtons.forEach((button) => {
  button.setAttribute('aria-live', 'polite');
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

const openDetailsFromHash = () => {
  const hash = window.location.hash;
  if (!hash) return;
  let id;
  try {
    id = decodeURIComponent(hash.slice(1));
  } catch {
    return;
  }
  const target = document.getElementById(id);
  for (let element = target; element; element = element.parentElement) {
    if (element.tagName === 'DETAILS') element.open = true;
  }
  if (target) requestAnimationFrame(() => target.scrollIntoView({ block: 'start' }));
};

openDetailsFromHash();
window.addEventListener('hashchange', openDetailsFromHash);

if (sections.length > 0) {
  let currentId;
  let framePending = false;
  const updateCurrentSection = () => {
    framePending = false;
    const offset = (siteHeader?.offsetHeight || 0) + 64;
    let current = sections[0];
    for (const section of sections) {
      if (section.getBoundingClientRect().top <= offset) current = section;
    }
    if (current.id === currentId) return;
    currentId = current.id;
    navLinks.forEach((link) => {
      const active = link.getAttribute('href') === `#${currentId}`;
      link.setAttribute('data-active', String(active));
      if (active) link.setAttribute('aria-current', 'location');
      else link.removeAttribute('aria-current');
    });
  };
  const scheduleSectionUpdate = () => {
    if (framePending) return;
    framePending = true;
    requestAnimationFrame(updateCurrentSection);
  };
  window.addEventListener('scroll', scheduleSectionUpdate, { passive: true });
  window.addEventListener('resize', scheduleSectionUpdate);
  updateCurrentSection();
}

const docSearch = document.querySelector('#doc-search');
const searchResults = document.querySelector('#doc-search-results');

if (docSearch && searchResults) {
  const searchRoot = docSearch.closest('.header-search');
  const searchPanel = searchRoot.querySelector('.search-panel');
  const searchCaption = searchRoot.querySelector('[data-search-caption]');
  const searchStatus = document.querySelector('#doc-search-status');
  const shortcutHint = searchRoot.querySelector('.search-key');
  shortcutHint.textContent = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘ K' : 'Ctrl K';
  let activeIndex = -1;
  let matches = [];

  const searchItems = docSections.map((section) => {
    const title = section.dataset.docTitle || section.querySelector('h2, h3')?.textContent?.trim() || section.id;
    const code = (section.dataset.docCode || '').trim();
    const group = (section.dataset.docGroup || '').replace(/-/g, ' ');
    const groupLabel = group.replace(/\b\w/g, (letter) => letter.toUpperCase());
    // Parent sections must not outrank a specific topic by inheriting all its text.
    const content = section.cloneNode(true);
    content.querySelectorAll('[data-doc-section], button').forEach((child) => child.remove());
    const fields = [
      { text: normalizeSearch(code), weight: 0 },
      { text: normalizeSearch(title), weight: 1 },
      { text: normalizeSearch(section.dataset.docTags || ''), weight: 3 },
      { text: normalizeSearch(group), weight: 5 },
      { text: normalizeSearch(content.textContent || ''), weight: 12 },
    ].map((field) => ({ ...field, words: [...new Set(field.text.split(' ').filter(Boolean))] }));
    return { id: section.id, title, code, groupLabel, fields };
  });

  const setResultsVisible = (visible) => {
    searchPanel.hidden = !visible;
    docSearch.setAttribute('aria-expanded', String(visible));
    if (!visible) {
      docSearch.removeAttribute('aria-activedescendant');
      activeIndex = -1;
    }
  };

  const setActive = (index, scroll = false) => {
    const options = [...searchResults.querySelectorAll('[role="option"]')];
    activeIndex = index;
    options.forEach((option, position) => option.setAttribute('aria-selected', String(position === index)));
    const active = options[index];
    if (active) {
      docSearch.setAttribute('aria-activedescendant', active.id);
      if (scroll) active.scrollIntoView({ block: 'nearest' });
    } else {
      docSearch.removeAttribute('aria-activedescendant');
    }
  };

  const openResult = (item) => {
    if (!item) return;
    setResultsVisible(false);
    const hash = `#${item.id}`;
    if (window.location.hash === hash) openDetailsFromHash();
    else window.location.hash = hash;
    const target = document.getElementById(item.id);
    if (target) {
      if (!target.hasAttribute('tabindex')) target.setAttribute('tabindex', '-1');
      target.focus({ preventScroll: true });
    }
  };

  const renderResults = () => {
    const query = normalizeSearch(docSearch.value);
    const terms = query.split(' ').filter(Boolean).slice(0, 12);
    matches = query
      ? searchItems.map((item) => ({ item, score: matchScore(item, query, terms) }))
          .filter((entry) => Number.isFinite(entry.score))
          .sort((a, b) => a.score - b.score || a.item.title.localeCompare(b.item.title))
          .slice(0, 8).map((entry) => entry.item)
      : searchItems.slice(0, 6);
    searchResults.replaceChildren();
    activeIndex = -1;
    docSearch.removeAttribute('aria-activedescendant');
    searchCaption.textContent = query ? `${matches.length} matching section${matches.length === 1 ? '' : 's'}` : 'Suggested sections';
    searchStatus.textContent = matches.length ? `${matches.length} suggestions available. Use up and down arrows to navigate.` : 'No matching sections.';

    if (!matches.length) {
      const empty = document.createElement('div');
      empty.className = 'search-result-empty';
      empty.setAttribute('role', 'presentation');
      empty.textContent = 'No matching sections. Try a topic or error code.';
      searchResults.appendChild(empty);
    }

    matches.forEach((item, index) => {
      const option = document.createElement('a');
      option.id = `search-option-${index}`;
      option.href = `#${item.id}`;
      option.className = 'search-result';
      option.setAttribute('role', 'option');
      option.setAttribute('aria-selected', 'false');
      option.tabIndex = -1;
      const title = document.createElement('strong');
      title.textContent = item.title;
      const meta = document.createElement('span');
      meta.textContent = [item.code, item.groupLabel !== item.title ? item.groupLabel : ''].filter(Boolean).join(' · ') || 'Section';
      option.append(title, meta);
      option.addEventListener('pointermove', () => setActive(index));
      // Keep typing focus in the combobox until a result is chosen.
      option.addEventListener('mousedown', (event) => event.preventDefault());
      option.addEventListener('click', (event) => {
        if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        openResult(item);
      });
      searchResults.appendChild(option);
    });
    setResultsVisible(true);
  };

  docSearch.closest('form')?.addEventListener('submit', (event) => event.preventDefault());
  docSearch.addEventListener('input', renderResults);
  docSearch.addEventListener('focus', renderResults);
  docSearch.addEventListener('click', () => { if (searchPanel.hidden) renderResults(); });
  searchRoot.querySelector('.search-field').addEventListener('click', () => docSearch.focus());
  docSearch.addEventListener('keydown', (event) => {
    if (event.isComposing) return;
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      if (searchPanel.hidden) renderResults();
      if (!matches.length) return;
      const direction = event.key === 'ArrowDown' ? 1 : -1;
      const next = activeIndex < 0 ? (direction > 0 ? 0 : matches.length - 1) : (activeIndex + direction + matches.length) % matches.length;
      setActive(next, true);
    } else if (event.key === 'Enter' && !searchPanel.hidden) {
      event.preventDefault();
      openResult(matches[Math.max(0, activeIndex)]);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      setResultsVisible(false);
    } else if (event.key === 'Tab') {
      setResultsVisible(false);
    }
  });

  window.addEventListener('hashchange', () => setResultsVisible(false));
  document.addEventListener('pointerdown', (event) => {
    if (!searchRoot.contains(event.target)) setResultsVisible(false);
  });
  document.addEventListener('focusin', (event) => {
    if (!searchRoot.contains(event.target)) setResultsVisible(false);
  });
  document.addEventListener('keydown', (event) => {
    if (event.isComposing) return;
    const shortcut = (event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'k';
    const editing = event.target instanceof Element && event.target.closest('input, textarea, select, [contenteditable="true"]');
    const slash = event.key === '/' && !editing && !event.ctrlKey && !event.metaKey && !event.altKey;
    if (shortcut || slash) {
      event.preventDefault();
      docSearch.focus();
      if (shortcut) docSearch.select();
      renderResults();
    }
  });
}

function normalizeSearch(value) {
  return value.normalize('NFKD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();
}

function matchScore(item, query, terms) {
  const compactCode = item.fields[0].text.replace(/ /g, '');
  const compactQuery = query.replace(/ /g, '');
  if (compactCode && compactCode === compactQuery) return -100;
  if (compactCode && compactCode.startsWith(compactQuery)) return -75;
  let score = 0;
  for (const term of terms) {
    let best = Infinity;
    for (const field of item.fields) {
      for (const word of field.words) {
        const cost = fuzzyWordScore(term, word);
        best = Math.min(best, cost + field.weight);
      }
    }
    if (!Number.isFinite(best)) return Infinity;
    score += best;
  }
  if (item.fields[1].text === query) score -= 10;
  else if (item.fields[1].text.includes(query)) score -= 5;
  return score / terms.length;
}

function fuzzyWordScore(term, word) {
  if (word === term) return 0;
  if (word.startsWith(term)) return 1 + (word.length - term.length) / 100;
  if (term.length >= 3 && word.includes(term)) return 3;
  if (term.length < 3) return Infinity;
  const allowedEdits = term.length >= 8 ? 2 : 1;
  if (Math.abs(term.length - word.length) <= allowedEdits) {
    const distance = editDistance(term, word);
    if (distance <= allowedEdits) return 4 + distance;
  }
  // Abbreviations such as "wrkbnch" still match "workbench".
  if (term.length >= 4 && word.length <= term.length * 2) {
    let cursor = 0;
    for (const letter of word) if (letter === term[cursor]) cursor += 1;
    if (cursor === term.length) return 8 + (word.length - term.length) / term.length;
  }
  return Infinity;
}

// Optimal string alignment includes a single adjacent-letter transposition.
function editDistance(left, right) {
  let previousPrevious;
  let previous = Array.from({ length: right.length + 1 }, (_, index) => index);
  for (let row = 1; row <= left.length; row += 1) {
    const current = [row];
    for (let column = 1; column <= right.length; column += 1) {
      current[column] = Math.min(
        current[column - 1] + 1,
        previous[column] + 1,
        previous[column - 1] + (left[row - 1] === right[column - 1] ? 0 : 1)
      );
      if (row > 1 && column > 1 && left[row - 1] === right[column - 2] && left[row - 2] === right[column - 1]) {
        current[column] = Math.min(current[column], previousPrevious[column - 2] + 1);
      }
    }
    previousPrevious = previous;
    previous = current;
  }
  return previous[right.length];
}
