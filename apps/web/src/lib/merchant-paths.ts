type StoreLink = { display_name: string; handle?: string };

/** Names make links readable; the persistent provider-local handle disambiguates them. */
export function merchantStorePath(store: StoreLink): string {
  if (!store.handle) throw new Error('Current store handle required for navigation');
  const name = store.display_name.toLowerCase().replace(/ä/g, 'ae').replace(/ö/g, 'oe').replace(/ü/g, 'ue').replace(/ß/g, 'ss')
    .normalize('NFKD').replace(/\p{M}/gu, '').replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-|-$/g, '').slice(0, 80).replace(/-$/g, '') || 'shop';
  return `/merchant/stores/${encodeURIComponent(name)}-${store.handle}`;
}

export function storeHandleFromSlug(slug: string): string | undefined {
  return /-(store-[1-9][0-9]{0,17})$/.exec(slug)?.[1];
}
