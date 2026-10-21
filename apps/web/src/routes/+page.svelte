<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
</script>
<h1>{t('ui-home-title')}</h1>
<p>{t('ui-home-intro')}</p>
<section><h2>{t('ui-for-merchants')}</h2><p>{t('ui-home-merchant')}</p><a href="/merchant">{t('ui-open-workspace')}</a></section>
<section><h2>{t('ui-browse-store')}</h2><p>{t('ui-home-browse')}</p></section>
