'use strict';

document.addEventListener('click', async (event) => {
  const button = event.target.closest('[data-copy-link]');
  if (!button) return;
  const status = button.parentElement.querySelector('[role="status"]');
  try {
    await navigator.clipboard.writeText(button.dataset.copyLink);
    status.textContent = button.dataset.copyMessage || 'Link copied';
  } catch {
    status.textContent = button.dataset.copyMessage
      ? 'Select and copy the configuration above.'
      : 'Open Technical details to copy the link.';
  }
});
