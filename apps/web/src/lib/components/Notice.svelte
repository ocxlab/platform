<script lang="ts">
  let { text, error = false }: { text: string; error?: boolean } = $props();
</script>
<p class:error role={error ? 'alert' : 'status'}>{text}</p>
<style>p { padding: .8rem 1rem; background: #eaf2f5; border-left: 4px solid #256078; } p.error { background: #fff0ed; border-color: #a03e2d; }</style>
