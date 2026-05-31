// KubeGate Mockup — shared prototype interactions

// ─── Toast auto-dismiss ───────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  // Auto-dismiss any static toast after 4 seconds
  document.querySelectorAll('.toast-stack .toast').forEach(t => {
    setTimeout(() => {
      t.style.transition = 'opacity .3s ease, transform .3s ease';
      t.style.opacity = '0';
      t.style.transform = 'translateX(12px)';
      setTimeout(() => t.remove(), 320);
    }, 4000);
  });

  // Filter chips toggle (products page)
  document.querySelectorAll('.filter-chip').forEach(chip => {
    chip.addEventListener('click', () => {
      chip.closest('.filter-row').querySelectorAll('.filter-chip').forEach(c => c.classList.remove('on'));
      chip.classList.add('on');
    });
  });

  // Nav item active state (single-page highlight)
  const currentFile = window.location.pathname.split('/').pop() || 'index.html';
  document.querySelectorAll('.nav-item').forEach(item => {
    const href = item.getAttribute('href');
    if (href && href !== '#' && href.includes(currentFile)) {
      item.classList.add('active');
    }
  });
});
