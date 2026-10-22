import { expect, test } from 'vitest';
import { basketMeal, mealChoices, basketPrice, basketSignature, menuName, menuSearchKey, validBasketLine, type FoodMenu } from './food-menu';
const menu: FoodMenu = { currency: 'EUR', availability_mode: 'manual', items: [{ id: 'pizza', name: 'Pizza', translations: { ar: 'بيتزا' }, available: true,
  variants: [{ id: 'large', name: 'Large', price_minor: 1000 }], option_groups: [{ id: 'extras', name: 'Extras', min_selected: 0, max_selected: 2,
    options: [{ id: 'cheese', name: 'Cheese', price_minor: 100 }, { id: 'garlic', name: 'Garlic', price_minor: 0 }] }] }] };
test('localizes labels with a stable source-name fallback', () => {
  expect(menuName(menu.items[0], 'ar')).toBe('بيتزا');
  expect(menuName(menu.items[0], 'de')).toBe('Pizza');
});

test('search ignores accents, casing, Arabic vowel marks, and surrounding whitespace', () => {
  expect(menuSearchKey(' DÖNER ')).toBe(menuSearchKey('Doner'));
  expect(menuSearchKey('Crème')).toBe(menuSearchKey('Crème'));
  expect(menuSearchKey('Straße')).toBe(menuSearchKey('STRASSE'));
  expect(menuSearchKey('مَاء')).toBe(menuSearchKey('ماء'));
});
test('estimates variants and free/paid extras for every quantity', () => {
  expect(basketPrice(menu, { item_id: 'pizza', variant_id: 'large', quantity: 2, options: [{ group_id: 'extras', option_id: 'cheese' }, { group_id: 'extras', option_id: 'garlic' }] })).toBe(2200);
});
test('cart signature ignores checkbox order but distinguishes quantities and notes', () => {
  const line = { item_id: 'pizza', variant_id: 'large', quantity: 1, options: [{ group_id: 'extras', option_id: 'cheese' }, { group_id: 'extras', option_id: 'garlic' }] };
  expect(basketSignature([line], '')).toBe(basketSignature([{ ...line, options: [...line.options].reverse() }], ''));
  expect(basketSignature([line], '')).not.toBe(basketSignature([{ ...line, quantity: 2 }], ''));
  expect(basketSignature([line], '')).not.toBe(basketSignature([{ ...line, note: 'No onions' }], ''));
  expect(basketSignature([line], '')).not.toBe(basketSignature([line], 'Bring a bag'));
});
test('validates bounded stored lines without trusting browser objects', () => {
  const line = { item_id: 'pizza', variant_id: 'large', quantity: 1, options: [] };
  expect(validBasketLine(line)).toBe(true);
  for (const bad of [null, {}, { ...line, quantity: 100 }, { ...line, quantity: 1.5 }, { ...line, item_id: '../x' }, { ...line, note: 'x'.repeat(501) }, { ...line, options: [null] }]) expect(validBasketLine(bad)).toBe(false);
});

test('uses selected-size prices including zero and charges each meal surcharge once per main', () => {
  const m = structuredClone(menu);
  m.items[0].option_groups[0].options[0].variant_prices = { large: 0 };
  m.items.push({ id: 'salad', name: 'Salad', available: true, variants: [{ id: 'regular', name: 'Regular', price_minor: 650 }], option_groups: [] });
  m.meal_deals = [{ id: 'healthy', name: 'Healthy Pizza', main_item_ids: ['pizza'], surcharge_minor: 200, addon_choices: [{ item_id: 'salad', variant_id: 'regular' }] }];
  const line = { item_id: 'pizza', variant_id: 'large', quantity: 2, options: [{ group_id: 'extras', option_id: 'cheese' }], meal_deal: { deal_id: 'healthy', addon_item_id: 'salad', addon_variant_id: 'regular' } };
  expect(basketPrice(m,line)).toBe(2400);
  expect(basketMeal(m,line)?.item.name).toBe('Salad');
  expect(validBasketLine(line)).toBe(true);
  expect(validBasketLine({ ...line, meal_deal: null })).toBe(false);
  expect(validBasketLine({ ...line, meal_deal: { ...line.meal_deal, addon_item_id: '../secret' } })).toBe(false);
  expect(basketSignature([line],'')).not.toBe(basketSignature([{ ...line, meal_deal: undefined }],''));
  expect(basketSignature([line],'')).not.toBe(basketSignature([{ ...line, meal_deal: { ...line.meal_deal, addon_variant_id: 'large' } }],''));
  m.items[1].available = false;
  expect(mealChoices(m,m.meal_deals[0])).toEqual([]);
  expect(basketMeal(m,line)).toBeUndefined();
});

