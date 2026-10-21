import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, expect, test, vi } from 'vitest';
import Trash2 from '@lucide/svelte/icons/trash-2';
import ActionButton from './ActionButton.svelte';

afterEach(cleanup);

test('names an icon-only button and hides decorative SVG from assistive technology', () => {
  render(ActionButton, { label: 'Remove menu item Döner', icon: Trash2, iconOnly: true, type: 'button' });
  const button = screen.getByRole('button', { name: 'Remove menu item Döner' });
  expect(button).toHaveAttribute('type', 'button');
  expect(button.textContent?.trim()).toBe('');
  expect(button.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
});

test('shows a tooltip on keyboard focus, dismisses it with Escape, and reopens on refocus', async () => {
  const onfocus = vi.fn();
  render(ActionButton, { label: 'Delete item', icon: Trash2, iconOnly: true, onfocus });
  const button = screen.getByRole('button', { name: 'Delete item' });
  await fireEvent.focus(button);
  const tooltip = screen.getByRole('tooltip');
  expect(tooltip).toHaveTextContent('Delete item');
  expect(button).toHaveAttribute('aria-describedby', tooltip.id);
  expect(onfocus).toHaveBeenCalledOnce();
  await fireEvent.keyDown(window, { key: 'Escape' });
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  await fireEvent.blur(button);
  await fireEvent.focus(button);
  expect(screen.getByRole('tooltip')).toBeVisible();
  await fireEvent.blur(button);
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
});

test('keeps pointer tooltip available while hovering its wrapper and forwards click behavior', async () => {
  const onclick = vi.fn();
  render(ActionButton, { label: 'Delete item', icon: Trash2, iconOnly: true, onclick, type: 'button' });
  const button = screen.getByRole('button');
  await fireEvent.pointerEnter(button.parentElement!);
  expect(screen.getByRole('tooltip')).toBeVisible();
  await fireEvent.click(button);
  expect(onclick).toHaveBeenCalledOnce();
  await fireEvent.pointerLeave(button.parentElement!);
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
});

test('retains a visible label without an icon and preserves disabled and form attributes', () => {
  render(ActionButton, { label: 'Save draft', iconOnly: true, disabled: true, name: 'intent', value: 'save' });
  const button = screen.getByRole('button', { name: 'Save draft' });
  expect(button).toHaveTextContent('Save draft');
  expect(button).toBeDisabled();
  expect(button).toHaveAttribute('type', 'submit');
  expect(button).toHaveAttribute('name', 'intent');
  expect(button).toHaveAttribute('value', 'save');
});

test('keeps labels alongside icons for less obvious actions', () => {
  render(ActionButton, { label: 'Withdraw menu', icon: Trash2 });
  expect(screen.getByRole('button', { name: 'Withdraw menu' })).toHaveTextContent('Withdraw menu');
});

test('does not repeat a visible label in a tooltip but preserves distinct supplementary help', async () => {
  const { unmount } = render(ActionButton, { label: 'Save draft' });
  await fireEvent.focus(screen.getByRole('button'));
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  unmount();
  render(ActionButton, { label: 'Save draft', tooltip: 'Customers see changes only after publication' });
  await fireEvent.focus(screen.getByRole('button'));
  expect(screen.getByRole('tooltip')).toHaveTextContent('Customers see changes only after publication');
});
