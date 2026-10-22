export default function problemResponses(responses, _options, { path }) {
  const findings = [];
  for (const [status, response] of Object.entries(responses ?? {})) {
    // Redirects are not errors. 204/304 may legitimately omit a body.
    if (status !== 'default' && !/^[45](?:[0-9]{2}|XX)$/i.test(status)) continue;
    const content = response?.content ?? {};
    const schema = content['application/problem+json']?.schema;
    if (Object.keys(content).length !== 1 || !schema) {
      findings.push({ message: 'Errors must declare only application/problem+json with a schema.', path: [...path, status, 'content'] });
    }
  }
  return findings;
}
