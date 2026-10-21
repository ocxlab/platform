<script lang="ts">
  import { page } from '$app/state';
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
</script>
<h1>{t('ui-error-heading')}</h1>
<p role="alert">{page.error?.message}</p>
<a href="/">{t('ui-back-home')}</a>
