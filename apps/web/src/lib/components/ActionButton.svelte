<script lang="ts">
  import type { HTMLButtonAttributes } from 'svelte/elements';
  import type { LucideIcon } from '@lucide/svelte';
  import { onMount } from 'svelte';
  import '$lib/tooltip.css';

  type Props = Omit<HTMLButtonAttributes, 'children' | 'title' | 'aria-label'> & {
    label: string;
    icon?: LucideIcon;
    iconOnly?: boolean;
    tooltip?: string;
    danger?: boolean;
  };
  let { label, icon: Icon, iconOnly = false, tooltip = label, danger = false,
    type = 'submit', disabled = false, class: className = '',
    onfocus, onblur, onclick, ...attributes }: Props = $props();
  const tooltipId = $props.id();
  let hovered = $state(false);
  let focused = $state(false);
  let dismissed = $state(false);
  let interactive = $state(false);
  onMount(() => { interactive = true; });
  const visible = $derived((iconOnly || tooltip !== label) && !disabled && !dismissed && (hovered || focused));
  let button = $state<HTMLButtonElement>();
  let tip = $state<HTMLSpanElement>();
  let tipLeft = $state(0);
  let tipTop = $state(0);
  function positionTooltip() {
    if (!tip || !button) return;
    const anchor = button.getBoundingClientRect();
    const bounds = tip.getBoundingClientRect();
    tipLeft = Math.max(8, Math.min(anchor.left, window.innerWidth - bounds.width - 8));
    tipTop = anchor.top >= bounds.height + 8 ? anchor.top - bounds.height : anchor.bottom;
  }
  $effect(() => { if (visible) positionTooltip(); });
</script>

<svelte:window onkeydown={(event) => { if (event.key === 'Escape') dismissed = true; }}
  onscroll={positionTooltip} onresize={positionTooltip} />
<span class="action" role="presentation" onpointerenter={() => { hovered = true; dismissed = false; }} onpointerleave={() => { hovered = false; }}>
  <button bind:this={button} {...attributes} {type} disabled={disabled || (type === 'button' && !interactive)} aria-label={label}
    aria-describedby={[attributes['aria-describedby'], visible ? tooltipId : undefined].filter(Boolean).join(' ') || undefined}
    class={['action-button', className, { 'icon-only': iconOnly && !!Icon, danger }]}
    onclick={(event) => { dismissed = true; onclick?.(event); }}
    onfocus={(event) => { focused = true; dismissed = false; onfocus?.(event); }}
    onblur={(event) => { focused = false; onblur?.(event); }}>
    {#if Icon}<Icon size={18} strokeWidth={2} aria-hidden="true" focusable="false" />{/if}
    {#if !iconOnly || !Icon}<span>{label}</span>{/if}
  </button>
  {#if visible}<span bind:this={tip} id={tooltipId} role="tooltip" class="ocp-tooltip" style:left={`${tipLeft}px`} style:top={`${tipTop}px`}>{tooltip}</span>{/if}
</span>

<style>
  .action { position: relative; display: inline-block; vertical-align: middle; }
  .action-button { display: inline-flex; align-items: center; justify-content: center; gap: .45rem; min-width: 44px; min-height: 44px; }
  .icon-only { padding: .55rem; }
  .danger { background: #9f3038; border-color: #9f3038; }
</style>