test('stored selection validation accepts exact bounds and rejects malformed fields individually', () => {
  const line = { item_id:'A_0-' + 'x'.repeat(60), variant_id:'large', quantity:1, options:[{ group_id:'extras',option_id:'cheese' }], note:'😀'.repeat(500), meal_deal:{ deal_id:'meal',addon_item_id:'salad',addon_variant_id:'regular' } };
  expect(validBasketLine(line)).toBe(true);
  expect(validBasketLine({ ...line, quantity:99, options:Array(600).fill(line.options[0]) })).toBe(true);
  for (const bad of [false, 1, '', { ...line, item_id:'x'.repeat(65) }, { ...line, item_id:4 }, { ...line, item_id:'' },
    { ...line, variant_id:'../x' }, { ...line, variant_id:4 }, { ...line, variant_id:'' }, { ...line, quantity:0 },
    { ...line, options:{} }, { ...line, options:Array(601).fill(line.options[0]) }, { ...line, note:3 },
    { ...line, note:'😀'.repeat(501) }, { ...line, options:[{ group_id:'../x',option_id:'a' }] },
    { ...line, options:[{ group_id:'a',option_id:'../x' }] }, { ...line, meal_deal:'oops' },
    ...['deal_id','addon_item_id','addon_variant_id'].map(key => ({ ...line, meal_deal:{ ...line.meal_deal,[key]:'../x' } }))]) expect(validBasketLine(bad)).toBe(false);
});

test('preview safely handles missing references and only offers eligible physical addons', () => {
  const m = structuredClone(menu);
  const line = { item_id:'pizza',variant_id:'large',quantity:1,options:[] };
  expect(basketPrice(m,{ ...line,item_id:'missing' })).toBe(0);
  expect(basketPrice(m,{ ...line,variant_id:'missing' })).toBe(0);
  expect(basketPrice(m,{ ...line,options:[{ group_id:'missing',option_id:'missing' }] })).toBe(1000);
  expect(basketPrice(m,{ ...line,options:[{ group_id:'extras',option_id:'missing' }] })).toBe(1000);
  expect(basketMeal(m,line)).toBeUndefined();
  m.items.push({ id:'salad',name:'Salad',available:true,variants:[{ id:'regular',name:'Regular',price_minor:650 }],option_groups:[] });
  m.meal_deals = [{ id:'healthy',name:'Meal',main_item_ids:['pizza'],surcharge_minor:200,addon_choices:[{ item_id:'salad',variant_id:'regular' }] }];
  const deal = m.meal_deals[0];
  const mealLine = { ...line,meal_deal:{ deal_id:'healthy',addon_item_id:'salad',addon_variant_id:'regular' } };
  expect(mealChoices(m,deal).map(choice => [choice.item.id,choice.variant.id])).toEqual([['salad','regular']]);
  expect(basketMeal(m,mealLine)?.deal.id).toBe('healthy');
  for (const patch of [{ deal_id:'unknown' }, { addon_item_id:'pizza' }, { addon_variant_id:'large' }]) expect(basketMeal(m,{ ...mealLine,meal_deal:{ ...mealLine.meal_deal,...patch } })).toBeUndefined();
  expect(basketMeal(m,{ ...mealLine,item_id:'salad' })).toBeUndefined();
  expect(basketPrice(m,{ ...mealLine,item_id:'salad',variant_id:'regular' })).toBe(650);
  for (const choice of [{ item_id:'missing',variant_id:'regular' }, { item_id:'salad',variant_id:'missing' }]) expect(mealChoices(m,{ ...deal,addon_choices:[choice] })).toEqual([]);
  m.items[1].option_groups = [{ id:'dressing',name:'Dressing',min_selected:1,max_selected:1,options:[{ id:'oil',name:'Oil',price_minor:0 }] }];
  expect(mealChoices(m,deal)).toEqual([]);
  expect(basketSignature([line],'')).not.toBe(basketSignature([{ ...line,item_id:'salad' }],''));
  expect(basketSignature([line],'')).not.toBe(basketSignature([{ ...line,variant_id:'other' }],''));
  expect(basketSignature([mealLine],'')).not.toBe(basketSignature([{ ...mealLine,meal_deal:{ ...mealLine.meal_deal,deal_id:'other' } }],''));
  expect(basketSignature([mealLine],'')).not.toBe(basketSignature([{ ...mealLine,meal_deal:{ ...mealLine.meal_deal,addon_item_id:'other' } }],''));
  expect(menuName({ name:'Default' },'en')).toBe('Default');
});
