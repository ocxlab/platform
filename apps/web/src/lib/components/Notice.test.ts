import { render, screen, cleanup } from '@testing-library/svelte';
import { afterEach, expect, test } from 'vitest';
import Notice from './Notice.svelte';
afterEach(cleanup);
test('errors are announced and text is escaped', () => { render(Notice, { text: '<script>not markup</script>', error: true }); expect(screen.getByRole('alert')).toHaveTextContent('<script>not markup</script>'); expect(document.querySelector('script')).toBeNull(); });
test('successful operations have status semantics', () => { render(Notice, { text: 'Menu published.' }); expect(screen.getByRole('status')).toHaveTextContent('Menu published.'); });
