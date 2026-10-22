import './tooltip.css';

// Progressive enhancement for links, buttons and native summary controls.
// Labels remain available to assistive technology without hovering or JavaScript.
export function tooltip(node: HTMLElement, label: string) {
  let text = label;
  let tip: HTMLSpanElement | undefined;
  let hovered = false;
  let focused = false;
  let dismissed = false;
  const id = `tooltip-${crypto.randomUUID()}`;
  const previous = node.getAttribute('aria-describedby');
  function position() {
    if (!tip) return;
    const anchor = node.getBoundingClientRect();
    const bounds = tip.getBoundingClientRect();
    tip.style.left = `${Math.max(8, Math.min(anchor.left, innerWidth - bounds.width - 8))}px`;
    tip.style.top = `${anchor.top >= bounds.height + 8 ? anchor.top - bounds.height : anchor.bottom}px`;
  }
  function hide() {
    tip?.remove(); tip = undefined;
    if (previous) node.setAttribute('aria-describedby', previous);
    else node.removeAttribute('aria-describedby');
  }
  function refresh() {
    if (!text || dismissed || (!hovered && !focused) || node.matches(':disabled')) { hide(); return; }
    if (!tip) {
      tip = document.createElement('span');
      tip.id = id; tip.role = 'tooltip'; tip.className = 'ocp-tooltip';
      // Staying inside the trigger also keeps tooltips in a dialog's top layer.
      node.append(tip);
      node.setAttribute('aria-describedby', [previous,id].filter(Boolean).join(' '));
    }
    tip.textContent = text;
    position();
  }
  const enter = () => { hovered = true; dismissed = false; refresh(); };
  const leave = () => { hovered = false; refresh(); };
  const focus = () => { focused = true; dismissed = false; refresh(); };
  const blur = () => { focused = false; refresh(); };
  const dismiss = () => { dismissed = true; hide(); };
  const key = (event: KeyboardEvent) => { if (event.key === 'Escape') dismiss(); };
  node.addEventListener('pointerenter', enter); node.addEventListener('pointerleave', leave);
  node.addEventListener('focus', focus); node.addEventListener('blur', blur);
  node.addEventListener('click', dismiss); window.addEventListener('keydown', key);
  window.addEventListener('scroll', position, true); window.addEventListener('resize', position);
  return {
    update(label: string) { text = label; refresh(); },
    destroy() {
      hide();
      node.removeEventListener('pointerenter', enter); node.removeEventListener('pointerleave', leave);
      node.removeEventListener('focus', focus); node.removeEventListener('blur', blur);
      node.removeEventListener('click', dismiss); window.removeEventListener('keydown', key);
      window.removeEventListener('scroll', position, true); window.removeEventListener('resize', position);
    },
  };
}
