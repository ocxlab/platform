export const money = (minor: number) => new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' }).format(minor / 100);
