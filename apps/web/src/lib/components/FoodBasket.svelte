<script lang="ts">
  import { onMount, tick, untrack } from 'svelte';
  import { deserialize, enhance } from '$app/forms';
  import type { components } from '$lib/api/generated';
  import { basketPrice, basketSignature, menuName, validBasketLine, type BasketLine, type FoodMenu } from '$lib/food-menu';
  import { useI18n } from '$lib/i18n/context';
  import { money } from '$lib/money';
  import MenuBrowser from './MenuBrowser.svelte';
  import Notice from './Notice.svelte';
  import ActionButton from './ActionButton.svelte';
  import ShoppingBag from '@lucide/svelte/icons/shopping-bag';
  import Plus from '@lucide/svelte/icons/plus';
  import Minus from '@lucide/svelte/icons/minus';
  import Pencil from '@lucide/svelte/icons/pencil';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import X from '@lucide/svelte/icons/x';
  const { t, locale } = useI18n();
  let { menu, merchant, store, generation, quote, intent, accepting, error }: {
    menu: FoodMenu; merchant: string; store: string; generation: number;
    quote?: components['schemas']['FoodQuote']; intent?: { quote: string; key: string }; accepting: boolean; error?: string;
  } = $props();
  let lines = $state<BasketLine[]>([]);
  let note = $state('');
  let selected = $state('');
  let editing = $state(-1);
  let variant = $state('');
  let options = $state<string[]>([]);
  let quantity = $state(1);
  let itemNote = $state('');
  let dialog = $state<HTMLDialogElement>();
  let basketDialog = $state<HTMLDialogElement>();
  let reopenBasket = false;
  let heading = $state<HTMLHeadingElement>();
  let validatedQuote = $state(untrack(() => quote));
  let validatedIntent = $state(untrack(() => intent));
  let validatedInput = $state('');
  let checking = $state(false);
  let checkError = $state('');
  let refresh = $state(0);
  let alive = true;
  let pending = false;
  let desiredInput = '';
  let ready = $state(false);
  let busy = $state(false);
  let status = $state('');
  let expandedNotes = $state<number[]>([]);
  const notePrefix = $props.id();
  let opener: HTMLElement | undefined;
  const storageKey = $derived(`ocp-basket:${merchant}:${store}`);
  const item = $derived(menu.items.find(item => item.id === selected));
  const signature = $derived(basketSignature(lines, note));
  const input = $derived(JSON.stringify({ publication_generation: generation, locale: locale(), lines, note }));
  const currentQuote = $derived(validatedQuote && validatedInput === input && basketSignature(validatedQuote.lines.map(l => l.selection), validatedQuote.note ?? '') === signature ? validatedQuote : undefined);
  const total = $derived(lines.reduce((sum, line) => sum + basketPrice(menu, line), 0));
  const configuredLine = $derived<BasketLine>({ item_id: selected, variant_id: variant, quantity, options: options.map(value => { const [group_id, option_id] = value.split(':'); return { group_id, option_id }; }), ...(itemNote ? { note: itemNote } : {}) });
  const choicesValid = $derived(!!item && item.option_groups.every(group => {
    const count = options.filter(value => value.startsWith(`${group.id}:`)).length;
    return count >= group.min_selected && count <= group.max_selected;
  }));
  onMount(() => {
    // Tab-local selections, never authoritative prices. Publication changes retain
    // the customer's choices; validation blocks anything no longer available.
    let saved: unknown;
    try { saved = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null'); } catch { /* storage unavailable */ }
    const basket = saved as { generation?: number; lines?: unknown[]; note?: string } | null;
    if (Array.isArray(basket?.lines) && basket.lines.length <= 20 && basket.lines.every(validBasketLine)
      && typeof basket.note === 'string' && [...basket.note].length <= 500) {
      lines = basket.lines; note = basket.note;
    } else if (!saved && quote) { lines = quote.lines.map(line => line.selection); note = quote.note ?? ''; }
    ready = true;
    return () => { alive = false; };
  });
  async function synchronize() {
    if (pending || !desiredInput || !alive || untrack(() => busy)) return;
    const requested = desiredInput;
    pending = true; checking = true; checkError = '';
    try {
      const body = new FormData(); body.set('basket', requested);
      const response = await fetch('?/quote', { method: 'POST', body, headers: { 'x-sveltekit-action': 'true', 'x-ocp-basket-preview': 'true' } });
      const result = deserialize(await response.text());
      if (!alive || desiredInput !== requested) return;
      if (result.type === 'success') {
        const data = result.data as { quote: components['schemas']['FoodQuote']; intent: { quote: string; key: string } };
        validatedQuote = data.quote; validatedIntent = data.intent; validatedInput = requested;
      } else {
        checkError = result.type === 'failure' ? String((result.data as { error?: string })?.error ?? t('basket-check-failed')) : t('basket-check-failed');
      }
    } catch { if (alive && desiredInput === requested) checkError = t('basket-check-failed'); }
    finally {
      pending = false;
      if (alive && desiredInput && desiredInput !== requested) void synchronize();
      else checking = false;
    }
  }
  $effect(() => {
    const payload = input; const revision = refresh;
    if (!ready) return;
    desiredInput = lines.length && accepting ? payload : '';
    if (!desiredInput) { checking = false; checkError = ''; return; }
    checking = true;
    const timer = setTimeout(() => { void revision; void synchronize(); }, 350);
    return () => clearTimeout(timer);
  });
  $effect(() => {
    if (!currentQuote) return;
    const timer = setTimeout(() => { validatedInput = ''; refresh++; }, Math.max(0, Date.parse(currentQuote.expires_at) - Date.now()));
    return () => clearTimeout(timer);
  });
  $effect(() => {
    if (!ready) return;
    try { sessionStorage.setItem(storageKey, JSON.stringify({ generation, lines, note })); } catch { /* basket still works in memory */ }
  });
  async function open(id: string, index = -1) {
    opener = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
    reopenBasket = basketDialog?.open ?? false;
    basketDialog?.close();
    selected = id; editing = index;
    const existing = index >= 0 ? lines[index] : undefined;
    const chosen = menu.items.find(item => item.id === id);
    variant = existing?.variant_id ?? chosen?.variants[0].id ?? '';
    options = existing?.options.map(o => `${o.group_id}:${o.option_id}`) ?? chosen?.option_groups.flatMap(group => (group.default_option_ids ?? []).map(id => `${group.id}:${id}`)) ?? [];
    quantity = existing?.quantity ?? 1; itemNote = existing?.note ?? '';
    await tick();
    dialog?.showModal();
    heading?.focus({ preventScroll: true });
  }
  function close() { dialog?.close(); }
  function closed() { selected = ''; if (reopenBasket) basketDialog?.showModal(); opener?.focus({ preventScroll: true }); }
  function save() {
    if (!choicesValid || !validBasketLine(configuredLine)) return;
    const next = structuredClone($state.snapshot(configuredLine));
    if (editing >= 0) lines[editing] = next;
    else {
      const same = lines.findIndex(line => basketSignature([{ ...line, quantity: 1 }], '') === basketSignature([{ ...next, quantity: 1 }], ''));
      if (same >= 0 && lines[same].quantity + next.quantity <= 99) lines[same].quantity += next.quantity;
      else if (lines.length < 20) lines.push(next);
      else { status = t('basket-limit'); return; }
    }
    status = t('basket-added'); close();
  }
</script>

<div class="store-layout">
  <MenuBrowser {menu} {merchant} {store} {selected} onchoose={id => open(id)} />
  <dialog bind:this={basketDialog} aria-labelledby="basket-heading" class="basket-dialog">
  <aside class="basket" id="basket" aria-label={t('basket-title')}>
    <header class="basket-header">
    <div class="dialog-heading"><h2 id="basket-heading"><ShoppingBag size={22} aria-hidden="true" />{t('basket-title')} <span class="basket-count">{lines.reduce((n, line) => n + line.quantity, 0)}</span></h2><ActionButton type="button" icon={X} iconOnly label={t('basket-close-basket')} onclick={() => basketDialog?.close()} /></div>
    {#if error}<Notice text={error} error />{/if}
    {#if checkError}<Notice text={checkError} error />{/if}
    </header>
    <!-- Keyboard focus enables arrow/Page Down scrolling of this named region. -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div class="basket-scroll" tabindex="0" role="region" aria-label={t('basket-items')}>
    {#if !lines.length}<p class="muted">{t('basket-empty')}</p>{/if}
    <ul class="basket-lines">
    {#each lines as line, index}
      {@const dish = menu.items.find(item => item.id === line.item_id)}
      {@const size = dish?.variants.find(v => v.id === line.variant_id)}
      <li class:line-unavailable={!dish?.available}>
        <div class="line-title"><strong><bdi dir="auto">{dish ? menuName(dish, locale()) : line.item_id}</bdi></strong><bdi>{money(basketPrice(menu, line), locale())}</bdi></div>
        {#if !dish?.available}<strong class="line-unavailable-label">{t('basket-sold-out')}</strong>{/if}
        <div class="line-description">{#if size}<bdi dir="auto">{menuName(size, locale())}</bdi>{/if}{#each line.options as selection}{@const extra = dish?.option_groups.find(g => g.id === selection.group_id)?.options.find(o => o.id === selection.option_id)}{#if extra}<span aria-hidden="true"> · </span><bdi dir="auto">{menuName(extra, locale())}</bdi>{/if}{/each}</div>
        <div class="line-actions">
          <div class="quantity" role="group" aria-label={t('storefront-quantity')}>
            <ActionButton type="button" icon={Minus} iconOnly label={t('basket-less')} disabled={busy || line.quantity <= 1} onclick={() => line.quantity--} />
            <span>{line.quantity}</span>
            <ActionButton type="button" icon={Plus} iconOnly label={t('basket-more')} disabled={busy || line.quantity >= 99} onclick={() => line.quantity++} />
          </div>
          <button type="button" class="line-note-toggle" aria-expanded={expandedNotes.includes(index)} aria-controls={`${notePrefix}-${index}`} disabled={busy} onclick={() => expandedNotes = expandedNotes.includes(index) ? expandedNotes.filter(value => value !== index) : [...expandedNotes, index]}>{t(line.note ? 'basket-edit-note' : 'basket-add-note')}</button>
          <ActionButton type="button" icon={Pencil} iconOnly label={t('basket-edit')} disabled={busy} onclick={() => open(line.item_id, index)} />
          <ActionButton type="button" icon={Trash2} iconOnly danger label={t('basket-remove')} disabled={busy} onclick={() => { lines.splice(index, 1); expandedNotes = []; }} />
        </div>
        <div class="line-note-field" id={`${notePrefix}-${index}`} hidden={!expandedNotes.includes(index)}><textarea aria-label={t('basket-item-note')} dir="auto" maxlength="500" bind:value={line.note} disabled={busy}></textarea></div>
        {#if line.note && !expandedNotes.includes(index)}<p class="note-preview"><bdi dir="auto">{line.note}</bdi></p>{/if}
      </li>
    {/each}
    </ul>
    </div>
    <footer class="basket-checkout">
    {#if lines.length}
      <details class="order-note"><summary>{t('basket-note')}{#if note} · ✓{/if}</summary><textarea aria-label={t('basket-note')} dir="auto" maxlength="500" bind:value={note} disabled={busy}></textarea><p class="muted note-help">{t('basket-note-help')}</p></details>
      <p class="basket-total"><strong>{t(currentQuote ? 'basket-total' : 'basket-estimate')}</strong><strong><bdi>{money(currentQuote?.total_minor ?? total, locale())}</bdi></strong></p>
      <p class="checkout-status" role="status">{checking ? t('basket-updating') : currentQuote ? t('basket-reviewed') : t('basket-unconfirmed')}</p>
      {#if checkError}<ActionButton type="button" label={t('basket-retry')} onclick={() => refresh++} />{/if}
      {#if currentQuote && validatedIntent && validatedIntent.quote === currentQuote.id && !checking}
        <section id="selection-quote" aria-label={t('storefront-quote')}>
          <form method="POST" action="?/place" use:enhance={() => { busy = true; return async ({ result, update }) => {
            try { if (result.type === 'redirect') sessionStorage.setItem(storageKey, JSON.stringify({ generation, lines: [], note: '' })); } catch { /* storage optional */ }
            try { await update(); } finally { busy = false; }
          }; }}>
            <input type="hidden" name="quote" value={validatedIntent.quote} /><input type="hidden" name="key" value={validatedIntent.key} />
            <ActionButton icon={ShoppingBag} label={t('storefront-place')} disabled={busy || !accepting} />
          </form>
        </section>
      {:else}<ActionButton icon={ShoppingBag} label={t('storefront-place')} disabled />{/if}
    {/if}
    </footer>
  </aside>
  </dialog>
</div>
<button type="button" class="floating-basket" onclick={() => basketDialog?.showModal()}><ShoppingBag size={20} aria-hidden="true" /><span>{t('basket-title')} · {lines.reduce((n, line) => n + line.quantity, 0)}</span>{#if lines.length}<bdi>{money(total, locale())}</bdi>{/if}</button>
<span class="sr-only" role="status">{status}</span>

<dialog bind:this={dialog} onclose={closed} aria-labelledby="dish-title" class="dish-dialog">
  {#if item}
    <div class="dialog-heading"><h2 id="dish-title" tabindex="-1" bind:this={heading}><bdi dir="auto">{menuName(item, locale())}</bdi></h2><ActionButton type="button" icon={X} iconOnly label={t('basket-close')} onclick={close} /></div>
    {#if item.image_id}<img class="dish-photo" src={`/media/public/${merchant}/${store}/${item.image_id}`} alt={menuName(item, locale())} width="480" height="360" />{/if}
    <form onsubmit={event => { event.preventDefault(); save(); }}>
      <label>{t('storefront-variant')}<select bind:value={variant}>{#each item.variants as choice}<option value={choice.id} dir="auto">{menuName(choice, locale())} · {money(choice.price_minor, locale())}</option>{/each}</select></label>
      {#each item.option_groups as group}
        <fieldset><legend><bdi dir="auto">{menuName(group, locale())}</bdi></legend><p class="choice-guidance">{t(group.min_selected === group.max_selected ? 'basket-choose-exactly' : group.min_selected === 0 ? 'basket-choose-up-to' : 'storefront-options', { min: group.min_selected, max: group.max_selected })}</p>
          <div class="choice-grid">
          {#each group.options as choice}<label class="extra"><input type={group.max_selected === 1 && group.min_selected === 1 ? 'radio' : 'checkbox'} name={group.id} value={`${group.id}:${choice.id}`} checked={options.includes(`${group.id}:${choice.id}`)} onchange={event => {
            const key = `${group.id}:${choice.id}`;
            if (group.max_selected === 1 && event.currentTarget.checked) options = options.filter(v => !v.startsWith(`${group.id}:`));
            options = event.currentTarget.checked ? [...options.filter(v => v !== key), key] : options.filter(v => v !== key);
          }} /><span><bdi dir="auto">{menuName(choice, locale())}</bdi><bdi>{choice.price_minor ? '+' : ''}{money(choice.price_minor, locale())}</bdi></span></label>{/each}
          </div>
        </fieldset>
      {/each}
      <label>{t('storefront-quantity')}<input type="number" min="1" max="99" required bind:value={quantity} /></label>
      <details><summary>{t('basket-item-note')}</summary><textarea aria-label={t('basket-item-note')} dir="auto" maxlength="500" bind:value={itemNote}></textarea><p class="muted">{t('basket-note-help')}</p></details>
      {#if !choicesValid}<p class="muted">{t('basket-choose-required')}</p>{/if}
      <div class="dialog-footer"><strong><bdi>{money(basketPrice(menu, configuredLine), locale())}</bdi></strong><ActionButton icon={ShoppingBag} label={t(editing >= 0 ? 'basket-save' : 'basket-add')} disabled={!choicesValid || !item.available || busy || !accepting} /></div>
    </form>
  {/if}
</dialog>

<style>
  :global(html:has(dialog[open])) { overflow: hidden; }
  .store-layout { padding-block-end: 5rem; }
  .basket { padding: 0; border: 0; background: white; display: flex; flex-direction: column; min-block-size: 0; max-block-size: calc(100dvh - 2rem); }
  .basket-header, .basket-checkout { flex: 0 0 auto; padding: 1rem 1.25rem; background: white; }
  .basket-header { border-block-end: 1px solid #d8e1e6; }
  .basket-checkout { border-block-start: 1px solid #d8e1e6; }
  .basket-scroll { min-block-size: 0; overflow-y: auto; overscroll-behavior: contain; padding-inline: 1.25rem; scrollbar-width: none; }
  .basket-scroll::-webkit-scrollbar { display: none; }
  .basket-scroll:focus-visible { outline: 2px solid #20586f; outline-offset: -2px; }
  .order-note { margin-block: .5rem; }
  .note-preview { overflow-wrap: anywhere; }
  .basket-total { margin-block: .65rem; } .checkout-status { font-size: .85rem; margin-block: .4rem; color: #526671; }
  .basket h2 { display: flex; align-items: center; gap: .5rem; margin-block: 0 1rem; font-size: 1.25rem; }
  .basket-count { margin-inline-start: auto; background: #edf4f7; padding: .15rem .5rem; border-radius: 1rem; }
  .basket-lines { list-style: none; margin: 0; padding: 0; }
  .basket-lines li { padding-block: .75rem; border-block-end: 1px solid #d8e1e6; }
  .line-unavailable { border-inline-start: 3px solid #9f3038; padding-inline-start: .65rem; }
  .line-unavailable-label { color: #9f3038; font-size: 1rem; }
  .line-title, .line-actions, .quantity, .basket-total, .dialog-heading, .dialog-footer { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
  .line-title { align-items: start; } .line-title > bdi { white-space: nowrap; }
  .line-description { display: flex; flex-wrap: wrap; gap: .25rem; color: #526671; margin-block: .25rem; }
  .line-actions { display: grid; grid-template-columns: auto minmax(0, 1fr) auto auto; margin-block-start: .4rem; }
  .quantity { gap: .4rem; } .line-actions :global(button) { margin: 0; }
  .line-note-toggle { justify-self: center; color: #20586f; background: transparent; border: 0; padding: .4rem; min-block-size: 44px; font: inherit; }
  .line-note-toggle:hover, .line-note-toggle[aria-expanded=true] { background: #edf4f7; border-radius: .4rem; }
  .note-preview { margin-block: .35rem 0; font-size: 1rem; color: #526671; }
  .line-note-field { margin-block: .75rem .25rem; }
  .line-note-field textarea { display: block; }
  textarea { inline-size: 100%; box-sizing: border-box; min-block-size: 5rem; font: inherit; padding: .6rem; border: 1px solid #9ab2be; border-radius: .4rem; }
  .note-help { font-size: .85rem; }
  .basket :global(section) { margin: 0; padding: 0; border: 0; background: transparent; border-radius: 0; }
  .basket-checkout :global(form), .basket-checkout :global(.action), .basket-checkout :global(button) { inline-size: 100%; box-sizing: border-box; }
  .basket-checkout :global(button) { margin: .25rem 0 0; }
  .dish-dialog, .basket-dialog { color: #193441; background: white; border: 1px solid #b8ccd5; border-radius: 1rem; padding: 1.25rem; inline-size: min(34rem, calc(100% - 2rem)); box-sizing: border-box; max-block-size: calc(100dvh - 2rem); overflow-y: auto; }
  dialog::backdrop { background: #102c3bb3; }
  .basket-dialog { padding: 0; overflow: hidden; }
  .dialog-heading h2 { margin: 0; } .dialog-heading h2:focus { outline: none; }
  .dialog-heading :global(button) { margin: 0; }
  .dish-photo { inline-size: 100%; block-size: auto; aspect-ratio: 4/3; object-fit: cover; border-radius: .6rem; margin-block: 1rem; }
  .choice-guidance { margin-block: 0 .75rem; color: #526671; font-size: .9rem; }
  .choice-grid { display: grid; gap: .5rem; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .extra { display: flex; align-items: center; gap: .5rem; padding: .65rem; border: 1px solid #c5d5dd; border-radius: .6rem; margin: 0; cursor: pointer; min-inline-size: 0; }
  .extra:has(input:checked) { background: #edf4f7; border-color: #20586f; }
  .extra:focus-within { outline: 2px solid #20586f; outline-offset: 2px; }
  .extra input { inline-size: 1rem; block-size: 1rem; flex: 0 0 1rem; margin: 0; padding: 0; accent-color: #20586f; }
  .extra span { display: flex; flex: 1; flex-direction: column; align-items: start; gap: .15rem; min-inline-size: 0; overflow-wrap: anywhere; }
  .extra span bdi + bdi { font-size: .85rem; color: #526671; white-space: nowrap; }
  .dialog-footer { position: sticky; inset-block-end: -1.25rem; background: white; padding-block: .75rem; border-block-start: 1px solid #d8e1e6; margin-block-start: 1rem; }
  .dialog-footer :global(button) { margin: 0; } summary { cursor: pointer; color: #20586f; }
  .sr-only { position: absolute; inline-size: 1px; block-size: 1px; padding: 0; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .floating-basket { position: fixed; bottom: max(1rem, env(safe-area-inset-bottom)); right: calc(max(1.4rem, (100vw - 1080px) / 2 + 1.4rem) + 3.5rem); display: flex; gap: .65rem; align-items: center; padding: .5rem .9rem; height: 44px; margin: 0; background: #20586f; color: white; border-radius: .6rem; z-index: 5; box-shadow: 0 3px 12px #102c3b55; }
  @media (max-width: 480px) { .dish-dialog, .basket-dialog { inline-size: calc(100% - 1rem); max-block-size: calc(100dvh - 1rem); } .dish-dialog { padding: 1rem; } .basket { max-block-size: calc(100dvh - 1rem); } .basket-header, .basket-checkout { padding: .75rem 1rem; } .basket-scroll { padding-inline: 1rem; } .dialog-footer { inset-block-end: -1rem; } .floating-basket { left: 1.4rem; right: 4.9rem; justify-content: center; gap: .4rem; padding: .5rem; font-size: .9rem; } }
</style>
